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
    historyWindow time.Duration
    srv          ServerPusher
    store        StatsStore
    state        StateStore
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
    Get(username string, minute time.Time) (int, error)
    Close() error
}

type StateStore interface {
    SaveJob(jobID int, nonce string, createdAt time.Time) error
    LoadLatestJob() (jobID int, nonce string, createdAt time.Time, err error)
    LoadJobHistory(since time.Duration) (map[int]JobRecord, error)
    LoadUserState(username string) (latestJobID int, latestServerNonce string, lastSubmitAt time.Time, err error)
    SaveUserState(username string, latestJobID int, latestServerNonce string, lastSubmitAt time.Time) error
    SaveUsedNonce(username string, jobID int, clientNonce string) error
    HasUsedNonce(username string, jobID int, clientNonce string) (bool, error)
    Close() error
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
