package server

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/JellyTony/kupool/logger"
	"github.com/JellyTony/kupool/protocol"
)

func NewCoordinator(p ServerPusher, store StatsStore, mq MessageQueue, interval time.Duration, expire time.Duration) *Coordinator {
	return &Coordinator{
		sessions:      make(map[string]*Session),
		srv:           p,
		store:         store,
		mq:            mq,
		nonceInterval: interval,
		history:       make(map[int]JobRecord),
		expireAfter:   expire,
		stopCh:        make(chan struct{}),
	}
}

func (c *Coordinator) RegisterSession(channelID, username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[channelID] = &Session{ChannelID: channelID, Username: username, UsedNonces: make(map[int]map[string]struct{})}
}

func (c *Coordinator) UnregisterSession(channelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, channelID)
}

func (c *Coordinator) StartBroadcast() { go c.loop() }

func (c *Coordinator) loop() {
	ticker := time.NewTicker(c.nonceInterval)
	defer ticker.Stop()
	for {
		c.rotateJob()
		c.broadcastJob()
		select {
		case <-ticker.C:
			continue
		case <-c.stopCh:
			return
		}
	}
}

func (c *Coordinator) rotateJob() {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	c.mu.Lock()
	c.jobID++
	c.serverNonce = hex.EncodeToString(buf)
	c.history[c.jobID] = JobRecord{Nonce: c.serverNonce, CreatedAt: time.Now()}
	c.mu.Unlock()
}

type broadcastMsg struct {
	ID     any                `json:"id"`
	Method string             `json:"method"`
	Params protocol.JobParams `json:"params"`
}

func (c *Coordinator) broadcastJob() {
	c.mu.RLock()
	jobID := c.jobID
	nonce := c.serverNonce
	var sessions []*Session
	for _, s := range c.sessions {
		sessions = append(sessions, s)
	}
	c.mu.RUnlock()
	msg := broadcastMsg{ID: nil, Method: "job", Params: protocol.JobParams{JobID: jobID, ServerNonce: nonce}}
	data, _ := protocol.Encode(msg)
	c.mu.Lock()
	for _, s := range sessions {
		s.LatestJobID = jobID
		s.LatestServerNonce = nonce
		_ = c.srv.Push(s.ChannelID, data)
	}
	c.mu.Unlock()
	logger.WithFields(logger.Fields{"module": "app.coordinator", "job_id": jobID, "nonce": nonce, "sessions": len(sessions)}).Info("broadcast job")
}

func (c *Coordinator) Stop() { close(c.stopCh) }
