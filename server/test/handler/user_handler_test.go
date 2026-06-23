//go:build integration

// Package handler_test 验证 UserHandler HTTP 接口。
package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var userHandlerDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	userHandlerDB = db
}

func setupUserHandler(t *testing.T) (*handler.UserHandler, *model.User) {
	t.Helper()
	repo := repository.NewUserRepo(userHandlerDB)
	auditRepo := repository.NewAuditRepo(userHandlerDB)
	svc := service.NewUserService(repo, auditRepo, userHandlerDB, nil)
	h := handler.NewUserHandler(svc)

	// 清理同用户名的旧数据
	userHandlerDB.Where("username = ?", "test_handleruser_1").Delete(&model.User{})
	// 同时清理可能残留的同手机号空记录
	userHandlerDB.Where("phone = ?", "").Delete(&model.User{})

	user := &model.User{
		Username:     "test_handleruser_1",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		RealName:     "测试用户",
		Phone:        "10000000001",
		Status:       1,
	}
	userHandlerDB.Create(user)

	return h, user
}

func setupUserRouter(h *handler.UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/api/v1/admin")
	admin.GET("/users", h.List)
	admin.GET("/users/:id", h.GetByID)
	admin.PATCH("/users/:id/freeze", h.Freeze)
	admin.PATCH("/users/:id/restore", h.Restore)
	return r
}

func TestUserHandler_GetByID_Success(t *testing.T) {
	h, user := setupUserHandler(t)
	r := setupUserRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/admin/users/"+fmt.Sprintf("%d", user.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Errorf("期望 code=0, got %v", resp["code"])
	}
}

func TestUserHandler_GetByID_NotFound(t *testing.T) {
	h, _ := setupUserHandler(t)
	r := setupUserRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/admin/users/999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望 404, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 10004 {
		t.Errorf("期望 code=10004, got %v", resp["code"])
	}
}

func TestUserHandler_List_Success(t *testing.T) {
	h, _ := setupUserHandler(t)
	r := setupUserRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Errorf("期望 code=0, got %v", resp["code"])
	}
}

func TestUserHandler_Freeze_Success(t *testing.T) {
	h, user := setupUserHandler(t)
	r := setupUserRouter(h)

	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+fmt.Sprintf("%d", user.ID)+"/freeze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

func TestUserHandler_Restore_Success(t *testing.T) {
	h, user := setupUserHandler(t)
	userHandlerDB.Model(user).Update("status", 2)
	r := setupUserRouter(h)

	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+fmt.Sprintf("%d", user.ID)+"/restore", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}
