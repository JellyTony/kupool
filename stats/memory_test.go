package stats

import (
    "testing"
    "time"
)

func TestMemoryStoreIncrementGet(t *testing.T) {
    s := NewMemoryStore()
    now := time.Now()
    _ = s.Increment("u", now)
    _ = s.Increment("u", now.Add(10*time.Second))
    v, _ := s.Get("u", now)
    if v != 2 { t.Fatal("aggregate by minute") }
    v2, _ := s.Get("u", now.Add(time.Minute))
    if v2 != 0 { t.Fatal("next minute should be 0") }
}

