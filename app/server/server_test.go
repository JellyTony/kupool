package server

import (
    "crypto/sha256"
    "encoding/hex"
    "net"
    "testing"
    "time"

    kupool "github.com/JellyTony/kupool"
    "github.com/JellyTony/kupool/mq"
    "github.com/JellyTony/kupool/protocol"
    "github.com/JellyTony/kupool/stats"
    "github.com/JellyTony/kupool/tcp"
)

func dialAuthorize(t *testing.T, addr, username string) net.Conn {
    conn, err := net.DialTimeout("tcp", addr, time.Second*3)
    if err != nil { t.Fatal(err) }
    id := 1
    req := protocol.Request{ID: &id, Method: "authorize"}
    p, _ := protocol.Encode(protocol.AuthorizeParams{Username: username})
    req.Params = p
    data, _ := protocol.Encode(req)
    _ = tcp.WriteFrame(conn, kupool.OpBinary, data)
    return conn
}

func readJob(t *testing.T, conn net.Conn) protocol.JobParams {
    c := tcp.NewConn(conn)
    for {
        f, err := c.ReadFrame()
        if err != nil { t.Fatal(err) }
        if f.GetOpCode() != kupool.OpBinary { continue }
        var req protocol.Request
        _ = protocol.Decode(f.GetPayload(), &req)
        if req.Method == "job" {
            var p protocol.JobParams
            _ = protocol.Decode(req.Params, &p)
            return p
        }
    }
}

func sendSubmit(t *testing.T, conn net.Conn, id int, p protocol.SubmitParams) protocol.Response {
    req := protocol.Request{ID: &id, Method: "submit"}
    raw, _ := protocol.Encode(p)
    req.Params = raw
    data, _ := protocol.Encode(req)
    c := tcp.NewConn(conn)
    _ = c.WriteFrame(kupool.OpBinary, data)
    f, err := c.ReadFrame()
    if err != nil { t.Fatal(err) }
    var resp protocol.Response
    _ = protocol.Decode(f.GetPayload(), &resp)
    return resp
}

func TestSuccessFlow(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(16)
    app := NewAppServer("127.0.0.1:9091", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    conn := dialAuthorize(t, "127.0.0.1:9091", "u1")
    job := readJob(t, conn)
    res := clientResult(job.ServerNonce, "abc")
    resp := sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "abc", Result: res})
    if !resp.Result { t.Fatal("expect success") }
}

func TestInvalidResult(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(16)
    app := NewAppServer("127.0.0.1:9092", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    conn := dialAuthorize(t, "127.0.0.1:9092", "u2")
    job := readJob(t, conn)
    resp := sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "abc", Result: "deadbeef"})
    if resp.Result { t.Fatal("expect failure") }
}

func TestRateLimit(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(16)
    app := NewAppServer("127.0.0.1:9093", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    conn := dialAuthorize(t, "127.0.0.1:9093", "u3")
    job := readJob(t, conn)
    res := clientResult(job.ServerNonce, "x1")
    _ = sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "x1", Result: res})
    res2 := clientResult(job.ServerNonce, "x2")
    resp2 := sendSubmit(t, conn, 3, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "x2", Result: res2})
    if resp2.Result { t.Fatal("expect rate limit failure") }
}

func TestDuplicate(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(16)
    app := NewAppServer("127.0.0.1:9094", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    conn := dialAuthorize(t, "127.0.0.1:9094", "u4")
    job := readJob(t, conn)
    res := clientResult(job.ServerNonce, "dup")
    _ = sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "dup", Result: res})
    resp2 := sendSubmit(t, conn, 3, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "dup", Result: res})
    if resp2.Result { t.Fatal("expect duplicate failure") }
}

func TestTaskNotExist(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(16)
    app := NewAppServer("127.0.0.1:9095", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    conn := dialAuthorize(t, "127.0.0.1:9095", "u5")
    job := readJob(t, conn)
    res := clientResult(job.ServerNonce, "z")
    resp := sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID+100, ClientNonce: "z", Result: res})
    if resp.Result { t.Fatal("expect task not exist failure") }
}

func clientResult(serverNonce, clientNonce string) string {
    h := sha256.Sum256([]byte(serverNonce + clientNonce))
    return hex.EncodeToString(h[:])
}

func TestConcurrentClients(t *testing.T) {
    store := stats.NewMemoryStore()
    queue := mq.NewMemoryQueue(64)
    app := NewAppServer("127.0.0.1:9096", store, queue, time.Millisecond*200)
    go func(){ _ = app.Start() }()
    t.Cleanup(func(){ _ = app.Shutdown() })
    time.Sleep(time.Millisecond*100)
    n := 5
    done := make(chan struct{}, n)
    for i := 0; i < n; i++ {
        go func(idx int){
            conn := dialAuthorize(t, "127.0.0.1:9096", "u"+string(rune('a'+idx)))
            job := readJob(t, conn)
            res := clientResult(job.ServerNonce, "c")
            resp := sendSubmit(t, conn, 2, protocol.SubmitParams{JobID: job.JobID, ClientNonce: "c", Result: res})
            if !resp.Result { t.Fatal("expect success") }
            done <- struct{}{}
        }(i)
    }
    for i := 0; i < n; i++ { <-done }
}
