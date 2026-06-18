//go:build integration

// Package integration_test 提供 API 集成测试的共享基础设施。
//
// 架构：httptest.NewServer 启动完整 Gin 路由 → net/http.Client 真实调用。
// 依赖链：DB → Repo → Service → Handler → Router（与 main.go 一致）。
// 每个测试独立调用 startAPITestServer，TRUNCATE 保证隔离。
//
// 运行：go test -tags=integration ./tests/integration/ -v -run "TestAPI"
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	"opsmind/internal/repository"
	"opsmind/internal/router"
	"opsmind/internal/service"
	"opsmind/pkg/hash"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── 测试服务器 ──────────────────────────────────────────

type apiTestServer struct {
	Server     *httptest.Server
	DB         *gorm.DB
	BaseURL    string
	AdminToken string
	AdminID    int64
}

func startAPITestServer(t *testing.T) *apiTestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbCfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind",
		Password: "opsmind_dev", DBName: "opsmind_test", SSLMode: "disable",
	}
	jwtCfg := config.JWTConfig{
		Secret: "test_secret_key_2024", AccessExpire: 2 * time.Hour, RefreshExpire: 168 * time.Hour,
	}

	db, err := database.Init(dbCfg)
	require.NoError(t, err, "连接测试数据库失败")

	cleanTables(t, db)
	db.Exec("DROP INDEX IF EXISTS idx_users_phone")
	db.Exec("DROP INDEX IF EXISTS idx_users_username")

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// Repository
	userRepo, roleRepo, menuRepo := repository.NewUserRepo(db), repository.NewRoleRepo(db), repository.NewMenuRepo(db)
	ticketRepo := repository.NewTicketRepo(db)
	knowledgeRepo := repository.NewKnowledgeRepo(db)
	chatRepo, messageRepo := repository.NewChatRepo(db), repository.NewMessageRepo(db)
	auditRepo, dashboardRepo := repository.NewAuditRepo(db), repository.NewDashboardRepo(db)
	configRepo, llmConfigRepo := repository.NewConfigRepo(db), repository.NewLlmConfigRepo(db)

	// Service
	authSvc := service.NewAuthService(userRepo, menuRepo, db, jwtCfg)
	userSvc := service.NewUserService(userRepo, auditRepo, db)
	roleSvc := service.NewRoleService(roleRepo, menuRepo, auditRepo, db)
	messageSvc := service.NewMessageService(messageRepo)
	ticketSvc := service.NewTicketService(ticketRepo, service.NewGormTxManager(db), messageSvc, nil)
	dashboardSvc := service.NewDashboardService(dashboardRepo)
	configSvc := service.NewConfigService(configRepo, auditRepo)
	auditSvc := service.NewAuditService(auditRepo)

	llmConfigSvc, err := service.NewLLMConfigService(llmConfigRepo)
	require.NoError(t, err)

	knowledgeSvc := service.NewKnowledgeService(knowledgeRepo,
		service.WithUserNames(userRepo), service.WithAuditRepo(auditRepo))
	ticketSvc.SetKnowledgeService(knowledgeSvc)

	chatSvc := service.NewChatService(knowledgeRepo, chatRepo, nil, service.RAGDefaults{
		TopK: 5, QueryRewrite: false, MultiRoute: false, Hybrid: false, Rerank: false,
	})

	// Handler → Router → HTTP Server
	handlers := &router.Handlers{
		Auth: handler.NewAuthHandler(authSvc), User: handler.NewUserHandler(userSvc),
		Role: handler.NewRoleHandler(roleSvc), Ticket: handler.NewTicketHandler(ticketSvc),
		Knowledge: handler.NewKnowledgeHandler(knowledgeSvc), Chat: handler.NewChatHandler(chatSvc),
		Message: handler.NewMessageHandler(messageSvc), Dashboard: handler.NewDashboardHandler(dashboardSvc),
		Audit: handler.NewAuditHandler(auditSvc), Config: handler.NewConfigHandler(configSvc),
		LLMConfig: handler.NewLLMConfigHandler(llmConfigSvc),
	}

	r := router.Setup(&config.AppConfig{
		Server: config.ServerConfig{Mode: "debug", ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second},
		JWT:    jwtCfg, CORS: config.CORSConfig{AllowOrigins: "http://localhost:5173"},
		Database: dbCfg,
	}, db, handlers)

	srv := httptest.NewServer(r)
	ts := &apiTestServer{Server: srv, DB: db, BaseURL: srv.URL}
	ts.AdminID, ts.AdminToken = ts.seedAdmin(t)
	return ts
}

func (ts *apiTestServer) close() {
	ts.Server.Close()
	if sqlDB, err := ts.DB.DB(); err == nil {
		sqlDB.Close()
	}
}

// ── 种子数据 ────────────────────────────────────────────

func (ts *apiTestServer) seedAdmin(t *testing.T) (int64, string) {
	t.Helper()
	db := ts.DB
	pwd := "Admin@123"
	hashed, err := hash.HashPassword(pwd)
	require.NoError(t, err)

	db.Exec(`INSERT INTO roles (name, description, permissions, created_at, updated_at)
		VALUES ('系统管理员', '系统全局管理',
		'["user:manage","ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:review","knowledge:manage","dashboard:read","audit:read","system:config"]',
		NOW(), NOW())`)
	var roleID int64
	db.Raw("SELECT id FROM roles WHERE name = '系统管理员'").Scan(&roleID)
	require.NotZero(t, roleID)

	db.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ('apitest_admin', $1, 'Admin', '13800009999', 1, false, NOW(), NOW())`, hashed)
	var userID int64
	db.Raw("SELECT id FROM users WHERE username = 'apitest_admin'").Scan(&userID)
	require.NotZero(t, userID)
	db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "apitest_admin", "password": pwd}, "")
	body := parseBody(t, resp)
	require.Equal(t, float64(0), body["code"], "管理员登录失败: %v", body["message"])
	data := body["data"].(map[string]interface{})
	return userID, data["access_token"].(string)
}

func (ts *apiTestServer) seedKB(t *testing.T, name string) int64 {
	t.Helper()
	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases", map[string]interface{}{
		"name": name, "description": "test", "embedding_model": "bge-m3", "vector_dimension": 1024,
	})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建知识库失败")
	var id int64
	ts.DB.Raw("SELECT id FROM knowledge_bases WHERE name = $1", name).Scan(&id)
	return id
}

func (ts *apiTestServer) seedArticle(t *testing.T, kbID int64, title, content string) int64 {
	t.Helper()
	resp := ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/articles", kbID),
		map[string]interface{}{"title": title, "content": content})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建文章失败")
	var id int64
	ts.DB.Raw("SELECT id FROM knowledge_articles WHERE title = $1 ORDER BY id DESC LIMIT 1", title).Scan(&id)
	return id
}

func (ts *apiTestServer) seedRole(t *testing.T, name string, perms []string) int64 {
	t.Helper()
	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/roles", map[string]interface{}{
		"name": name, "description": "test", "permissions": perms,
	})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建角色失败")
	var id int64
	ts.DB.Raw("SELECT id FROM roles WHERE name = $1", name).Scan(&id)
	return id
}

func (ts *apiTestServer) seedTicket(t *testing.T, title, desc, phone string) int64 {
	t.Helper()
	resp := ts.doAuth(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": title, "description": desc, "urgency": 1, "contact_phone": phone,
	})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建申告失败")
	var id int64
	ts.DB.Raw("SELECT id FROM tickets WHERE title = $1 ORDER BY id DESC LIMIT 1", title).Scan(&id)
	return id
}

// ── HTTP 请求 ───────────────────────────────────────────

func (ts *apiTestServer) do(t *testing.T, method, path string, body interface{}, token string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.BaseURL+path, r)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func (ts *apiTestServer) doAuth(t *testing.T, method, path string, body interface{}) *http.Response {
	return ts.do(t, method, path, body, ts.AdminToken)
}

// ── SSE ─────────────────────────────────────────────────

func (ts *apiTestServer) doSSE(t *testing.T, path string, body interface{}) (*http.Response, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.BaseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.AdminToken)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	return resp, respBody
}

// ── 断言 ────────────────────────────────────────────────

func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func assertCode(t *testing.T, resp *http.Response, code float64) {
	t.Helper()
	body := parseBody(t, resp)
	c, ok := body["code"].(float64)
	if !ok || c != code {
		t.Fatalf("code=%v (期望 %v), message=%v", body["code"], code, body["message"])
	}
}

func assertOK(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body := parseBody(t, resp)
	c, ok := body["code"].(float64)
	if !ok || c != 0 {
		t.Fatalf("code=%v (期望 0), message=%v", body["code"], body["message"])
	}
	return body
}

// ── 数据库清理 ──────────────────────────────────────────

func cleanTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, tbl := range []string{
		"knowledge_chunks", "knowledge_articles", "knowledge_bases",
		"ticket_records", "tickets", "chat_messages", "chat_sessions",
		"messages", "audit_logs", "user_roles", "role_menus",
		"users", "roles", "menus", "llm_configs", "system_configs",
	} {
		db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", tbl))
	}
}
