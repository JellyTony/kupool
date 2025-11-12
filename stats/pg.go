package stats

import (
	"time"

	"github.com/JellyTony/kupool/app/server"
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
	if err = s.ensureSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PGStore) ensureSchema() error {
	return s.db.AutoMigrate(&Submission{})
}

func (s *PGStore) Increment(username string, minute time.Time) error {
	m := minute.Truncate(time.Minute)

	// 使用upsert来增加提交计数
	// 修复列引用歧义问题，在gorm.Expr中明确指定表名
	sql := s.db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}, {Name: "timestamp"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"submission_count": gorm.Expr("excluded.submission_count + 1")}),
		}).Create(&Submission{
			Username:        username,
			Timestamp:       m,
			SubmissionCount: 1,
		})
	})

	return s.db.Exec(sql).Error
}

func (s *PGStore) Get(username string, minute time.Time) (int, error) {
	m := minute.Truncate(time.Minute)
	var rec Submission
	err := s.db.Where("username = ? AND timestamp = ?", username, m).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return rec.SubmissionCount, err
}

func (s *PGStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetUserSubmissionsByTimeRange 获取用户在指定时间范围内的提交计数
func (s *PGStore) GetUserSubmissionsByTimeRange(username string, startTime, endTime time.Time) (map[time.Time]int, error) {
	var submissions []Submission
	err := s.db.Where("username = ? AND timestamp >= ? AND timestamp <= ?", username, startTime, endTime).
		Order("timestamp ASC").Find(&submissions).Error
	if err != nil {
		return nil, err
	}

	result := make(map[time.Time]int)
	for _, sub := range submissions {
		result[sub.Timestamp] = sub.SubmissionCount
	}
	return result, nil
}

// GetTopUsersBySubmissions 获取指定时间范围内提交次数最多的用户
func (s *PGStore) GetTopUsersBySubmissions(startTime, endTime time.Time, limit int) ([]struct {
	Username string
	Total    int
}, error) {
	type Result struct {
		Username string
		Total    int
	}
	var results []Result

	err := s.db.Model(&Submission{}).
		Select("username, SUM(submission_count) as total").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("username").
		Order("total DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// 转换为所需的返回类型
	var finalResults []struct {
		Username string
		Total    int
	}
	for _, r := range results {
		finalResults = append(finalResults, struct {
			Username string
			Total    int
		}{Username: r.Username, Total: r.Total})
	}

	return finalResults, nil
}

// GetTotalSubmissions 获取指定时间范围内的总提交次数
func (s *PGStore) GetTotalSubmissions(startTime, endTime time.Time) (int, error) {
	var total int
	err := s.db.Model(&Submission{}).
		Select("COALESCE(SUM(submission_count), 0)").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Scan(&total).Error
	return total, err
}

// StateStore models
type JobHistory struct {
	JobID       int       `gorm:"primaryKey"`
	ServerNonce string    `gorm:"size:64;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

type UserState struct {
	Username          string `gorm:"primaryKey;size:255"`
	LatestJobID       int    `gorm:"not null"`
	LatestServerNonce string `gorm:"size:64"`
	LastSubmitAt      time.Time
}

type UsedNonce struct {
	Username    string `gorm:"primaryKey;size:255"`
	JobID       int    `gorm:"primaryKey"`
	ClientNonce string `gorm:"primaryKey;size:64"`
}

// StateStore impl
func (s *PGStore) SaveJob(jobID int, nonce string, createdAt time.Time) error {
	_ = s.db.AutoMigrate(&JobHistory{})
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&JobHistory{JobID: jobID, ServerNonce: nonce, CreatedAt: createdAt}).Error
}

func (s *PGStore) LoadLatestJob() (int, string, time.Time, error) {
	_ = s.db.AutoMigrate(&JobHistory{})
	var j JobHistory
	err := s.db.Order("job_id DESC").First(&j).Error
	if err == gorm.ErrRecordNotFound {
		return 0, "", time.Time{}, nil
	}
	return j.JobID, j.ServerNonce, j.CreatedAt, err
}

func (s *PGStore) LoadJobHistory(since time.Duration) (map[int]server.JobRecord, error) {
	_ = s.db.AutoMigrate(&JobHistory{})
	var js []JobHistory
	q := s.db
	if since > 0 {
		q = q.Where("created_at >= ?", time.Now().Add(-since))
	}
	if err := q.Find(&js).Error; err != nil {
		return nil, err
	}
	out := make(map[int]server.JobRecord)
	for _, j := range js {
		out[j.JobID] = server.JobRecord{Nonce: j.ServerNonce, CreatedAt: j.CreatedAt}
	}
	return out, nil
}

func (s *PGStore) LoadUserState(username string) (int, string, time.Time, error) {
	_ = s.db.AutoMigrate(&UserState{})
	var u UserState
	err := s.db.Where("username = ?", username).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return 0, "", time.Time{}, nil
	}
	return u.LatestJobID, u.LatestServerNonce, u.LastSubmitAt, err
}

func (s *PGStore) SaveUserState(username string, latestJobID int, latestServerNonce string, lastSubmitAt time.Time) error {
	_ = s.db.AutoMigrate(&UserState{})
	return s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoUpdates: clause.Assignments(map[string]interface{}{"latest_job_id": latestJobID, "latest_server_nonce": latestServerNonce, "last_submit_at": lastSubmitAt})}).Create(&UserState{Username: username, LatestJobID: latestJobID, LatestServerNonce: latestServerNonce, LastSubmitAt: lastSubmitAt}).Error
}

func (s *PGStore) SaveUsedNonce(username string, jobID int, clientNonce string) error {
	_ = s.db.AutoMigrate(&UsedNonce{})
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&UsedNonce{Username: username, JobID: jobID, ClientNonce: clientNonce}).Error
}

func (s *PGStore) HasUsedNonce(username string, jobID int, clientNonce string) (bool, error) {
	_ = s.db.AutoMigrate(&UsedNonce{})
	var u UsedNonce
	err := s.db.Where("username = ? AND job_id = ? AND client_nonce = ?", username, jobID, clientNonce).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return err == nil, err
}
