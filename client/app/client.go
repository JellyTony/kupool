package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"time"

	kupool "github.com/JellyTony/kupool"
	"github.com/JellyTony/kupool/logger"
	"github.com/JellyTony/kupool/protocol"
	"github.com/JellyTony/kupool/tcp"
)

type Client struct {
	cli         kupool.Client
	username    string
	jobID       int
	serverNonce string
	nextID      int
}

func NewClient(username string) *Client {
	c := &Client{username: username, nextID: 1}
	c.cli = tcp.NewClient(username, "client", tcp.ClientOptions{})
	c.cli.SetDialer(&dialer{username: username})
	return c
}

func (c *Client) Connect(addr string) error { return c.cli.Connect(addr) }

func (c *Client) Run() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	minuteTicker := time.NewTicker(time.Minute)
	defer minuteTicker.Stop()
	lastSubmit := time.Time{}
	for {
		frame, err := c.cli.Read()
		if err != nil {
			return err
		}
		if frame.GetOpCode() != kupool.OpBinary {
			continue
		}
		var msg protocol.Request
		if err := protocol.Decode(frame.GetPayload(), &msg); err != nil {
			continue
		}
		if msg.Method == "job" {
			var p protocol.JobParams
			_ = protocol.Decode(msg.Params, &p)
			c.jobID = p.JobID
			c.serverNonce = p.ServerNonce
			logger.WithFields(logger.Fields{"module": "client", "job_id": p.JobID, "server_nonce": p.ServerNonce}).Info("job received")
		}
		select {
		case <-ticker.C:
			if c.serverNonce == "" {
				continue
			}
			if !lastSubmit.IsZero() && time.Since(lastSubmit) < time.Second {
				logger.WithFields(logger.Fields{"module": "client"}).Debug("skip submit due to rate limit")
				continue
			}
			clientNonce := randNonce()
			res := computeSHA256Hex(c.serverNonce + clientNonce)
			id := c.nextID
			c.nextID++
			req := protocol.Request{ID: &id, Method: "submit"}
			sp, _ := protocol.Encode(protocol.SubmitParams{JobID: c.jobID, ClientNonce: clientNonce, Result: res})
			req.Params = sp
			data, _ := protocol.Encode(req)
			_ = c.cli.Send(data)
			logger.WithFields(logger.Fields{"module": "client", "job_id": c.jobID, "client_nonce": clientNonce}).Info("submit sent")
			lastSubmit = time.Now()
		case <-minuteTicker.C:
			if c.serverNonce == "" {
				continue
			}
			if time.Since(lastSubmit) >= time.Minute {
				clientNonce := randNonce()
				res := computeSHA256Hex(c.serverNonce + clientNonce)
				id := c.nextID
				c.nextID++
				req := protocol.Request{ID: &id, Method: "submit"}
				sp, _ := protocol.Encode(protocol.SubmitParams{JobID: c.jobID, ClientNonce: clientNonce, Result: res})
				req.Params = sp
				data, _ := protocol.Encode(req)
				_ = c.cli.Send(data)
				logger.WithFields(logger.Fields{"module": "client", "job_id": c.jobID, "client_nonce": clientNonce}).Info("submit sent (minute guard)")
				lastSubmit = time.Now()
			}
		default:
		}
		if msg.Method == "submit" && msg.ID != nil {
			var resp protocol.Response
			if err := protocol.Decode(frame.GetPayload(), &resp); err == nil {
				if !resp.Result && resp.Error != nil {
					logger.Warnf("submit failed: %s", *resp.Error)
				} else {
					logger.WithFields(logger.Fields{"module": "client", "id": resp.ID}).Info("submit ok")
				}
			}
		}
	}
}

func (c *Client) Close() { c.cli.Close() }

type dialer struct{ username string }

func (d *dialer) DialAndHandshake(ctx kupool.DialerContext) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", ctx.Address, ctx.Timeout)
	if err != nil {
		return nil, err
	}
	id := 1
	req := protocol.Request{ID: &id, Method: "authorize"}
	p, _ := protocol.Encode(protocol.AuthorizeParams{Username: d.username})
	req.Params = p
	data, _ := protocol.Encode(req)
	_ = tcp.WriteFrame(conn, kupool.OpBinary, data)
	return conn, nil
}

func randNonce() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func computeSHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
