//go:build integration

// Package handler_test 验证 RoleHandler HTTP 接口。
package handler_test

import (
	"bytes"
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
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var roleHandlerDB *gorm.DB

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
	roleHandlerDB = db
}

func setupRoleHandler(t *testing.T) *handler.RoleHandler {
	t.Helper()
	repo := repository.NewRoleRepo(roleHandlerDB)
	menuRepo := repository.NewMenuRepo(roleHandlerDB)
	auditRepo := repository.NewAuditRepo(roleHandlerDB)
	svc := service.NewRoleService(repo, menuRepo, service.NewAuditService(auditRepo), roleHandlerDB)
	return handler.NewRoleHandler(svc)
}

func seedHandlerRole(t *testing.T, name string) *model.Role {
	t.Helper()
	roleHandlerDB.Where("name = ?", name).Delete(&model.Role{})
	role := &model.Role{
		Name:        name,
		Description: "测试角色",
		Permissions: datatypes.JSON(`["ticket:read"]`),
	}
	roleHandlerDB.Create(role)
	return role
}

func setupRoleRouter(h *handler.RoleHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/api/v1/admin")
	admin.POST("/roles", h.Create)
	admin.GET("/roles", h.List)
	admin.GET("/roles/:id", h.GetByID)
	admin.PUT("/roles/:id", h.Update)
	admin.DELETE("/roles/:id", h.Delete)
	return r
}

func TestRoleHandler_Create_Success(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)
	roleHandlerDB.Where("name = ?", "test_handler_role_create").Delete(&model.Role{})

	body := `{"name":"test_handler_role_create","description":"测试","permissions":["ticket:read"]}`
	req := httptest.NewRequest("POST", "/api/v1/admin/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestRoleHandler_Create_MissingName(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)

	body := `{"description":"测试","permissions":["ticket:read"]}`
	req := httptest.NewRequest("POST", "/api/v1/admin/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望 400, got %d", w.Code)
	}
}

func TestRoleHandler_GetByID_Success(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)
	role := seedHandlerRole(t, "test_handler_role_getbyid")

	req := httptest.NewRequest("GET", "/api/v1/admin/roles/"+fmt.Sprintf("%d", role.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

func TestRoleHandler_GetByID_NotFound(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/admin/roles/999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望 404, got %d", w.Code)
	}
}

func TestRoleHandler_List_Success(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)
	seedHandlerRole(t, "test_handler_role_list")

	req := httptest.NewRequest("GET", "/api/v1/admin/roles?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

func TestRoleHandler_Update_Success(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)
	roleHandlerDB.Where("name = ?", "test_handler_role_updated").Delete(&model.Role{})
	role := seedHandlerRole(t, "test_handler_role_update")

	body := `{"name":"test_handler_role_updated","description":"已更新","permissions":["ticket:read","ticket:write"]}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/roles/"+fmt.Sprintf("%d", role.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

func TestRoleHandler_Delete_Success(t *testing.T) {
	h := setupRoleHandler(t)
	r := setupRoleRouter(h)
	role := seedHandlerRole(t, "test_handler_role_delete")

	req := httptest.NewRequest("DELETE", "/api/v1/admin/roles/"+fmt.Sprintf("%d", role.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}
