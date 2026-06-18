
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestUser_Fields 验证 User 模型字段定义
func TestUser_Fields(t *testing.T) {
	now := time.Now()
	u := model.User{
		Username:     "testuser",
		PasswordHash: "$2a$10$hash",
		RealName:     "测试用户",
		Phone:        "13800138000",
		Email:        "test@example.com",
		Status:       model.StatusActive,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	u.ID = 1

	if u.Username != "testuser" {
		t.Errorf("Username = %q, 期望 testuser", u.Username)
	}
	if u.PasswordHash != "$2a$10$hash" {
		t.Errorf("PasswordHash = %q, 期望 $2a$10$hash", u.PasswordHash)
	}
	if u.RealName != "测试用户" {
		t.Errorf("RealName = %q, 期望 测试用户", u.RealName)
	}
	if u.Phone != "13800138000" {
		t.Errorf("Phone = %q, 期望 13800138000", u.Phone)
	}
	if u.Email != "test@example.com" {
		t.Errorf("Email = %q, 期望 test@example.com", u.Email)
	}
	if u.Status != model.StatusActive {
		t.Errorf("Status = %d, 期望 %d", u.Status, model.StatusActive)
	}
	if !u.FirstLogin {
		t.Error("FirstLogin = false, 期望 true")
	}
	if u.ID != 1 {
		t.Errorf("ID = %d, 期望 1", u.ID)
	}
}

// TestRole_Fields 验证 Role 模型字段定义
func TestRole_Fields(t *testing.T) {
	now := time.Now()
	r := model.Role{
		Name:        "管理员",
		Description: "系统管理员",
		Permissions: datatypes.JSON([]byte(`["user:read","user:write"]`)),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.ID = 1

	if r.Name != "管理员" {
		t.Errorf("Name = %q, 期望 管理员", r.Name)
	}
	if r.Description != "系统管理员" {
		t.Errorf("Description = %q, 期望 系统管理员", r.Description)
	}
	if len(r.Permissions) == 0 {
		t.Error("Permissions 为空，期望非空 JSONB")
	}
}

// TestMenu_Fields 验证 Menu 模型字段定义
func TestMenu_Fields(t *testing.T) {
	m := model.Menu{
		Name:      "用户管理",
		Path:      "/admin/users",
		Icon:      "users",
		ParentID:  0,
		SortOrder: 1,
		Type:      "menu",
	}
	m.ID = 1

	if m.Name != "用户管理" {
		t.Errorf("Name = %q, 期望 用户管理", m.Name)
	}
	if m.Path != "/admin/users" {
		t.Errorf("Path = %q, 期望 /admin/users", m.Path)
	}
	if m.Type != "menu" {
		t.Errorf("Type = %q, 期望 menu", m.Type)
	}
	if m.ParentID != 0 {
		t.Errorf("ParentID = %d, 期望 0", m.ParentID)
	}
}

// TestUserRole_Fields 验证 UserRole 中间表
func TestUserRole_Fields(t *testing.T) {
	ur := model.UserRole{
		UserID: 1,
		RoleID: 2,
	}

	if ur.UserID != 1 {
		t.Errorf("UserID = %d, 期望 1", ur.UserID)
	}
	if ur.RoleID != 2 {
		t.Errorf("RoleID = %d, 期望 2", ur.RoleID)
	}
}

// TestRoleMenu_Fields 验证 RoleMenu 中间表
func TestRoleMenu_Fields(t *testing.T) {
	rm := model.RoleMenu{
		RoleID: 1,
		MenuID: 3,
	}

	if rm.RoleID != 1 {
		t.Errorf("RoleID = %d, 期望 1", rm.RoleID)
	}
	if rm.MenuID != 3 {
		t.Errorf("MenuID = %d, 期望 3", rm.MenuID)
	}
}
