//go:build integration

// Package handler_test 验证 MessageHandler HTTP 接口。
//
// 测试覆盖门户端站内消息的 ListMessages / MarkAsRead / CountUnread 端点。
package handler_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	"opsmind/internal/middleware"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupMessageHandler(t *testing.T) (*gin.Engine, *gorm.DB) {
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

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 确保表存在
	db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL,
		type VARCHAR(32) NOT NULL, related_type VARCHAR(32), related_id BIGINT,
		title VARCHAR(255) NOT NULL, content TEXT,
		is_read BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理并准备测试数据
	db.Exec("DELETE FROM messages")
	db.Exec(`INSERT INTO messages (id, user_id, type, title, content, is_read, created_at)
		VALUES (1, 1, 'ticket_status', '申告处理中', '申告已被接单', false, NOW())`)
	db.Exec(`INSERT INTO messages (id, user_id, type, title, content, is_read, created_at)
		VALUES (2, 1, 'ticket_resolved', '申告已解决', '问题已处理', true, NOW())`)
	db.Exec(`INSERT INTO messages (id, user_id, type, title, content, is_read, created_at)
		VALUES (3, 2, 'ticket_supplement', '请补充信息', '需要更多信息', false, NOW())`)

	svc := service.NewMessageService(repository.NewMessageRepo(db))
	h := handler.NewMessageHandler(svc)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})

	portal := r.Group("/api/v1/portal")
	{
		portal.GET("/messages", h.ListMessages)
		portal.PUT("/messages/:id/read", h.MarkAsRead)
		portal.GET("/messages/unread-count", h.CountUnread)
	}

	return r, db
}

// TestMessageHandler_ListMessages 验证消息列表。
func TestMessageHandler_ListMessages(t *testing.T) {
	r, _ := setupMessageHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/portal/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int              `json:"code"`
		Data []model.Message  `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("期望 code=0, got %d", resp.Code)
	}
	// user_id=1 应有 2 条消息
	if len(resp.Data) != 2 {
		t.Errorf("期望 2 条消息, 实际 %d", len(resp.Data))
	}
}

// TestMessageHandler_MarkAsRead 验证标记已读。
func TestMessageHandler_MarkAsRead(t *testing.T) {
	r, db := setupMessageHandler(t)

	req := httptest.NewRequest("PUT", "/api/v1/portal/messages/1/read", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	var msg model.Message
	db.First(&msg, 1)
	if !msg.IsRead {
		t.Error("消息应标记为已读")
	}
}

// TestMessageHandler_CountUnread 验证未读计数。
func TestMessageHandler_CountUnread(t *testing.T) {
	r, _ := setupMessageHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/portal/messages/unread-count", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d", w.Code)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("期望 code=0, got %d", resp.Code)
	}
	// user_id=1 应有 1 条未读（id=1 is_read=false, id=2 is_read=true）
	if resp.Data.Count != 1 {
		t.Errorf("期望 1 条未读, 实际 %d", resp.Data.Count)
	}
}
