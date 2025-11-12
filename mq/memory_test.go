package mq

import (
    "testing"
    "github.com/JellyTony/kupool/events"
)

func TestMemoryQueuePubSub(t *testing.T) {
    q := NewMemoryQueue(2)
    ch := q.Subscribe()
    _ = q.Publish(events.SubmitEvent{Username: "u"})
    _ = q.Publish(events.SubmitEvent{Username: "u2"})
    e1 := <-ch
    e2 := <-ch
    if e1.Username == "" || e2.Username == "" { t.Fatal("event") }
    _ = q.Close()
}

