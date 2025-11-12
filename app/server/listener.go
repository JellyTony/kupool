package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	kupool "github.com/JellyTony/kupool"
	"github.com/JellyTony/kupool/events"
	"github.com/JellyTony/kupool/logger"
	"github.com/JellyTony/kupool/protocol"
)

type Listener struct {
	coord *Coordinator
}

func NewListener(coord *Coordinator) *Listener {
	return &Listener{coord: coord}
}

func (l *Listener) Receive(ag kupool.Agent, payload []byte) {
    var req protocol.Request
    if err := protocol.Decode(payload, &req); err != nil {
        logger.WithFields(logger.Fields{"module":"app.listener","stage":"decode","error":err}).Warn("decode error")
        return
    }
    if req.Method != "submit" || req.ID == nil {
        return
    }
    var p protocol.SubmitParams
    if err := protocol.Decode(req.Params, &p); err != nil {
        l.respondError(ag, *req.ID, "Invalid result")
        return
    }
    chID := ag.(kupool.Channel).ID()
    logger.WithFields(logger.Fields{"module":"app.listener","channel_id":chID,"job_id":p.JobID}).Debug("submit received")
    if err := l.handleSubmit(chID, p); err != nil {
        l.respondError(ag, *req.ID, err.Error())
        return
    }
	resp := protocol.Response{ID: *req.ID, Result: true}
	data, _ := protocol.Encode(resp)
	_ = ag.Push(data)
}

func (l *Listener) handleSubmit(channelID string, p protocol.SubmitParams) error {
	l.coord.mu.Lock()
	s, ok := l.coord.sessions[channelID]
	l.coord.mu.Unlock()
	if !ok {
		return errors.New("Task does not exist")
	}
    if p.JobID != s.LatestJobID {
        if rec, ok := l.coord.history[p.JobID]; ok {
            s.LatestServerNonce = rec.Nonce
            if l.coord.expireAfter > 0 && time.Since(rec.CreatedAt) > l.coord.expireAfter {
                return errors.New("Task expired")
            }
        } else {
            return errors.New("Task does not exist")
        }
    }
	now := time.Now()
    if !s.LastSubmitAt.IsZero() && now.Sub(s.LastSubmitAt) < time.Second {
        return errors.New("Submission too frequent")
    }
    m, ok := s.UsedNonces[p.JobID]
	if !ok {
		m = make(map[string]struct{})
		s.UsedNonces[p.JobID] = m
	}
	if _, dup := m[p.ClientNonce]; dup {
		return errors.New("Duplicate submission")
	}
	computed := sha256.Sum256([]byte(s.LatestServerNonce + p.ClientNonce))
	hexed := hex.EncodeToString(computed[:])
	if !strings.EqualFold(hexed, p.Result) {
		return errors.New("Invalid result")
	}
    m[p.ClientNonce] = struct{}{}
    s.LastSubmitAt = now
    if l.coord.state != nil {
        _ = l.coord.state.SaveUsedNonce(s.Username, p.JobID, p.ClientNonce)
        _ = l.coord.state.SaveUserState(s.Username, s.LatestJobID, s.LatestServerNonce, s.LastSubmitAt)
    }
    _ = l.coord.mq.Publish(events.SubmitEvent{Username: s.Username, Time: now})
    logger.WithFields(logger.Fields{"module":"app.listener","username":s.Username,"job_id":p.JobID,"client_nonce":p.ClientNonce}).Info("submit accepted")
    return nil
}

func (l *Listener) respondError(ag kupool.Agent, id int, msg string) {
	resp := protocol.Response{ID: id, Result: false, Error: &msg}
    data, _ := protocol.Encode(resp)
    _ = ag.Push(data)
    logger.WithFields(logger.Fields{"module":"app.listener","error":msg}).Warn("submit rejected")
}

type State struct {
	coord *Coordinator
}

func NewState(coord *Coordinator) *State { return &State{coord: coord} }

func (s *State) Disconnect(id string) error {
	s.coord.UnregisterSession(id)
	return nil
}
