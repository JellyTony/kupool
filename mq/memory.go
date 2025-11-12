package mq

import (
    "sync"

    "github.com/JellyTony/kupool/events"
)

type MemoryQueue struct {
    ch   chan events.SubmitEvent
    once sync.Once
    closed bool
}

func NewMemoryQueue(size int) *MemoryQueue {
    return &MemoryQueue{ch: make(chan events.SubmitEvent, size)}
}

func (q *MemoryQueue) Publish(evt events.SubmitEvent) error {
    if q.closed { return nil }
    q.ch <- evt
    return nil
}

func (q *MemoryQueue) Subscribe() <-chan events.SubmitEvent {
    return q.ch
}

func (q *MemoryQueue) Close() error {
    q.once.Do(func(){ close(q.ch); q.closed = true })
    return nil
}
