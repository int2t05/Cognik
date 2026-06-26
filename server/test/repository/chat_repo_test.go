//go:build integration

// Package repository_test 验证 ChatRepo 数据访问层。
//
// 测试覆盖 PLAN.md Task25 定义的 5 个方法：
// ChatSession: Create/FindByID/UpdateFeedback/ListByUser
// ChatMessage: CreateBatch
package repository_test

import (
	"context"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/gorm"
)

func setupChatTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbCfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	// 创建 users 表（FK 依赖）
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL, real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(11) NOT NULL, email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, first_login BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 创建 knowledge_bases 表（FK 依赖）
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(128) NOT NULL, description TEXT,
		rag_workspace_slug VARCHAR(128), embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INT NOT NULL DEFAULT 0, created_by BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 创建 chat_sessions 表
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, kb_id BIGINT NOT NULL,
		question TEXT NOT NULL, answer TEXT, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, feedback SMALLINT DEFAULT 0,
		duration_ms INT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 创建 chat_messages 表
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGSERIAL PRIMARY KEY, session_id BIGINT NOT NULL,
		role VARCHAR(16) NOT NULL, content TEXT NOT NULL, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return db
}

func cleanChatTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 按 FK 依赖逆序清理，避免外键约束冲突
	db.Exec("DELETE FROM chat_messages")
	db.Exec("DELETE FROM chat_sessions")
	db.Exec("DELETE FROM knowledge_chunks")       // FK → knowledge_articles
	db.Exec("DELETE FROM knowledge_articles")     // FK → knowledge_bases
	db.Exec("DELETE FROM knowledge_bases")
	db.Exec("DELETE FROM ticket_records")         // FK → tickets
	db.Exec("DELETE FROM tickets")                // FK → users
	db.Exec("DELETE FROM users WHERE username LIKE 'test_%'")
}

// createTestKB 创建测试知识库。
func createTestKB(t *testing.T, db *gorm.DB) *model.KnowledgeBase {
	t.Helper()
	kb := &model.KnowledgeBase{
		Name:            "测试知识库",
		Description:     "测试用",
		EmbeddingModel:  "text-embedding-ada-002",
		VectorDimension: 1536,
		CreatedBy:       1,
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("创建测试知识库失败: %v", err)
	}
	return kb
}

// =============================================================================
// ChatSession 测试
// =============================================================================

func TestChatRepo_CreateSession(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)
	user := createTestUser(t, db, "test_chat_create")
	kb := createTestKB(t, db)

	session := &model.ChatSession{
		UserID:     user.ID,
		KBID:       kb.ID,
		Question:   "如何重置密码？",
		Answer:     "请前往设置页面修改密码。",
		Confidence: 0.85,
		DurationMs: 320,
	}

	err := repo.Create(context.Background(), session)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if session.ID == 0 {
		t.Error("Create 后应自动填充 ID")
	}
}

func TestChatRepo_FindByID(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)
	user := createTestUser(t, db, "test_chat_find")
	kb := createTestKB(t, db)

	session := &model.ChatSession{
		UserID: user.ID, KBID: kb.ID, Question: "问题",
		Answer: "答案", Confidence: 0.9, DurationMs: 100,
	}
	requireNoErr(t, db.Create(session).Error)

	got, err := repo.FindByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if got.Question != "问题" {
		t.Errorf("期望 Question='问题', got '%s'", got.Question)
	}
	if got.UserID != user.ID {
		t.Errorf("期望 UserID=%d, got %d", user.ID, got.UserID)
	}
}

func TestChatRepo_FindByID_NotFound(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepo(db)

	got, err := repo.FindByID(context.Background(), 999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if err != gorm.ErrRecordNotFound {
		t.Errorf("期望 gorm.ErrRecordNotFound, got %v", err)
	}
	if got != nil {
		t.Error("期望 nil")
	}
}

func TestChatRepo_UpdateFeedback(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)
	user := createTestUser(t, db, "test_chat_feedback")
	kb := createTestKB(t, db)

	session := &model.ChatSession{
		UserID: user.ID, KBID: kb.ID, Question: "问题",
		Answer: "答案", Confidence: 0.7, Feedback: 0,
	}
	requireNoErr(t, db.Create(session).Error)

	err := repo.UpdateFeedback(context.Background(), session.ID, 1) // 已解决
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.ChatSession
	db.First(&updated, session.ID)
	if updated.Feedback != 1 {
		t.Errorf("期望 Feedback=1, got %d", updated.Feedback)
	}
}

func TestChatRepo_ListByUser(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)
	user := createTestUser(t, db, "test_chat_list")
	kb := createTestKB(t, db)

	// 创建 3 个会话
	for i := 0; i < 3; i++ {
		session := &model.ChatSession{
			UserID: user.ID, KBID: kb.ID, Question: "问题", Answer: "答案",
		}
		requireNoErr(t, db.Create(session).Error)
	}

	sessions, total, err := repo.ListByUser(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 3 {
		t.Errorf("期望 total=3, got %d", total)
	}
	if len(sessions) != 3 {
		t.Errorf("期望 3 条, got %d", len(sessions))
	}
}

func TestChatRepo_ListByUser_Pagination(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)
	user := createTestUser(t, db, "test_chat_page")
	kb := createTestKB(t, db)

	// 创建 5 个会话
	for i := 0; i < 5; i++ {
		session := &model.ChatSession{
			UserID: user.ID, KBID: kb.ID, Question: "问题", Answer: "答案",
		}
		requireNoErr(t, db.Create(session).Error)
	}

	// 第 1 页，每页 2 条
	sessions, total, err := repo.ListByUser(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 5 {
		t.Errorf("期望 total=5, got %d", total)
	}
	if len(sessions) != 2 {
		t.Errorf("期望第 1 页 2 条, got %d", len(sessions))
	}

	// 第 2 页，每页 2 条
	sessions, total, err = repo.ListByUser(context.Background(), user.ID, 2, 2)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("期望第 2 页 2 条, got %d", len(sessions))
	}
	_ = total
}

// =============================================================================
// ChatMessage 测试
// =============================================================================

func TestChatRepo_CreateBatch(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)

	messages := []model.ChatMessage{
		{SessionID: 1, Role: "user", Content: "如何重置密码？", CreatedAt: time.Now()},
		{SessionID: 1, Role: "assistant", Content: "请前往设置页面修改密码。", ConfidenceRaw: 0.85, CreatedAt: time.Now().Add(100 * time.Millisecond)},
	}

	err := repo.CreateBatch(context.Background(), messages)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证消息已写入
	var count int64
	db.Model(&model.ChatMessage{}).Where("session_id = ?", 1).Count(&count)
	if count != 2 {
		t.Errorf("期望 2 条消息, got %d", count)
	}

	// 验证第一条消息的 ID 已填充
	if messages[0].ID == 0 {
		t.Error("批量创建后应自动填充 ID")
	}
	if messages[1].ID == 0 {
		t.Error("批量创建后应自动填充 ID")
	}
}

func TestChatRepo_CreateBatch_Empty(t *testing.T) {
	db := setupChatTestDB(t)
	cleanChatTables(t, db)
	repo := repository.NewChatRepo(db)

	// 空切片不应报错
	err := repo.CreateBatch(context.Background(), []model.ChatMessage{})
	if err != nil {
		t.Fatalf("空切片期望无错误, got %v", err)
	}
}
