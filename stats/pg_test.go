package stats

import (
	"os"
	"testing"
	"time"
)

// TestPGStoreIncrementGet 测试基本的增加和获取功能
func TestPGStoreIncrementGet(t *testing.T) {
	// 获取PostgreSQL DSN环境变量，如果没有提供则跳过测试
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PostgreSQL DSN not provided, skipping test")
	}

	// 创建PGStore实例
	s, err := NewPGStore(dsn)
	if err != nil {
		t.Fatalf("Failed to create PGStore: %v", err)
	}
	defer s.Close()

	// 测试用的用户名和时间
	username := "test_user"
	now := time.Now()
	minute := now.Truncate(time.Minute)

	// 执行两次增加操作
	err = s.Increment(username, now)
	if err != nil {
		t.Fatalf("First Increment failed: %v", err)
	}

	err = s.Increment(username, now.Add(5*time.Second)) // 同一分钟内的另一个时间点
	if err != nil {
		t.Fatalf("Second Increment failed: %v", err)
	}

	// 验证获取的计数是否为2
	count, err := s.Get(username, minute)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// 验证下一分钟的计数是否为0（不存在）
	nextMinute := minute.Add(time.Minute)
	count, err = s.Get(username, nextMinute)
	if err != nil {
		t.Fatalf("Get for next minute failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected count 0 for next minute, got %d", count)
	}
}

// TestPGStoreAnalytics 测试分析功能
func TestPGStoreAnalytics(t *testing.T) {
	// 获取PostgreSQL DSN环境变量，如果没有提供则跳过测试
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PostgreSQL DSN not provided, skipping test")
	}

	// 创建PGStore实例
	s, err := NewPGStore(dsn)
	if err != nil {
		t.Fatalf("Failed to create PGStore: %v", err)
	}
	defer s.Close()

	// 准备测试数据
	now := time.Now()
	user1 := "user1"
	user2 := "user2"
	user3 := "user3"

	// 为不同用户在不同时间添加提交记录
	// 用户1：3次提交
	s.Increment(user1, now.Add(-5*time.Minute))
	s.Increment(user1, now.Add(-5*time.Minute+10*time.Second))
	s.Increment(user1, now.Add(-3*time.Minute))

	// 用户2：2次提交
	s.Increment(user2, now.Add(-4*time.Minute))
	s.Increment(user2, now.Add(-4*time.Minute+15*time.Second))

	// 用户3：1次提交
	s.Increment(user3, now.Add(-2*time.Minute))

	// 1. 测试GetUserSubmissionsByTimeRange
	startTime := now.Add(-6 * time.Minute)
	endTime := now.Add(-1 * time.Minute)

	user1Submissions, err := s.GetUserSubmissionsByTimeRange(user1, startTime, endTime)
	if err != nil {
		t.Fatalf("GetUserSubmissionsByTimeRange failed: %v", err)
	}

	// 用户1应该有2个时间点的提交记录
	if len(user1Submissions) != 2 {
		t.Errorf("Expected 2 submission entries for user1, got %d", len(user1Submissions))
	}

	// 2. 测试GetTopUsersBySubmissions
	topUsers, err := s.GetTopUsersBySubmissions(startTime, endTime, 2)
	if err != nil {
		t.Fatalf("GetTopUsersBySubmissions failed: %v", err)
	}

	// 验证前两个用户的排序是否正确
	if len(topUsers) < 2 {
		t.Errorf("Expected at least 2 top users, got %d", len(topUsers))
	} else if topUsers[0].Username != user1 || topUsers[0].Total != 3 {
		t.Errorf("Expected user1 with total 3 as first, got %s with %d", topUsers[0].Username, topUsers[0].Total)
	} else if topUsers[1].Username != user2 || topUsers[1].Total != 2 {
		t.Errorf("Expected user2 with total 2 as second, got %s with %d", topUsers[1].Username, topUsers[1].Total)
	}

	// 3. 测试GetTotalSubmissions
	total, err := s.GetTotalSubmissions(startTime, endTime)
	if err != nil {
		t.Fatalf("GetTotalSubmissions failed: %v", err)
	}

	if total != 6 {
		t.Errorf("Expected total submissions 6, got %d", total)
	}
}

// TestPGStoreMinuteAggregation 测试分钟级聚合功能
func TestPGStoreMinuteAggregation(t *testing.T) {
	// 获取PostgreSQL DSN环境变量，如果没有提供则跳过测试
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PostgreSQL DSN not provided, skipping test")
	}

	// 创建PGStore实例
	s, err := NewPGStore(dsn)
	if err != nil {
		t.Fatalf("Failed to create PGStore: %v", err)
	}
	defer s.Close()

	username := "aggregation_test"
	now := time.Now()
	minute := now.Truncate(time.Minute)
	nextMinute := minute.Add(time.Minute)

	// 在同一分钟内添加多个提交
	for i := 0; i < 5; i++ {
		timePoint := minute.Add(time.Duration(i*10) * time.Second)
		err = s.Increment(username, timePoint)
		if err != nil {
			t.Fatalf("Increment failed at %v: %v", timePoint, err)
		}
	}

	// 验证同一分钟的计数是否为5
	count, err := s.Get(username, minute)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected count 5 for aggregated minute, got %d", count)
	}

	// 验证下一分钟的计数是否为0
	count, err = s.Get(username, nextMinute)
	if err != nil {
		t.Fatalf("Get for next minute failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected count 0 for next minute, got %d", count)
	}
}
