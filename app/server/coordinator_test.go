package server

import (
    "testing"
    "time"
)

type memPusher struct{ count int; last []byte }
func (p *memPusher) Push(id string, data []byte) error { p.count++; p.last = data; return nil }

func TestCoordinatorBroadcast(t *testing.T) {
    p := &memPusher{}
    coord := NewCoordinator(p, &fakeStore{}, nil, &fakeMQ{}, time.Millisecond*100, 0, time.Hour)
    coord.RegisterSession("c1", "u1")
    coord.RegisterSession("c2", "u2")
    coord.rotateJob()
    coord.broadcastJob()
    if p.count != 2 { t.Fatal("broadcast count") }
    s1 := coord.sessions["c1"]
    if s1.LatestJobID == 0 || s1.LatestServerNonce == "" { t.Fatal("session updated") }
}
