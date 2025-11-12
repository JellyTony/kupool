package server

import (
    "sync"
    "time"
    "github.com/JellyTony/kupool/events"
)

type Session struct {
    ChannelID        string
    Username         string
    LatestJobID      int
    LatestServerNonce string
    LastSubmitAt     time.Time
    UsedNonces       map[int]map[string]struct{}
}

type Coordinator struct {
    mu           sync.RWMutex
    sessions     map[string]*Session
    jobID        int
    serverNonce  string
    nonceInterval time.Duration
    srv          ServerPusher
    store        StatsStore
    mq           MessageQueue
    history      map[int]JobRecord
    expireAfter  time.Duration
    stopCh       chan struct{}
}

type ServerPusher interface {
    Push(id string, data []byte) error
}

type StatsStore interface {
    Increment(username string, minute time.Time) error
}

type MessageQueue interface {
    Publish(evt events.SubmitEvent) error
    Subscribe() <-chan events.SubmitEvent
    Close() error
}

type JobRecord struct {
    Nonce     string
    CreatedAt time.Time
}
