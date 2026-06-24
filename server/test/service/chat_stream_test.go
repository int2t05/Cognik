//go:build integration

// Package service_test 验证 ChatRepo 的消息写入/更新/清理。
//
// 这三个方法是可续传流式对话的基础：
// CreateMessage 在生成开始先写入一条 generating 消息并拿到 ID，
// UpdateMessage 在生成完成后回填内容，
// MarkGeneratingFailed 在服务重启后清理残留的生成中状态。
package service_test

import (
	"context"
	"testing"

	"opsmind/internal/model"
	"opsmind/internal/repository"
)

// setupChatRepoTest 准备 ChatRepo 集成测试环境。
// 复用同包的 chatSvcDB（由 auth_service_test.go 的 init() 初始化），
// 确保 chat_sessions / chat_messages 两张表存在且包含新字段。
func setupChatRepoTest(t *testing.T) *repository.ChatRepo {
	t.Helper()

	// 建表（沿用 chat_service_test.go 的 DDL，补充新字段）
	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, kb_id BIGINT NOT NULL,
		question TEXT NOT NULL, answer TEXT, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, feedback SMALLINT DEFAULT 0,
		duration_ms INT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGSERIAL PRIMARY KEY, session_id BIGINT NOT NULL,
		role VARCHAR(16) NOT NULL, content TEXT NOT NULL, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 补充 chat_messages 缺失的列（CREATE TABLE IF NOT EXISTS 不会修改已有表结构）
	chatSvcDB.Exec(`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS pipeline_metrics JSONB`)
	chatSvcDB.Exec(`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'completed'`)

	// 清理数据，避免跨用例干扰
	chatSvcDB.Exec("DELETE FROM chat_messages")
	chatSvcDB.Exec("DELETE FROM chat_sessions")

	return repository.NewChatRepo(chatSvcDB)
}

// TestChatRepo_CreateUpdateAndMarkFailed 验证消息单写、更新、残留清理三条路径。
func TestChatRepo_CreateUpdateAndMarkFailed(t *testing.T) {
	repo := setupChatRepoTest(t)
	ctx := context.Background()

	// 准备一个会话（满足外键）
	sess := &model.ChatSession{KBID: 1, UserID: 1, Question: "测试"}
	if err := chatSvcDB.Create(sess).Error; err != nil {
		t.Fatalf("建会话失败: %v", err)
	}

	// --- CreateMessage: 写入并回填 ID ---
	m := &model.ChatMessage{
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   "",
		Status:    model.MessageStatusGenerating,
	}
	if err := repo.CreateMessage(ctx, m); err != nil || m.ID == 0 {
		t.Fatalf("CreateMessage 应回填 ID: err=%v id=%d", err, m.ID)
	}

	// --- UpdateMessage: 按主键全量更新 ---
	m.Content = "完整答案"
	m.Status = model.MessageStatusCompleted
	if err := repo.UpdateMessage(ctx, m); err != nil {
		t.Fatalf("UpdateMessage 失败: %v", err)
	}

	// 回读验证 UpdateMessage 是否持久化
	var reloaded model.ChatMessage
	if err := chatSvcDB.First(&reloaded, m.ID).Error; err != nil {
		t.Fatalf("回读消息失败: %v", err)
	}
	if reloaded.Content != "完整答案" {
		t.Errorf("期望 Content='完整答案', 实际 '%s'", reloaded.Content)
	}
	if reloaded.Status != model.MessageStatusCompleted {
		t.Errorf("期望 Status='completed', 实际 '%s'", reloaded.Status)
	}

	// --- MarkGeneratingFailed: 清理残留 ---
	// 再造一条残留 generating，验证清理
	stale := &model.ChatMessage{
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   "半截",
		Status:    model.MessageStatusGenerating,
	}
	if err := repo.CreateMessage(ctx, stale); err != nil {
		t.Fatalf("CreateMessage stale 失败: %v", err)
	}

	n, err := repo.MarkGeneratingFailed(ctx)
	if err != nil || n < 1 {
		t.Fatalf("MarkGeneratingFailed 应清理至少 1 行: n=%d err=%v", n, err)
	}

	// 验证该残留消息已被标记为 failed
	var staleReloaded model.ChatMessage
	if err := chatSvcDB.First(&staleReloaded, stale.ID).Error; err != nil {
		t.Fatalf("回读残留消息失败: %v", err)
	}
	if staleReloaded.Status != model.MessageStatusFailed {
		t.Errorf("期望 stale Status='failed', 实际 '%s'", staleReloaded.Status)
	}
}
