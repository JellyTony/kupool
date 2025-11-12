package mq

import (
    "os"
    "testing"
    "time"
    "github.com/JellyTony/kupool/events"
)

func TestRabbit(t *testing.T){
    url := os.Getenv("KUP_MQ_URL")
    if url == "" { t.Skip("MQ URL not provided") }
    r, err := NewRabbitMQ(url, "kupool_test")
    if err != nil { t.Fatal(err) }
    defer r.Close()
    ch := r.Subscribe()
    _ = r.Publish(events.SubmitEvent{Username:"u", Time: time.Now()})
    _ = r.Publish(events.SubmitEvent{Username:"u2", Time: time.Now()})
    select{
    case <-ch:
    case <-time.After(3*time.Second): t.Fatal("timeout")
    }
}

