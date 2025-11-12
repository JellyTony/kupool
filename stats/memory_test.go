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
	if v != 2 {
		t.Fatal("aggregate by minute")
	}
	v2, _ := s.Get("u", now.Add(time.Minute))
	if v2 != 0 {
		t.Fatal("next minute should be 0")
	}
}

func TestMemoryStoreJobHistoryNonce(t *testing.T) {
	s := NewMemoryStore()
	// save two jobs with different nonce
	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	_ = s.SaveJob(1, "nonce1", t1)
	_ = s.SaveJob(2, "nonce2", t2)
	// latest
	id, nonce, created, _ := s.LoadLatestJob()
	if id != 2 || nonce != "nonce2" || !created.Equal(t2) {
		t.Fatal("latest job mismatch")
	}
	// history within 3h should include both with their own nonce
	hist, _ := s.LoadJobHistory(3 * time.Hour)
	if len(hist) != 2 {
		t.Fatal("history size")
	}
	if hist[1].Nonce != "nonce1" || !hist[1].CreatedAt.Equal(t1) {
		t.Fatal("history 1 mismatch")
	}
	if hist[2].Nonce != "nonce2" || !hist[2].CreatedAt.Equal(t2) {
		t.Fatal("history 2 mismatch")
	}
	// cutoff 90m should only include job 2
	hist2, _ := s.LoadJobHistory(90 * time.Minute)
	if len(hist2) != 1 || hist2[2].Nonce != "nonce2" {
		t.Fatal("cutoff mismatch")
	}
}
