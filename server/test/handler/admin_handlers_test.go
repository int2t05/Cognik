//go:build integration

// Package handler_test 验证 Dashboard / Audit / Config Handler HTTP 接口。
//
// 测试覆盖后台管理的数据看板、审计日志、系统配置端点。
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	"opsmind/internal/middleware"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupAdminTestDB(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	// 确保必要的表存在
	db.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY, ticket_no VARCHAR(32), user_id BIGINT,
		title VARCHAR(255), description TEXT, urgency SMALLINT,
		status SMALLINT DEFAULT 1, source SMALLINT DEFAULT 1,
		created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT, kb_id BIGINT, question TEXT,
		answer TEXT, confidence DOUBLE PRECISION DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_articles (
		id BIGSERIAL PRIMARY KEY, kb_id BIGINT, question TEXT, answer TEXT,
		status SMALLINT DEFAULT 1,
		created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id BIGSERIAL PRIMARY KEY, operator_id BIGINT, action VARCHAR(64),
		target_type VARCHAR(32), target_id BIGINT, detail TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS system_configs (
		id BIGSERIAL PRIMARY KEY, key VARCHAR(128) UNIQUE, value TEXT,
		updated_by BIGINT, updated_at TIMESTAMPTZ DEFAULT NOW()
	)`)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})

	return r, db
}

// =============================================================================
// Dashboard Handler
// =============================================================================

func TestDashboardHandler_GetStats(t *testing.T) {
	r, db := setupAdminTestDB(t)

	// 清理并准备数据
	db.Exec("DELETE FROM tickets")
	db.Exec("DELETE FROM chat_sessions")
	db.Exec("DELETE FROM knowledge_articles")
	db.Exec("INSERT INTO tickets (id, ticket_no, user_id, title, status, created_at) VALUES (1, 'TK-001', 1, 'Test', 1, NOW())")
	db.Exec("INSERT INTO chat_sessions (id, user_id, kb_id, question, answer, confidence, created_at) VALUES (1, 1, 1, 'Q', 'A', 0.9, NOW())")
	db.Exec("INSERT INTO knowledge_articles (id, kb_id, question, answer, status, created_at) VALUES (1, 1, 'Q', 'A', 4, NOW())")

	dashboardSvc := service.NewDashboardService(repository.NewDashboardRepo(db))
	h := handler.NewDashboardHandler(dashboardSvc)

	r.GET("/stats", h.GetStats)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int            `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("期望 code=0, got %d", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("data 不应为 nil")
	}
}

func TestDashboardHandler_GetTrends(t *testing.T) {
	r, db := setupAdminTestDB(t)

	db.Exec("DELETE FROM tickets")
	db.Exec("INSERT INTO tickets (id, ticket_no, user_id, title, status, created_at) VALUES (1, 'TK-001', 1, 'Test', 1, NOW())")

	dashboardSvc := service.NewDashboardService(repository.NewDashboardRepo(db))
	h := handler.NewDashboardHandler(dashboardSvc)

	r.GET("/trends", h.GetTrends)

	req := httptest.NewRequest("GET", "/trends?start_date=2026-06-01&end_date=2026-06-12&granularity=day", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Audit Handler
// =============================================================================

func TestAuditHandler_List(t *testing.T) {
	r, db := setupAdminTestDB(t)

	db.Exec("DELETE FROM audit_logs")
	db.Exec("INSERT INTO audit_logs (id, operator_id, action, target_type, created_at) VALUES (1, 1, 'ticket:create', 'ticket', NOW())")

	auditSvc := service.NewAuditService(repository.NewAuditRepo(db))
	h := handler.NewAuditHandler(auditSvc)

	r.GET("/audit-logs", h.List)

	req := httptest.NewRequest("GET", "/audit-logs?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Config Handler
// =============================================================================

func TestConfigHandler_Get(t *testing.T) {
	r, db := setupAdminTestDB(t)

	db.Exec("DELETE FROM system_configs WHERE key = 'app_name'")
	db.Exec(`INSERT INTO system_configs (key, value, updated_by, updated_at) VALUES ('app_name', '"OpsMind"', 1, NOW()) ON CONFLICT (key) DO UPDATE SET value = '"OpsMind"', updated_at = NOW()`)

	configSvc := service.NewConfigService(repository.NewConfigRepo(db), repository.NewAuditRepo(db))
	h := handler.NewConfigHandler(configSvc)

	r.GET("/configs/:key", h.Get)

	req := httptest.NewRequest("GET", "/configs/app_name", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}
}

func TestConfigHandler_Update(t *testing.T) {
	r, db := setupAdminTestDB(t)

	db.Exec("DELETE FROM system_configs WHERE key = 'app_name'")
	db.Exec(`INSERT INTO system_configs (key, value, updated_by, updated_at) VALUES ('app_name', '"OpsMind"', 1, NOW()) ON CONFLICT (key) DO UPDATE SET value = '"OpsMind"', updated_at = NOW()`)

	configSvc := service.NewConfigService(repository.NewConfigRepo(db), repository.NewAuditRepo(db))
	h := handler.NewConfigHandler(configSvc)

	r.PUT("/configs/:key", h.Update)

	body, _ := json.Marshal(map[string]string{"value": "\"OpsMind v2\""})
	req := httptest.NewRequest("PUT", "/configs/app_name", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}
}
