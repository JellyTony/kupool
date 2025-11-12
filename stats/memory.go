package stats

import (
    "sync"
    "time"
    serverpkg "github.com/JellyTony/kupool/app/server"
)

type MemoryStore struct {
    mu   sync.Mutex
    data map[string]map[time.Time]int
    jobHist map[int]time.Time
    latestJobID int
    latestNonce string
    userStates map[string]struct{
        latestJobID int
        latestNonce string
        lastSubmit time.Time
    }
    used map[string]map[int]map[string]struct{}
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{data: make(map[string]map[time.Time]int), jobHist: make(map[int]time.Time), userStates: make(map[string]struct{latestJobID int; latestNonce string; lastSubmit time.Time}), used: make(map[string]map[int]map[string]struct{})}
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

func (s *MemoryStore) Close() error { return nil }

// StateStore
func (s *MemoryStore) SaveJob(jobID int, nonce string, createdAt time.Time) error {
    s.mu.Lock(); defer s.mu.Unlock()
    s.latestJobID = jobID
    s.latestNonce = nonce
    s.jobHist[jobID] = createdAt
    return nil
}

func (s *MemoryStore) LoadLatestJob() (int, string, time.Time, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    return s.latestJobID, s.latestNonce, s.jobHist[s.latestJobID], nil
}

func (s *MemoryStore) LoadJobHistory(since time.Duration) (map[int]serverpkg.JobRecord, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    out := make(map[int]serverpkg.JobRecord)
    cutoff := time.Now().Add(-since)
    for id, t := range s.jobHist {
        if since == 0 || t.After(cutoff) {
            out[id] = serverpkg.JobRecord{Nonce: s.latestNonce, CreatedAt: t}
        }
    }
    return out, nil
}

func (s *MemoryStore) LoadUserState(username string) (int, string, time.Time, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    st := s.userStates[username]
    return st.latestJobID, st.latestNonce, st.lastSubmit, nil
}

func (s *MemoryStore) SaveUserState(username string, latestJobID int, latestServerNonce string, lastSubmitAt time.Time) error {
    s.mu.Lock(); defer s.mu.Unlock()
    s.userStates[username] = struct{latestJobID int; latestNonce string; lastSubmit time.Time}{latestJobID, latestServerNonce, lastSubmitAt}
    return nil
}

func (s *MemoryStore) SaveUsedNonce(username string, jobID int, clientNonce string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    u, ok := s.used[username]
    if !ok { u = make(map[int]map[string]struct{}); s.used[username] = u }
    m, ok := u[jobID]
    if !ok { m = make(map[string]struct{}); u[jobID] = m }
    m[clientNonce] = struct{}{}
    return nil
}

func (s *MemoryStore) HasUsedNonce(username string, jobID int, clientNonce string) (bool, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    u := s.used[username]
    m := u[jobID]
    _, ok := m[clientNonce]
    return ok, nil
}
