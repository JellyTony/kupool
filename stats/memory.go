package stats

import (
    "sync"
    "time"
)

type MemoryStore struct {
    mu   sync.Mutex
    data map[string]map[time.Time]int
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{data: make(map[string]map[time.Time]int)}
}

func (s *MemoryStore) Increment(username string, minute time.Time) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    m := minute.Truncate(time.Minute)
    u, ok := s.data[username]
    if !ok {
        u = make(map[time.Time]int)
        s.data[username] = u
    }
    u[m] = u[m] + 1
    return nil
}

func (s *MemoryStore) Get(username string, minute time.Time) (int, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    m := minute.Truncate(time.Minute)
    u, ok := s.data[username]
    if !ok {
        return 0, nil
    }
    return u[m], nil
}

