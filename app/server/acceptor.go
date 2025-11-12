package server

import (
    "crypto/rand"
    "encoding/hex"
    "errors"
    "time"

    kupool "github.com/JellyTony/kupool"
    "github.com/JellyTony/kupool/logger"
    "github.com/JellyTony/kupool/protocol"
)

type Acceptor struct {
    coord *Coordinator
}

func NewAcceptor(coord *Coordinator) *Acceptor {
    return &Acceptor{coord: coord}
}

func (a *Acceptor) Accept(conn kupool.Conn, timeout time.Duration) (string, error) {
    _ = conn.SetReadDeadline(time.Now().Add(timeout))
    frame, err := conn.ReadFrame()
    if err != nil {
        logger.WithFields(logger.Fields{"module":"app.acceptor","stage":"read","error":err}).Warn("accept read error")
        return "", err
    }
    if frame.GetOpCode() != kupool.OpBinary {
        logger.WithFields(logger.Fields{"module":"app.acceptor","opcode":frame.GetOpCode()}).Warn("invalid opcode in accept")
        return "", errors.New("invalid opcode")
    }
    var req protocol.Request
    if err := protocol.Decode(frame.GetPayload(), &req); err != nil {
        return "", err
    }
    if req.Method != "authorize" || req.Params == nil {
        logger.WithFields(logger.Fields{"module":"app.acceptor","method":req.Method}).Warn("unauthorized: invalid method")
        return "", errors.New("unauthorized")
    }
    var p protocol.AuthorizeParams
    if err := protocol.Decode(req.Params, &p); err != nil {
        return "", err
    }
    if p.Username == "" {
        logger.WithFields(logger.Fields{"module":"app.acceptor"}).Warn("unauthorized: empty username")
        return "", errors.New("unauthorized")
    }
    buf := make([]byte, 16)
    if _, err := rand.Read(buf); err != nil {
        return "", err
    }
    chID := hex.EncodeToString(buf)
    a.coord.RegisterSession(chID, p.Username)
    logger.WithFields(logger.Fields{"module":"app.acceptor","username":p.Username,"channel_id":chID}).Info("authorized")
    resp := protocol.Response{ID: *req.ID, Result: true}
    data, _ := protocol.Encode(resp)
    _ = conn.WriteFrame(kupool.OpBinary, data)
    return chID, nil
}
