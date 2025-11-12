package protocol

import (
    "encoding/json"
    "testing"
)

func TestEncodeDecodeAuthorize(t *testing.T) {
    id := 1
    req := Request{ID: &id, Method: "authorize"}
    p, _ := Encode(AuthorizeParams{Username: "admin"})
    req.Params = p
    raw, err := Encode(req)
    if err != nil { t.Fatal(err) }
    var out Request
    if err := Decode(raw, &out); err != nil { t.Fatal(err) }
    if out.Method != "authorize" { t.Fatal("method mismatch") }
    var ap AuthorizeParams
    if err := Decode(out.Params, &ap); err != nil { t.Fatal(err) }
    if ap.Username != "admin" { t.Fatal("param mismatch") }
}

func TestEncodeDecodeJobSubmit(t *testing.T) {
    msg := struct{ ID any; Method string; Params JobParams }{ID: nil, Method: "job", Params: JobParams{JobID: 1, ServerNonce: "n"}}
    raw, _ := Encode(msg)
    var req Request
    if err := Decode(raw, &req); err != nil { t.Fatal(err) }
    if req.ID != nil { t.Fatal("id should be nil") }
    if req.Method != "job" { t.Fatal("method mismatch") }
    var jp JobParams
    if err := Decode(req.Params, &jp); err != nil { t.Fatal(err) }
    if jp.JobID != 1 || jp.ServerNonce != "n" { t.Fatal("params mismatch") }

    id := 2
    s := SubmitParams{JobID: 1, ClientNonce: "c", Result: "r"}
    sp, _ := Encode(s)
    req2 := Request{ID: &id, Method: "submit", Params: sp}
    raw2, _ := Encode(req2)
    var out2 Request
    _ = Decode(raw2, &out2)
    if out2.ID == nil || *out2.ID != 2 { t.Fatal("id mismatch") }
}

func TestResponseEncodeDecode(t *testing.T) {
    r := Response{ID: 2, Result: false}
    raw, _ := Encode(r)
    var out Response
    _ = Decode(raw, &out)
    if out.ID != 2 || out.Result != false { t.Fatal("response mismatch") }
}

func TestInvalidJSON(t *testing.T) {
    var req Request
    if err := Decode([]byte("{"), &req); err == nil { t.Fatal("expect error") }
    var params AuthorizeParams
    if err := json.Unmarshal([]byte("{"), &params); err == nil { t.Fatal("expect error") }
}

