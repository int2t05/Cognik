//go:build integration

// Package repository_test 验证 DashboardRepo 数据访问层。
package repository_test

import (
	"context"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/repository"

	"gorm.io/gorm"
)

func setupDashboardRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	port, _ := strconv.Atoi(getEnv("TEST_DB_PORT", "5432"))
	db, err := database.Init(config.DatabaseConfig{
		Host: getEnv("TEST_DB_HOST", "localhost"), Port: port,
		User: getEnv("TEST_DB_USER", "opsmind"), Password: getEnv("TEST_DB_PASSWORD", "opsmind_dev"),
		DBName: getEnv("TEST_DB_NAME", "opsmind_test"), SSLMode: getEnv("TEST_DB_SSLMODE", "disable"),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY, ticket_no VARCHAR(32) NOT NULL, user_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL, description TEXT NOT NULL, urgency SMALLINT NOT NULL DEFAULT 1,
		status SMALLINT NOT NULL DEFAULT 1, contact_phone VARCHAR(20) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, question TEXT NOT NULL,
		answer TEXT, confidence FLOAT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_articles (
		id BIGSERIAL PRIMARY KEY, kb_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec("DELETE FROM tickets WHERE title LIKE 'test_dash_%'")
	db.Exec("DELETE FROM chat_sessions WHERE question LIKE 'test_dash_%'")
	db.Exec("DELETE FROM knowledge_articles WHERE title LIKE 'test_dash_%'")
	return db
}

func TestDashboardRepo_CountTodayTickets(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO tickets (ticket_no, user_id, title, description, contact_phone, status, created_at) VALUES
		('TK-DASH-001', 1, 'test_dash_today1', 'd', '13800000001', 1, NOW()),
		('TK-DASH-002', 1, 'test_dash_today2', 'd', '13800000001', 2, NOW())`)

	count, err := repo.CountTodayTickets(ctx)
	if err != nil {
		t.Fatalf("CountTodayTickets 失败: %v", err)
	}
	if count < 2 {
		t.Errorf("期望 count>=2, 实际 %d", count)
	}
}

func TestDashboardRepo_CountByStatus(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO tickets (ticket_no, user_id, title, description, contact_phone, status) VALUES
		('TK-DASH-S1', 1, 'test_dash_stat1', 'd', '13800000001', 1),
		('TK-DASH-S2', 1, 'test_dash_stat2', 'd', '13800000001', 1),
		('TK-DASH-S3', 1, 'test_dash_stat3', 'd', '13800000001', 2)`)

	count1, err := repo.CountByStatus(ctx, 1)
	if err != nil {
		t.Fatalf("CountByStatus 1 失败: %v", err)
	}
	if count1 < 2 {
		t.Errorf("status=1: 期望 count>=2, 实际 %d", count1)
	}

	count2, err := repo.CountByStatus(ctx, 2)
	if err != nil {
		t.Fatalf("CountByStatus 2 失败: %v", err)
	}
	if count2 < 1 {
		t.Errorf("status=2: 期望 count>=1, 实际 %d", count2)
	}
}

func TestDashboardRepo_CountTodayChats(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO chat_sessions (user_id, question, answer, confidence)
		VALUES (1, 'test_dash_chat1', 'answer1', 0.8), (1, 'test_dash_chat2', 'answer2', 0.6)`)

	count, err := repo.CountTodayChats(ctx)
	if err != nil {
		t.Fatalf("CountTodayChats 失败: %v", err)
	}
	if count < 2 {
		t.Errorf("期望 count>=2, 实际 %d", count)
	}
}

func TestDashboardRepo_AvgTodayConfidence(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO chat_sessions (user_id, question, answer, confidence)
		VALUES (1, 'test_dash_avg1', 'a', 0.9), (1, 'test_dash_avg2', 'a', 0.5)`)

	avg, err := repo.AvgTodayConfidence(ctx)
	if err != nil {
		t.Fatalf("AvgTodayConfidence 失败: %v", err)
	}
	if avg <= 0 {
		t.Errorf("期望 avg>0, 实际 %f", avg)
	}
}

func TestDashboardRepo_CountKnowledgeArticles(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO knowledge_articles (kb_id, title, content, status)
		VALUES (1, 'test_dash_art1', 'c1', 1), (1, 'test_dash_art2', 'c2', 2)`)

	count, err := repo.CountKnowledgeArticles(ctx)
	if err != nil {
		t.Fatalf("CountKnowledgeArticles 失败: %v", err)
	}
	if count < 2 {
		t.Errorf("期望 count>=2, 实际 %d", count)
	}
}

func TestDashboardRepo_GetTicketTrends(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO tickets (ticket_no, user_id, title, description, contact_phone, status)
		VALUES ('TK-DASH-T1', 1, 'test_dash_trend', 'd', '13800000001', 1)`)

	points, err := repo.GetTicketTrends(ctx,
		"2000-01-01", "2099-12-31", "day")
	if err != nil {
		t.Fatalf("GetTicketTrends 失败: %v", err)
	}
	if len(points) == 0 {
		t.Error("期望至少 1 个数据点")
	}
}

func TestDashboardRepo_GetChatTrends(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO chat_sessions (user_id, question, answer)
		VALUES (1, 'test_dash_chattrend', 'a')`)

	points, err := repo.GetChatTrends(ctx,
		"2000-01-01", "2099-12-31", "day")
	if err != nil {
		t.Fatalf("GetChatTrends 失败: %v", err)
	}
	if len(points) == 0 {
		t.Error("期望至少 1 个数据点")
	}
}

func TestDashboardRepo_GetTicketTrends_Week(t *testing.T) {
	db := setupDashboardRepoTestDB(t)
	repo := repository.NewDashboardRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO tickets (ticket_no, user_id, title, description, contact_phone, status)
		VALUES ('TK-DASH-W1', 1, 'test_dash_weekt', 'd', '13800000001', 1)`)

	points, err := repo.GetTicketTrends(ctx,
		"2000-01-01", "2099-12-31", "week")
	if err != nil {
		t.Fatalf("GetTicketTrends week 失败: %v", err)
	}
	if len(points) == 0 {
		t.Error("期望至少 1 个数据点")
	}
}
