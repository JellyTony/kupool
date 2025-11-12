package protocol

import (
    "encoding/json"
)

type Request struct {
    ID     *int            `json:"id"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params"`
}

type Response struct {
    ID     int     `json:"id"`
    Result bool    `json:"result"`
    Error  *string `json:"error,omitempty"`
}

type AuthorizeParams struct {
    Username string `json:"username"`
}

type JobParams struct {
    JobID       int    `json:"job_id"`
    ServerNonce string `json:"server_nonce"`
}

type SubmitParams struct {
    JobID       int    `json:"job_id"`
    ClientNonce string `json:"client_nonce"`
    Result      string `json:"result"`
}

func Encode(v any) ([]byte, error) {
    return json.Marshal(v)
}

func Decode(data []byte, v any) error {
    return json.Unmarshal(data, v)
}

