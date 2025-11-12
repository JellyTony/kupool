package stats

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

type PGStore struct{ db *sql.DB }

func NewPGStore(dsn string) (*PGStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &PGStore{db: db}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PGStore) ensureSchema() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS submissions (
  username VARCHAR(255),
  timestamp TIMESTAMP,
  submission_count INT,
  PRIMARY KEY (username, timestamp)
);`)
	return err
}

func (s *PGStore) Increment(username string, minute time.Time) error {
	m := minute.Truncate(time.Minute)
	_, err := s.db.Exec(`
INSERT INTO submissions(username, timestamp, submission_count)
VALUES($1, date_trunc('minute', $2), 1)
ON CONFLICT (username, timestamp)
DO UPDATE SET submission_count = submissions.submission_count + 1;
`, username, m)
	return err
}

func (s *PGStore) Get(username string, minute time.Time) (int, error) {
	m := minute.Truncate(time.Minute)
	var cnt int
	err := s.db.QueryRow(`SELECT submission_count FROM submissions WHERE username=$1 AND timestamp=date_trunc('minute', $2)`, username, m).Scan(&cnt)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return cnt, err
}
