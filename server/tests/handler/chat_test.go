//go:build integration

// Package handler_test 验证 ChatHandler HTTP 接口。
//
// 测试覆盖 PLAN.md Task26 定义的门户端问答端点：
// POST /portal/chat-sessions（创建问答）
// POST /portal/chat-sessions/:id/feedback（提交反馈）
// GET /portal/chat-sessions/:id（查询详情）
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	respDto "opsmind/internal/dto/response"
	"opsmind/internal/handler"
	"opsmind/internal/middleware"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// =============================================================================
// 测试环境
// =============================================================================

// chatHandlerEnv 封装 ChatHandler 测试环境。
type chatHandlerEnv struct {
	r  *gin.Engine
	db *gorm.DB
	kb *model.KnowledgeBase
}

func setupChatHandlerTest(t *testing.T) *chatHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbCfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(128) NOT NULL, description TEXT,
		rag_workspace_slug VARCHAR(128), embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INT NOT NULL DEFAULT 0, created_by BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, kb_id BIGINT NOT NULL,
		question TEXT NOT NULL, answer TEXT, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, feedback SMALLINT DEFAULT 0,
		duration_ms INT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGSERIAL PRIMARY KEY, session_id BIGINT NOT NULL,
		role VARCHAR(16) NOT NULL, content TEXT NOT NULL, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理
	db.Exec("DELETE FROM chat_messages")
	db.Exec("DELETE FROM chat_sessions")
	db.Exec("DELETE FROM knowledge_bases")

	// 创建测试知识库
	kb := &model.KnowledgeBase{
		Name:             "测试",
		RAGWorkspaceSlug: "test-workspace",
		EmbeddingModel:   "text-embedding-ada-002",
		VectorDimension:  1536,
		CreatedBy:        1,
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("创建知识库失败: %v", err)
	}

	// 组装依赖链
	knowledgeRepo := repository.NewKnowledgeRepo(db)
	chatRepo := repository.NewChatRepo(db)
	chatSvc := service.NewChatService(knowledgeRepo, chatRepo, nil, service.RAGDefaults{
		TopK: 5, QueryRewrite: true, MultiRoute: true, Hybrid: true, Rerank: true,
	})
	chatH := handler.NewChatHandler(chatSvc)

	// 路由
	r := gin.New()
	r.Use(middleware.RequestID())
	// 模拟认证中间件：注入 user_id=1
	r.Use(func(c *gin.Context) {
		c.Set("currentUser", map[string]interface{}{"user_id": float64(1)})
		c.Set("userID", int64(1))
		c.Next()
	})

	portal := r.Group("/api/v1/portal")
	{
		portal.POST("/chat-sessions/stream", chatH.StreamChatMessage)
		portal.POST("/chat-sessions", chatH.CreateChatSession)
		portal.POST("/chat-sessions/:id/feedback", chatH.SubmitFeedback)
		portal.GET("/chat-sessions/:id", chatH.GetChatDetail)
	}

	return &chatHandlerEnv{r: r, db: db, kb: kb}
}

// =============================================================================
// POST /portal/chat-sessions
// =============================================================================

func TestChatHandler_CreateSession_Success(t *testing.T) {
	env := setupChatHandlerTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"question": "网络连不上怎么办？",
		"kb_id":    env.kb.ID,
	})
	req := httptest.NewRequest("POST", "/api/v1/portal/chat-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int                      `json:"code"`
		Data respDto.ChatSessionResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.SessionID == 0 {
		t.Error("应填充 SessionID")
	}
	if resp.Data.Question != "网络连不上怎么办？" {
		t.Errorf("期望 Question='网络连不上怎么办？', got '%s'", resp.Data.Question)
	}
}

func TestChatHandler_CreateSession_LowConfidence(t *testing.T) {
	env := setupChatHandlerTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"question": "复杂问题",
		"kb_id":    env.kb.ID,
	})
	req := httptest.NewRequest("POST", "/api/v1/portal/chat-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int                      `json:"code"`
		Data respDto.ChatSessionResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Data.CanSubmitTicket {
		t.Error("兜底响应置信度为 0，CanSubmitTicket 应为 true")
	}
}

func TestChatHandler_CreateSession_MissingParams(t *testing.T) {
	env := setupChatHandlerTest(t)

	body, _ := json.Marshal(map[string]interface{}{"question": ""})
	req := httptest.NewRequest("POST", "/api/v1/portal/chat-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// 参数校验失败 → ErrParam(10003) → HTTP 400
	if resp.Code == 0 {
		t.Errorf("参数缺失应返回错误码, got code=%d", resp.Code)
	}
}

// =============================================================================
// POST /portal/chat-sessions/:id/feedback
// =============================================================================

func TestChatHandler_SubmitFeedback(t *testing.T) {
	env := setupChatHandlerTest(t)

	// 先创建一个会话
	createBody, _ := json.Marshal(map[string]interface{}{
		"question": "问题", "kb_id": env.kb.ID,
	})
	createReq := httptest.NewRequest("POST", "/api/v1/portal/chat-sessions", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	env.r.ServeHTTP(createW, createReq)

	var createResp struct {
		Data respDto.ChatSessionResponse `json:"data"`
	}
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// 提交反馈
	feedbackBody, _ := json.Marshal(map[string]interface{}{"feedback": 1})
	feedbackURL := "/api/v1/portal/chat-sessions/" + strconv.FormatInt(createResp.Data.SessionID, 10) + "/feedback"
	fbReq := httptest.NewRequest("POST", feedbackURL, bytes.NewReader(feedbackBody))
	fbReq.Header.Set("Content-Type", "application/json")
	fbW := httptest.NewRecorder()
	env.r.ServeHTTP(fbW, fbReq)

	if fbW.Code != 200 {
		t.Fatalf("期望 200, got %d: %s", fbW.Code, fbW.Body.String())
	}

	// 查询验证反馈
	detailReq := httptest.NewRequest("GET", feedbackURL[:len(feedbackURL)-len("/feedback")], nil)
	detailW := httptest.NewRecorder()
	env.r.ServeHTTP(detailW, detailReq)

	var detailResp struct {
		Data respDto.ChatSessionResponse `json:"data"`
	}
	json.Unmarshal(detailW.Body.Bytes(), &detailResp)
	if detailResp.Data.Feedback != 1 {
		t.Errorf("期望 Feedback=1, got %d", detailResp.Data.Feedback)
	}
}

// =============================================================================
// POST /portal/chat-sessions/stream (SSE)
// =============================================================================

func TestChatHandler_StreamSession(t *testing.T) {
	env := setupChatHandlerTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"question": "VPN怎么配置？",
		"kb_id":    env.kb.ID,
	})
	req := httptest.NewRequest("POST", "/api/v1/portal/chat-sessions/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证 SSE Content-Type
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("期望 Content-Type=text/event-stream, got %s", ct)
	}

	// 验证 SSE 输出包含 done 事件
	bodyStr := w.Body.String()
	if bodyStr == "" {
		t.Error("SSE 响应不应为空")
	}
	if !strings.Contains(bodyStr, `"type":"done"`) && !strings.Contains(bodyStr, "done") {
		t.Error("SSE 响应应包含 done 事件")
	}
}
