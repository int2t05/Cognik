//go:build integration

// Package repository_test 验证 MessageRepo 数据访问层。
package repository_test

import (
	"context"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/gorm"
)

func setupMessageTestDB(t *testing.T) *gorm.DB {
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

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, type VARCHAR(32) NOT NULL,
		related_type VARCHAR(32), related_id BIGINT, title VARCHAR(255) NOT NULL,
		content TEXT, is_read BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 清空所有消息，避免其他测试残留数据干扰
	db.Exec("DELETE FROM messages")
	return db
}

func TestMessageRepo_Create(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	msg := &model.Message{
		UserID: 1, Type: "ticket_status", Title: "test_create_msg", Content: "测试内容",
	}
	err := repo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if msg.ID == 0 {
		t.Error("期望 ID 被填充")
	}
}

func TestMessageRepo_ListByUser(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, is_read, created_at) VALUES
		(1, 'ticket_status', 'test_list_msg1', '内容1', false, NOW()),
		(1, 'ticket_status', 'test_list_msg2', '内容2', true, NOW()),
		(2, 'ticket_status', 'test_list_msg3', '内容3', false, NOW())`)

	msgs, total, err := repo.ListByUser(ctx, 1, 1, 10, repository.MessageFilter{})
	if err != nil {
		t.Fatalf("ListByUser 失败: %v", err)
	}
	if total != 2 {
		t.Errorf("用户1: 期望 total=2, 实际 %d", total)
	}
	if len(msgs) != 2 {
		t.Errorf("用户1: 期望 2 条, 实际 %d", len(msgs))
	}
}

func TestMessageRepo_ListByUser_UnreadFilter(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, is_read, created_at) VALUES
		(1, 'ticket_status', 'test_unread_msg', '内容', false, NOW()),
		(1, 'ticket_status', 'test_read_msg', '内容', true, NOW())`)

	isRead := false
	msgs, total, err := repo.ListByUser(ctx, 1, 1, 10, repository.MessageFilter{IsRead: &isRead})
	if err != nil {
		t.Fatalf("ListByUser 失败: %v", err)
	}
	if total != 1 {
		t.Errorf("未读过滤: 期望 total=1, 实际 %d", total)
	}
	_ = msgs
}

func TestMessageRepo_ListByUser_TypeFilter(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, created_at) VALUES
		(1, 'ticket_status', 'test_type_msg1', '内容1', NOW()),
		(1, 'ticket_supplement', 'test_type_msg2', '内容2', NOW())`)

	msgs, total, err := repo.ListByUser(ctx, 1, 1, 10, repository.MessageFilter{Type: "ticket_supplement"})
	if err != nil {
		t.Fatalf("ListByUser 失败: %v", err)
	}
	if total != 1 {
		t.Errorf("类型过滤: 期望 total=1, 实际 %d", total)
	}
	_ = msgs
}

func TestMessageRepo_MarkAsRead(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, is_read, created_at) VALUES (1, 'ticket_status', 'test_mark_read', '内容', false, NOW())`)
	var id int64
	db.Raw("SELECT id FROM messages WHERE title = 'test_mark_read'").Scan(&id)

	err := repo.MarkAsRead(ctx, id, 1)
	if err != nil {
		t.Fatalf("MarkAsRead 失败: %v", err)
	}

	// 验证已读
	isRead := true
	msgs, _, _ := repo.ListByUser(ctx, 1, 1, 10, repository.MessageFilter{IsRead: &isRead})
	found := false
	for _, m := range msgs {
		if m.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("标记已读后应在已读列表中")
	}
}

func TestMessageRepo_MarkAsRead_WrongUser(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, created_at) VALUES (1, 'ticket_status', 'test_wrong_user', '内容', NOW())`)
	var id int64
	db.Raw("SELECT id FROM messages WHERE title = 'test_wrong_user'").Scan(&id)

	// 用户 2 标记用户 1 的消息为已读 → 应失败
	err := repo.MarkAsRead(ctx, id, 2)
	if err == nil {
		t.Fatal("水平越权标记应失败")
	}
}

func TestMessageRepo_CountUnread(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := repository.NewMessageRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO messages (user_id, type, title, content, is_read, created_at) VALUES
		(1, 'ticket_status', 'test_count_1', '内容', false, NOW()),
		(1, 'ticket_status', 'test_count_2', '内容', false, NOW()),
		(1, 'ticket_status', 'test_count_3', '内容', true, NOW())`)

	count, err := repo.CountUnread(ctx, 1)
	if err != nil {
		t.Fatalf("CountUnread 失败: %v", err)
	}
	if count != 2 {
		t.Errorf("期望 2 条未读, 实际 %d", count)
	}
}
