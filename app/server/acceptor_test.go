package server

import (
    "net"
    "testing"
    "time"

    kupool "github.com/JellyTony/kupool"
    "github.com/JellyTony/kupool/protocol"
    "github.com/JellyTony/kupool/tcp"
    "github.com/JellyTony/kupool/events"
)

type fakeStore struct{}
func (f *fakeStore) Increment(string, time.Time) error { return nil }
type fakeMQ struct{}
func (f *fakeMQ) Publish(evt events.SubmitEvent) error { return nil }
func (f *fakeMQ) Subscribe() <-chan events.SubmitEvent { ch := make(chan events.SubmitEvent); close(ch); return ch }
func (f *fakeMQ) Close() error { return nil }

type fakePusher struct{ last []byte }
func (p *fakePusher) Push(id string, data []byte) error { p.last = data; return nil }

func TestAcceptorAuthorize(t *testing.T) {
    s, c := net.Pipe()
    defer s.Close(); defer c.Close()
    coord := NewCoordinator(&fakePusher{}, &fakeStore{}, &fakeMQ{}, time.Second, 0)
    acc := NewAcceptor(coord)
    go func(){
        id := 1
        req := protocol.Request{ID: &id, Method: "authorize"}
        p, _ := protocol.Encode(protocol.AuthorizeParams{Username: "u"})
        req.Params = p
        raw, _ := protocol.Encode(req)
        _ = tcp.WriteFrame(c, kupool.OpBinary, raw)
        cc := tcp.NewConn(c)
        _ = c.SetReadDeadline(time.Now().Add(time.Second))
        _, _ = cc.ReadFrame()
    }()
    conn := tcp.NewConn(s)
    chID, err := acc.Accept(conn, time.Second)
    if err != nil { t.Fatal(err) }
    if chID == "" { t.Fatal("empty channel id") }
    _ = c.SetReadDeadline(time.Now().Add(time.Second))
    if coord.sessions[chID] == nil { t.Fatal("session not registered") }
    if coord.sessions[chID].Username != "u" { t.Fatal("username mismatch") }
}
