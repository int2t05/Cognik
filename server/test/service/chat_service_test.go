//go:build integration

// Package service_test 验证 ChatService 业务逻辑。
//
// ChatService 使用自建 RAG 管道，在 AI/Embedding 不可用时返回兜底回答。
// 测试覆盖：会话创建/参数校验/反馈提交/详情查询/删除。
package service_test

import (
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

var chatSvcDB *gorm.DB

func init() {
	db, err := database.Init(config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	})
	if err != nil {
		panic(err)
	}
	chatSvcDB = db
}

func setupChatServiceTest(t *testing.T) (*service.ChatService, *model.KnowledgeBase) {
	t.Helper()

	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL, real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(20) NOT NULL, email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	chatSvcDB.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(128) NOT NULL, description TEXT,
		embedding_model VARCHAR(128) NOT NULL DEFAULT '', vector_dimension INT NOT NULL DEFAULT 0,
		created_by BIGINT NOT NULL,
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

	chatSvcDB.Exec("DELETE FROM chat_messages")
	chatSvcDB.Exec("DELETE FROM chat_sessions")
	chatSvcDB.Exec("DELETE FROM knowledge_bases")

	knowledgeRepo := repository.NewKnowledgeRepo(chatSvcDB)
	chatRepo := repository.NewChatRepo(chatSvcDB)
	svc := service.NewChatService(knowledgeRepo, chatRepo, nil, service.RAGDefaults{TopK: 5}, nil, nil, nil)

	kb := &model.KnowledgeBase{
		Name:            "测试知识库",
		Description:     "chat_service 测试",
		EmbeddingModel:  "bge-m3",
		VectorDimension: 1024,
		CreatedBy:       1,
	}
	if err := chatSvcDB.Create(kb).Error; err != nil {
		t.Fatalf("创建测试知识库失败: %v", err)
	}

	return svc, kb
}

// TestChatService_CreateSession_Success 验证会话容器创建成功。
func TestChatService_CreateSession_Success(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "如何重置密码？",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	if session.ID == 0 {
		t.Error("期望 Session ID 被填充")
	}
	if session.UserID != 1 {
		t.Errorf("期望 UserID=1, 实际 %d", session.UserID)
	}
}

// TestChatService_CreateSession_InvalidKB 不存在的知识库应返回错误。
func TestChatService_CreateSession_InvalidKB(t *testing.T) {
	svc, _ := setupChatServiceTest(t)

	_, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID: 999999,
	}, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestChatService_CreateSession_DefaultTitle 空标题自动填充默认值。
func TestChatService_CreateSession_DefaultTitle(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID: kb.ID,
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	if session.Question != "新会话" {
		t.Errorf("期望默认标题 '新会话', 实际 '%s'", session.Question)
	}
}

// TestChatService_SubmitFeedback 提交反馈后会话反馈值正确更新。
func TestChatService_SubmitFeedback(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "测试问题",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	err = svc.SubmitFeedback(bgCtx, session.ID, 1, 1)
	if err != nil {
		t.Fatalf("SubmitFeedback 失败: %v", err)
	}

	detail, err := svc.GetChatDetail(bgCtx, session.ID, 1)
	if err != nil {
		t.Fatalf("GetChatDetail 失败: %v", err)
	}
	if detail.Feedback != 1 {
		t.Errorf("期望 Feedback=1, 实际 %d", detail.Feedback)
	}
}

// TestChatService_SubmitFeedback_InvalidValue 非法反馈值应返回错误。
func TestChatService_SubmitFeedback_InvalidValue(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "测试",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	err = svc.SubmitFeedback(bgCtx, session.ID, 1, 5)
	if err == nil {
		t.Fatal("非法反馈值(5)应返回错误")
	}
}

// TestChatService_SubmitFeedback_WrongUser 其他用户不能提交反馈。
func TestChatService_SubmitFeedback_WrongUser(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "水平越权测试",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	err = svc.SubmitFeedback(bgCtx, session.ID, 2, 1)
	if err == nil {
		t.Fatal("用户2 无权提交用户1 的反馈")
	}
}

// TestChatService_GetChatDetail_NotFound 不存在的会话应返回错误。
func TestChatService_GetChatDetail_NotFound(t *testing.T) {
	svc, _ := setupChatServiceTest(t)

	_, err := svc.GetChatDetail(bgCtx, 999999, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestChatService_GetChatDetail_WrongUser 其他用户不能查看会话。
func TestChatService_GetChatDetail_WrongUser(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "归属校验测试",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	_, err = svc.GetChatDetail(bgCtx, session.ID, 2)
	if err == nil {
		t.Fatal("用户2 无权查看用户1 的会话")
	}
}

// TestChatService_ListSessions 验证会话列表查询。
func TestChatService_ListSessions(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	for i := 0; i < 3; i++ {
		svc.CreateSession(bgCtx, request.CreateSessionRequest{
			KBID:  kb.ID,
			Title: "列表测试",
		}, 1)
	}

	sessions, total, err := svc.ListSessions(bgCtx, 1, 1, 10)
	if err != nil {
		t.Fatalf("ListSessions 失败: %v", err)
	}
	if total < 3 {
		t.Errorf("期望 total>=3, 实际 %d", total)
	}
	if len(sessions) < 3 {
		t.Errorf("期望 >=3 条, 实际 %d", len(sessions))
	}
}

// TestChatService_DeleteSession 验证删除会话。
func TestChatService_DeleteSession(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "待删除",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	err = svc.DeleteSession(bgCtx, session.ID, 1)
	if err != nil {
		t.Fatalf("DeleteSession 失败: %v", err)
	}

	_, err = svc.GetChatDetail(bgCtx, session.ID, 1)
	if err == nil {
		t.Fatal("删除后查询应返回错误")
	}
}

// TestChatService_DeleteSession_WrongUser 其他用户不能删除他人会话。
func TestChatService_DeleteSession_WrongUser(t *testing.T) {
	svc, kb := setupChatServiceTest(t)

	session, err := svc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID:  kb.ID,
		Title: "越权删除测试",
	}, 1)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	err = svc.DeleteSession(bgCtx, session.ID, 2)
	if err == nil {
		t.Fatal("用户2 无权删除用户1 的会话")
	}
}
