//go:build integration

// Package integration_test 提供 API 集成测试的共享基础设施。
//
// 架构：httptest.NewServer 启动完整 Gin 路由 → net/http.Client 真实调用。
// 依赖链：DB → Repo → Service → Handler → Router（与 main.go 一致）。
// 每个测试独立调用 startAPITestServer，TRUNCATE 保证隔离。
//
// 三种预置用户类型覆盖全权限测试：
//   - Admin（系统管理员）：全部后台 + 门户权限
//   - Reporter（报障人）：仅门户端权限
//   - Operator（运维人员）：ticket:read/write + knowledge:read/write
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

	"opsmind/internal/cache"
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

// apiTestServer 封装测试服务器及其依赖。
//
// 三种预置 token 覆盖全权限测试：
//   - AdminToken：系统管理员，拥有全部权限
//   - ReporterToken：报障人，仅门户端
//   - OperatorToken：运维人员，ticket + knowledge 读写
type apiTestServer struct {
	Server        *httptest.Server
	DB            *gorm.DB
	BaseURL       string
	AdminToken    string
	AdminID       int64
	ReporterToken string
	ReporterID    int64
	OperatorToken string
	OperatorID    int64
	authSvc       *service.AuthService
}

// startAPITestServer 启动完整的 API 测试服务器。
//
// 初始化流程：数据库连接 → 清理 → 自动迁移 → 构建依赖链 → 注册路由 → 启动 HTTP 服务。
// 每个测试用例独立调用此函数，TRUNCATE 保证测试隔离。
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

	// Repository 层
	userRepo, roleRepo, menuRepo := repository.NewUserRepo(db), repository.NewRoleRepo(db), repository.NewMenuRepo(db)
	ticketRepo := repository.NewTicketRepo(db)
	knowledgeRepo := repository.NewKnowledgeRepo(db)
	chatRepo, messageRepo := repository.NewChatRepo(db), repository.NewMessageRepo(db)
	auditRepo, dashboardRepo := repository.NewAuditRepo(db), repository.NewDashboardRepo(db)
	configRepo, llmConfigRepo := repository.NewConfigRepo(db), repository.NewLlmConfigRepo(db)

	// 缓存
	userCache := cache.NewUserStatusCache(db, 30*time.Second)

	// Service 层
	authSvc := service.NewAuthService(userRepo, menuRepo, db, jwtCfg)
	userSvc := service.NewUserService(userRepo, auditRepo, db, userCache)
	roleSvc := service.NewRoleService(roleRepo, menuRepo, auditRepo, db)
	messageSvc := service.NewMessageService(messageRepo)
	ticketSvc := service.NewTicketService(ticketRepo, service.NewGormTxManager(db), messageSvc, nil) // knowledgeCandidate 在 knowledgeSvc 构造后注入
	dashboardSvc := service.NewDashboardService(dashboardRepo)
	configSvc := service.NewConfigService(configRepo, auditRepo)
	auditSvc := service.NewAuditService(auditRepo)

	llmConfigSvc, err := service.NewLLMConfigService(llmConfigRepo, db, auditRepo)
	require.NoError(t, err)

	knowledgeSvc := service.NewKnowledgeService(knowledgeRepo,
		service.WithUserNames(userRepo), service.WithAuditRepo(auditRepo))
	ticketSvc.SetKnowledgeCandidate(knowledgeSvc)

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
	}, userCache, handlers)

	srv := httptest.NewServer(r)
	ts := &apiTestServer{Server: srv, DB: db, BaseURL: srv.URL, authSvc: authSvc}

	// 预置三种用户（按角色权限从大到小创建，避免外键约束问题）
	ts.AdminID, ts.AdminToken = ts.seedAdmin(t)
	ts.OperatorID, ts.OperatorToken = ts.seedOperator(t)
	ts.ReporterID, ts.ReporterToken = ts.seedReporter(t)

	return ts
}

func (ts *apiTestServer) close() {
	ts.Server.Close()
	if sqlDB, err := ts.DB.DB(); err == nil {
		sqlDB.Close()
	}
}

// ── 种子用户 ────────────────────────────────────────────

// seedAdmin 创建系统管理员用户并返回 (userID, token)。
//
// 系统管理员拥有全部权限，是所有后台 API 测试的基础。
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

// seedReporter 创建报障人用户并返回 (userID, token)。
//
// 报障人仅拥有门户端权限，无任何后台管理权限。
func (ts *apiTestServer) seedReporter(t *testing.T) (int64, string) {
	t.Helper()
	db := ts.DB
	pwd := "Reporter@123"
	hashed, err := hash.HashPassword(pwd)
	require.NoError(t, err)

	db.Exec(`INSERT INTO roles (name, description, permissions, created_at, updated_at)
		VALUES ('报障人', '门户端用户', '[]', NOW(), NOW())`)
	var roleID int64
	db.Raw("SELECT id FROM roles WHERE name = '报障人'").Scan(&roleID)
	require.NotZero(t, roleID)

	db.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ('apitest_reporter', $1, 'Reporter', '13800008881', 1, false, NOW(), NOW())`, hashed)
	var userID int64
	db.Raw("SELECT id FROM users WHERE username = 'apitest_reporter'").Scan(&userID)
	require.NotZero(t, userID)
	db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	return userID, ts.loginAs(t, "apitest_reporter", pwd)
}

// seedOperator 创建运维人员用户并返回 (userID, token)。
//
// 运维人员拥有申告处理 (ticket:read/write) 和知识读写 (knowledge:read/write/create) 权限。
func (ts *apiTestServer) seedOperator(t *testing.T) (int64, string) {
	t.Helper()
	db := ts.DB
	pwd := "Operator@123"
	hashed, err := hash.HashPassword(pwd)
	require.NoError(t, err)

	db.Exec(`INSERT INTO roles (name, description, permissions, created_at, updated_at)
		VALUES ('运维人员', '处理申告和回访',
		'["ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:review"]',
		NOW(), NOW())`)
	var roleID int64
	db.Raw("SELECT id FROM roles WHERE name = '运维人员'").Scan(&roleID)
	require.NotZero(t, roleID)

	db.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ('apitest_operator', $1, 'Operator', '13800008882', 1, false, NOW(), NOW())`, hashed)
	var userID int64
	db.Raw("SELECT id FROM users WHERE username = 'apitest_operator'").Scan(&userID)
	require.NotZero(t, userID)
	db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	return userID, ts.loginAs(t, "apitest_operator", pwd)
}

// loginAs 登录指定用户并返回 access_token。
func (ts *apiTestServer) loginAs(t *testing.T, username, password string) string {
	t.Helper()
	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username, "password": password,
	}, "")
	body := parseBody(t, resp)
	if body["code"] != float64(0) {
		t.Fatalf("%s 登录失败: %v", username, body["message"])
	}
	return body["data"].(map[string]interface{})["access_token"].(string)
}

// ── 种子业务数据 ────────────────────────────────────────

// seedKB 创建知识库并返回 kbID。
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

// seedArticle 在指定知识库中创建文章并返回 articleID。
func (ts *apiTestServer) seedArticle(t *testing.T, kbID int64, title, content string) int64 {
	t.Helper()
	resp := ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/articles", kbID),
		map[string]interface{}{"title": title, "content": content})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建文章失败")
	var id int64
	ts.DB.Raw("SELECT id FROM knowledge_articles WHERE title = $1 ORDER BY id DESC LIMIT 1", title).Scan(&id)
	return id
}

// seedRole 创建角色并返回 roleID。
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

// seedTicket 创建申告工单并返回 ticketID。
func (ts *apiTestServer) seedTicket(t *testing.T, title, desc, phone string) int64 {
	t.Helper()
	resp := ts.doReporter(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": title, "description": desc, "urgency": 1, "contact_phone": phone,
	})
	require.Equal(t, float64(0), parseBody(t, resp)["code"], "创建申告失败")
	var id int64
	ts.DB.Raw("SELECT id FROM tickets WHERE title = $1 ORDER BY id DESC LIMIT 1", title).Scan(&id)
	return id
}

// ── HTTP 请求 ───────────────────────────────────────────

// do 发送 HTTP 请求并返回响应。
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

// doAuth 使用管理员 token 发送请求。
func (ts *apiTestServer) doAuth(t *testing.T, method, path string, body interface{}) *http.Response {
	return ts.do(t, method, path, body, ts.AdminToken)
}

// doReporter 使用报障人 token 发送请求。
func (ts *apiTestServer) doReporter(t *testing.T, method, path string, body interface{}) *http.Response {
	return ts.do(t, method, path, body, ts.ReporterToken)
}

// doOperator 使用运维人员 token 发送请求。
func (ts *apiTestServer) doOperator(t *testing.T, method, path string, body interface{}) *http.Response {
	return ts.do(t, method, path, body, ts.OperatorToken)
}

// ── SSE ─────────────────────────────────────────────────

// doSSE 发送 SSE 流式请求并返回响应体和原始字节。
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

// ── 通用断言 ────────────────────────────────────────────

// parseBody 读取响应体并解析为 JSON map。
func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

// assertCode 断言响应的 code 等于期望值。
func assertCode(t *testing.T, resp *http.Response, code float64) {
	t.Helper()
	body := parseBody(t, resp)
	c, ok := body["code"].(float64)
	if !ok || c != code {
		t.Fatalf("code=%v (期望 %v), message=%v", body["code"], code, body["message"])
	}
}

// assertOK 断言响应成功 (code=0) 并返回 body。
func assertOK(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body := parseBody(t, resp)
	c, ok := body["code"].(float64)
	if !ok || c != 0 {
		t.Fatalf("code=%v (期望 0), message=%v", body["code"], body["message"])
	}
	return body
}

// assertHTTPStatus 断言 HTTP 状态码。
func assertHTTPStatus(t *testing.T, resp *http.Response, status int) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("HTTP status=%d (期望 %d)", resp.StatusCode, status)
	}
}

// assertUnauthorized 断言 code=10001（未认证）。
func assertUnauthorized(t *testing.T, resp *http.Response) {
	assertCode(t, resp, 10001)
}

// assertForbidden 断言 code=10002（无权限）。
func assertForbidden(t *testing.T, resp *http.Response) {
	assertCode(t, resp, 10002)
}

// assertNotFound 断言 code=10004（资源不存在）。
func assertNotFound(t *testing.T, resp *http.Response) {
	assertCode(t, resp, 10004)
}

// assertConflict 断言 code=10005（资源冲突）。
func assertConflict(t *testing.T, resp *http.Response) {
	assertCode(t, resp, 10005)
}

// assertBadRequest 断言 HTTP 400 和 code=10003。
func assertBadRequest(t *testing.T, resp *http.Response) {
	assertHTTPStatus(t, resp, http.StatusBadRequest)
	assertCode(t, resp, 10003)
}

// assertServerError 断言 code=99999（服务器内部错误）。
func assertServerError(t *testing.T, resp *http.Response) {
	assertCode(t, resp, 99999)
}

// ── 数据库清理 ──────────────────────────────────────────

// cleanTables 清空所有业务表，保证测试隔离。
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
