package stats

import (
    "os"
    "testing"
    "time"
)

func TestPGStoreIncrementGet(t *testing.T) {
    dsn := os.Getenv("KUP_PG_DSN")
    if dsn == "" { t.Skip("PG DSN not provided") }
    s, err := NewPGStore(dsn)
    if err != nil { t.Fatal(err) }
    now := time.Now()
    _ = s.Increment("u", now)
    _ = s.Increment("u", now.Add(10*time.Second))
    v, _ := s.Get("u", now)
    if v < 2 { t.Fatal("expect at least 2") }
}

