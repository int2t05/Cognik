//go:build integration

// Package service_test 验证 RoleService 业务逻辑。
package service_test

import (
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var roleSvcDB *gorm.DB

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
	roleSvcDB = db
}

func setupRoleService(t *testing.T) *service.RoleService {
	t.Helper()
	repo := repository.NewRoleRepo(roleSvcDB)
	menuRepo := repository.NewMenuRepo(roleSvcDB)
	auditRepo := repository.NewAuditRepo(roleSvcDB)
	return service.NewRoleService(repo, menuRepo, auditRepo, roleSvcDB)
}

func seedTestRole(t *testing.T, name string) *model.Role {
	t.Helper()
	roleSvcDB.Where("name = ?", name).Delete(&model.Role{})
	role := &model.Role{
		Name:        name,
		Description: "测试角色",
		Permissions: datatypes.JSON(`["ticket:read","ticket:write"]`),
	}
	roleSvcDB.Create(role)
	return role
}

func TestRoleService_Create_Success(t *testing.T) {
	svc := setupRoleService(t)
	roleSvcDB.Where("name = ?", "test_role_create").Delete(&model.Role{})

	err := svc.Create(bgCtx, "test_role_create", "测试角色", []string{"ticket:read", "knowledge:read"})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var count int64
	roleSvcDB.Model(&model.Role{}).Where("name = ?", "test_role_create").Count(&count)
	if count != 1 {
		t.Errorf("期望创建1条记录, got %d", count)
	}
}

func TestRoleService_Create_Duplicate(t *testing.T) {
	svc := setupRoleService(t)
	seedTestRole(t, "test_role_dup")

	err := svc.Create(bgCtx, "test_role_dup", "重复角色", []string{"ticket:read"})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if code := err.(service.AppError).Code; code != 10005 {
		t.Errorf("期望错误码 10005, got %d", code)
	}
}

func TestRoleService_GetByID_Success(t *testing.T) {
	svc := setupRoleService(t)
	role := seedTestRole(t, "test_role_getbyid")

	result, err := svc.GetByID(bgCtx, role.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Name != "test_role_getbyid" {
		t.Errorf("期望角色名 test_role_getbyid, got %s", result.Name)
	}
}

func TestRoleService_GetByID_NotFound(t *testing.T) {
	svc := setupRoleService(t)

	_, err := svc.GetByID(bgCtx, 999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if code := err.(service.AppError).Code; code != 10004 {
		t.Errorf("期望错误码 10004, got %d", code)
	}
}

func TestRoleService_List_Success(t *testing.T) {
	svc := setupRoleService(t)
	seedTestRole(t, "test_role_list")

	roles, total, err := svc.List(bgCtx, 1, 10, "")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total < 1 {
		t.Errorf("期望 total >= 1, got %d", total)
	}
	if len(roles) < 1 {
		t.Errorf("期望至少1条记录, got %d", len(roles))
	}
}

func TestRoleService_Update_Success(t *testing.T) {
	svc := setupRoleService(t)
	// 清理可能残留的目标名称，避免跨测试污染触发唯一约束冲突
	roleSvcDB.Where("name = ?", "test_role_updated").Delete(&model.Role{})
	role := seedTestRole(t, "test_role_update")

	err := svc.Update(bgCtx, role.ID, "test_role_updated", "更新后的角色", []string{"ticket:read", "ticket:write", "system:config"})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.Role
	roleSvcDB.First(&updated, role.ID)
	if updated.Name != "test_role_updated" {
		t.Errorf("期望名称 test_role_updated, got %s", updated.Name)
	}
}

func TestRoleService_Update_NotFound(t *testing.T) {
	svc := setupRoleService(t)

	err := svc.Update(bgCtx, 999999, "不存在", "不存在", []string{"ticket:read"})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

func TestRoleService_Delete_Success(t *testing.T) {
	svc := setupRoleService(t)
	role := seedTestRole(t, "test_role_delete")

	err := svc.Delete(bgCtx, role.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var count int64
	roleSvcDB.Model(&model.Role{}).Where("id = ?", role.ID).Count(&count)
	if count != 0 {
		t.Errorf("期望记录已删除, got %d", count)
	}
}

func TestRoleService_Delete_NotFound(t *testing.T) {
	svc := setupRoleService(t)

	err := svc.Delete(bgCtx, 999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}
