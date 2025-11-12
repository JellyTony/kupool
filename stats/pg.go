package stats

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Submission struct {
	Username        string    `gorm:"primaryKey;size:255"`
	Timestamp       time.Time `gorm:"primaryKey;type:timestamp"`
	SubmissionCount int       `gorm:"not null"`
}

type PGStore struct{ db *gorm.DB }

func NewPGStore(dsn string) (*PGStore, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	s := &PGStore{db: db}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PGStore) ensureSchema() error { return s.db.AutoMigrate(&Submission{}) }

func (s *PGStore) Increment(username string, minute time.Time) error {
	m := minute.Truncate(time.Minute)
	rec := Submission{Username: username, Timestamp: m, SubmissionCount: 1}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "username"}, {Name: "timestamp"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"submission_count": gorm.Expr("submissions.submission_count + EXCLUDED.submission_count")}),
	}).Create(&rec).Error
}

func (s *PGStore) Get(username string, minute time.Time) (int, error) {
	m := minute.Truncate(time.Minute)
	var rec Submission
	err := s.db.Where("username = ? AND timestamp = date_trunc('minute', ?::timestamp)", username, m).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return rec.SubmissionCount, err
}
