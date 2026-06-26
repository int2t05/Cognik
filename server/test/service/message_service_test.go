//go:build integration

// Package service_test 验证 MessageService 业务逻辑。
//
// 测试覆盖 PLAN.md Task29 定义的全部方法：
// ListMessages / MarkAsRead / CountUnread / NotifySupplement
package service_test

import (
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

var msgSvcDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	msgSvcDB = db
}

func setupMessageService(t *testing.T) *service.MessageService {
	t.Helper()

	msgSvcDB.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL, type VARCHAR(32) NOT NULL, related_type VARCHAR(32),
		related_id BIGINT, is_read BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理旧数据
	msgSvcDB.Exec("DELETE FROM messages")

	repo := repository.NewMessageRepo(msgSvcDB)
	return service.NewMessageService(repo)
}

// =============================================================================
// NotifySupplement
// =============================================================================

func TestMessageService_NotifySupplement(t *testing.T) {
	svc := setupMessageService(t)

	err := svc.NotifySupplement(bgCtx, 100, 42, "测试申告标题")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证消息已创建
	var msg model.Message
	if err := msgSvcDB.Where("user_id = ? AND type = ?", 42, "ticket_supplement").First(&msg).Error; err != nil {
		t.Fatalf("消息应已创建: %v", err)
	}
	if msg.Title != "申告需补充信息" {
		t.Errorf("期望 title='申告需补充信息', got '%s'", msg.Title)
	}
	if msg.RelatedType != "ticket" {
		t.Errorf("期望 related_type='ticket', got '%s'", msg.RelatedType)
	}
	if msg.RelatedID != 100 {
		t.Errorf("期望 related_id=100, got %d", msg.RelatedID)
	}
	if msg.IsRead {
		t.Error("新消息 IsRead 应为 false")
	}
}

// =============================================================================
// CountUnread
// =============================================================================

func TestMessageService_CountUnread(t *testing.T) {
	svc := setupMessageService(t)

	// 创建 3 条未读 + 1 条已读
	now := time.Now()
	msgSvcDB.Create(&model.Message{UserID: 1, Title: "A", Content: "a", Type: "test", IsRead: false, CreatedAt: now})
	msgSvcDB.Create(&model.Message{UserID: 1, Title: "B", Content: "b", Type: "test", IsRead: false, CreatedAt: now})
	msgSvcDB.Create(&model.Message{UserID: 1, Title: "C", Content: "c", Type: "test", IsRead: false, CreatedAt: now})
	msgSvcDB.Create(&model.Message{UserID: 1, Title: "D", Content: "d", Type: "test", IsRead: true, CreatedAt: now})
	// 其他用户的未读消息（不应计入）
	msgSvcDB.Create(&model.Message{UserID: 99, Title: "E", Content: "e", Type: "test", IsRead: false, CreatedAt: now})

	count, err := svc.CountUnread(bgCtx, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if count != 3 {
		t.Errorf("期望 count=3, got %d", count)
	}
}

func TestMessageService_CountUnread_Zero(t *testing.T) {
	svc := setupMessageService(t)

	count, err := svc.CountUnread(bgCtx, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if count != 0 {
		t.Errorf("期望 count=0, got %d", count)
	}
}

func TestMessageService_CountUnreadCacheInvalidatesOnMarkAsRead(t *testing.T) {
	setupMessageService(t)
	repo := repository.NewMessageRepo(msgSvcDB)
	svc := service.NewMessageServiceWithCacheTTL(repo, time.Minute)

	now := time.Now()
	first := &model.Message{UserID: 1, Title: "A", Content: "a", Type: "test", IsRead: false, CreatedAt: now}
	msgSvcDB.Create(first)

	count, err := svc.CountUnread(bgCtx, 1)
	if err != nil {
		t.Fatalf("CountUnread failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("first CountUnread = %d, want 1", count)
	}

	second := &model.Message{UserID: 1, Title: "B", Content: "b", Type: "test", IsRead: false, CreatedAt: now}
	msgSvcDB.Create(second)

	cached, err := svc.CountUnread(bgCtx, 1)
	if err != nil {
		t.Fatalf("cached CountUnread failed: %v", err)
	}
	if cached != 1 {
		t.Fatalf("cached CountUnread = %d, want stale cached value 1", cached)
	}

	if err := svc.MarkAsRead(bgCtx, first.ID, 1); err != nil {
		t.Fatalf("MarkAsRead failed: %v", err)
	}

	refreshed, err := svc.CountUnread(bgCtx, 1)
	if err != nil {
		t.Fatalf("refreshed CountUnread failed: %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed CountUnread = %d, want 1 unread message after invalidation", refreshed)
	}
}

func TestMessageService_CountUnreadCacheInvalidatesOnNotifySupplement(t *testing.T) {
	setupMessageService(t)
	repo := repository.NewMessageRepo(msgSvcDB)
	svc := service.NewMessageServiceWithCacheTTL(repo, time.Minute)

	count, err := svc.CountUnread(bgCtx, 42)
	if err != nil {
		t.Fatalf("CountUnread failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("initial CountUnread = %d, want 0", count)
	}

	if err := svc.NotifySupplement(bgCtx, 100, 42, "ticket title"); err != nil {
		t.Fatalf("NotifySupplement failed: %v", err)
	}

	refreshed, err := svc.CountUnread(bgCtx, 42)
	if err != nil {
		t.Fatalf("refreshed CountUnread failed: %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed CountUnread = %d, want 1 after cache invalidation", refreshed)
	}
}

// =============================================================================
// MarkAsRead
// =============================================================================

func TestMessageService_MarkAsRead(t *testing.T) {
	svc := setupMessageService(t)

	now := time.Now()
	msg := &model.Message{UserID: 1, Title: "测试", Content: "内容", Type: "test", IsRead: false, CreatedAt: now}
	msgSvcDB.Create(msg)

	err := svc.MarkAsRead(bgCtx, msg.ID, msg.UserID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.Message
	msgSvcDB.First(&updated, msg.ID)
	if !updated.IsRead {
		t.Error("MarkAsRead 后 IsRead 应为 true")
	}
}

func TestMessageService_MarkAsRead_NotFound(t *testing.T) {
	svc := setupMessageService(t)

	err := svc.MarkAsRead(bgCtx, 999999, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestMessageService_MarkAsRead_WrongOwner 验证跨用户标记已读被拒绝。
//
// 用户 A 的消息不应被用户 B 标记为已读，防止水平越权。
func TestMessageService_MarkAsRead_WrongOwner(t *testing.T) {
	svc := setupMessageService(t)

	now := time.Now()
	msg := &model.Message{UserID: 1, Title: "用户1的消息", Content: "内容", Type: "test", IsRead: false, CreatedAt: now}
	msgSvcDB.Create(msg)

	// 用户 2 尝试标记用户 1 的消息为已读 — 应被拒绝
	err := svc.MarkAsRead(bgCtx, msg.ID, 2)
	if err == nil {
		t.Fatal("跨用户标记已读应返回错误, got nil")
	}
}

// =============================================================================
// ListMessages
// =============================================================================

func TestMessageService_ListMessages(t *testing.T) {
	svc := setupMessageService(t)

	now := time.Now()
	for i := 0; i < 3; i++ {
		msgSvcDB.Create(&model.Message{
			UserID: 1, Title: "消息", Content: "内容", Type: "test",
			IsRead: false, CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	msgs, total, err := svc.ListMessages(bgCtx, 1, 1, 10, service.MessageFilter{})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 3 {
		t.Errorf("期望 total=3, got %d", total)
	}
	if len(msgs) != 3 {
		t.Errorf("期望 3 条, got %d", len(msgs))
	}
	// 验证按时间倒序（最新的在前）
	if len(msgs) >= 2 && msgs[0].CreatedAt.Before(msgs[1].CreatedAt) {
		t.Error("期望按 created_at DESC 排序")
	}
}

func TestMessageService_ListMessages_Pagination(t *testing.T) {
	svc := setupMessageService(t)

	now := time.Now()
	for i := 0; i < 5; i++ {
		msgSvcDB.Create(&model.Message{
			UserID: 1, Title: "消息", Content: "内容", Type: "test",
			IsRead: false, CreatedAt: now,
		})
	}

	msgs, total, err := svc.ListMessages(bgCtx, 1, 1, 2, service.MessageFilter{})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 5 {
		t.Errorf("期望 total=5, got %d", total)
	}
	if len(msgs) != 2 {
		t.Errorf("期望第 1 页 2 条, got %d", len(msgs))
	}
}
