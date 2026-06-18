//go:build integration

// Package service_test 验证 ChatService 业务逻辑。
//
// ChatService 使用自建 RAG 管道（rag.Pipeline），在 AI/Embedding 不可用时返回兜底回答。
//
// 保留测试：参数校验、会话持久化、反馈提交、详情查询。
package service_test

import (
	"fmt"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

// =============================================================================
// 测试基础设施
// =============================================================================

var chatSvcDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	chatSvcDB = db
}

func setupChatServiceTest(t *testing.T) (*service.ChatService, *model.KnowledgeBase) {
	t.Helper()

	// 创建表
	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL, real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(11) NOT NULL, email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, first_login BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(128) NOT NULL, description TEXT,
		rag_workspace_slug VARCHAR(128), embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INT NOT NULL DEFAULT 0, created_by BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
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

	// 清理旧数据（按 FK 依赖逆序）
	chatSvcDB.Exec("DELETE FROM chat_messages")
	chatSvcDB.Exec("DELETE FROM chat_sessions")
	chatSvcDB.Exec("DELETE FROM knowledge_chunks")
	chatSvcDB.Exec("DELETE FROM knowledge_articles")
	chatSvcDB.Exec("DELETE FROM knowledge_bases")

	knowledgeRepo := repository.NewKnowledgeRepo(chatSvcDB)
	chatRepo := repository.NewChatRepo(chatSvcDB)
	svc := service.NewChatService(knowledgeRepo, chatRepo, nil, service.RAGDefaults{TopK: 5})

	// 创建测试知识库（使用唯一 slug 避免冲突）
	kb := &model.KnowledgeBase{
		Name:             "运维知识库",
		Description:      "测试",
		RAGWorkspaceSlug: fmt.Sprintf("ops-workspace-%d", time.Now().UnixNano()),
		EmbeddingModel:   "text-embedding-ada-002",
		VectorDimension:  1536,
		CreatedBy:        1,
	}
	if err := chatSvcDB.Create(kb).Error; err != nil {
		t.Fatalf("创建测试知识库失败: %v", err)
	}

	return svc, kb
}

// =============================================================================
// CreateChatSession
// =============================================================================

// TestChatService_CreateChatSession_Success 验证问答会话创建成功。
func TestChatService_CreateChatSession_Success(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	req := request.CreateChatRequest{
		Question: "如何重置密码？",
		KBID:     kb.ID,
	}

	resp, err := svc.CreateChatSession(req, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if resp.Answer == "" {
		t.Error("期望非空 Answer")
	}
	if resp.Question != "如何重置密码？" {
		t.Errorf("期望 Question 保持原值, got '%s'", resp.Question)
	}
	if !resp.CanSubmitTicket {
		t.Error("CanSubmitTicket 应为 true")
	}
	if resp.SessionID == 0 {
		t.Error("应填充 SessionID")
	}
}

// TestChatService_CreateChatSession_LowConfidence 验证兜底回答场景。
func TestChatService_CreateChatSession_LowConfidence(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	req := request.CreateChatRequest{Question: "复杂问题", KBID: kb.ID}
	resp, err := svc.CreateChatSession(req, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if !resp.CanSubmitTicket {
		t.Error("CanSubmitTicket 应为 true")
	}
}

// TestChatService_CreateChatSession_InvalidKB 不存在的知识库应返回错误。
func TestChatService_CreateChatSession_InvalidKB(t *testing.T) {
	svc, _ := setupChatServiceTest(t)

	req := request.CreateChatRequest{Question: "问题", KBID: 999999}
	_, err := svc.CreateChatSession(req, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestChatService_CreateChatSession_EmptyQuestion 空问题应返回参数校验错误。
func TestChatService_CreateChatSession_EmptyQuestion(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	req := request.CreateChatRequest{Question: "", KBID: kb.ID}
	_, err := svc.CreateChatSession(req, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// =============================================================================
// SubmitFeedback
// =============================================================================

// TestChatService_SubmitFeedback 提交反馈后会话反馈值正确更新。
func TestChatService_SubmitFeedback(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	// 先创建会话
	resp, err := svc.CreateChatSession(request.CreateChatRequest{Question: "问题", KBID: kb.ID}, 1)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	// 提交反馈
	err = svc.SubmitFeedback(resp.SessionID, 1) // 1 = 已解决
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证反馈已更新
	detail, err := svc.GetChatDetail(resp.SessionID)
	if err != nil {
		t.Fatalf("查询详情失败: %v", err)
	}
	if detail.Feedback != 1 {
		t.Errorf("期望 Feedback=1, got %d", detail.Feedback)
	}
}

// =============================================================================
// GetChatDetail
// =============================================================================

// TestChatService_GetChatDetail_NotFound 不存在的会话应返回错误。
func TestChatService_GetChatDetail_NotFound(t *testing.T) {
	svc, _ := setupChatServiceTest(t)

	_, err := svc.GetChatDetail(999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}
