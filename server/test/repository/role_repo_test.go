//go:build integration

// Package repository_test 验证 RoleRepo 数据访问层。
//
// 测试覆盖角色 CRUD + 唯一性校验 + 删除防御。
package repository_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupRoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	port, _ := strconv.Atoi(getEnv("TEST_DB_PORT", "5432"))
	db, err := database.Init(config.DatabaseConfig{
		Host: getEnv("TEST_DB_HOST", "localhost"), Port: port,
		User: getEnv("TEST_DB_USER", "opsmind"), Password: getEnv("TEST_DB_PASSWORD", "opsmind_dev"),
		DBName: getEnv("TEST_DB_NAME", "opsmind_test"), SSLMode: getEnv("TEST_DB_SSLMODE", "disable"),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS roles (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL UNIQUE,
		description VARCHAR(255), permissions JSONB, is_system BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 清空角色和关联表，避免其他测试残留数据干扰
	db.Exec("DELETE FROM role_menus")
	db.Exec("DELETE FROM roles")
	return db
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func TestRoleRepo_Create(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	role := &model.Role{
		Name: "test_create_role", Description: "测试创建",
		Permissions: datatypes.JSON(`["ticket:read"]`),
	}
	err := repo.Create(ctx, role)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if role.ID == 0 {
		t.Error("期望 ID 被填充")
	}
}

func TestRoleRepo_GetByID(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, permissions, created_at, updated_at) VALUES ('test_get_role', '测试', '["ticket:read"]', NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM roles WHERE name = 'test_get_role'").Scan(&id)

	role, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID 失败: %v", err)
	}
	if role.Name != "test_get_role" {
		t.Errorf("期望 test_get_role, 实际 %s", role.Name)
	}
}

func TestRoleRepo_GetByID_NotFound(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	if err == nil {
		t.Fatal("期望 ErrRecordNotFound")
	}
}

func TestRoleRepo_ExistsByName(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES ('test_exists_check', '测试', NOW(), NOW())`)

	exists, err := repo.ExistsByName(ctx, "test_exists_check", 0)
	if err != nil {
		t.Fatalf("ExistsByName 失败: %v", err)
	}
	if !exists {
		t.Error("期望 exists=true")
	}

	notExists, err := repo.ExistsByName(ctx, "nonexistent_name_xyz", 0)
	if err != nil {
		t.Fatalf("ExistsByName 失败: %v", err)
	}
	if notExists {
		t.Error("期望 exists=false")
	}
}

func TestRoleRepo_ExistsByName_ExcludeSelf(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES ('test_exclude_self', '测试', NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM roles WHERE name = 'test_exclude_self'").Scan(&id)

	// 排除自身后不应冲突
	exists, err := repo.ExistsByName(ctx, "test_exclude_self", id)
	if err != nil {
		t.Fatalf("ExistsByName 失败: %v", err)
	}
	if exists {
		t.Error("排除自身后期望不存在冲突")
	}
}

func TestRoleRepo_List(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES
		('test_list_role1', '列表测试1', NOW(), NOW()), ('test_list_role2', '列表测试2', NOW(), NOW())`)

	roles, total, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total < 2 {
		t.Errorf("期望 total>=2, 实际 %d", total)
	}
	_ = roles
}

func TestRoleRepo_List_Keyword(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES
		('test_keyword_abc', 'ABC描述', NOW(), NOW()), ('test_keyword_xyz', 'XYZ描述', NOW(), NOW())`)

	roles, total, err := repo.List(ctx, 1, 10, "abc")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total == 0 {
		t.Error("关键词搜索应返回结果")
	}
	for _, r := range roles {
		if r.Name != "test_keyword_abc" {
			t.Errorf("关键词过滤失败: 意外角色 %s", r.Name)
		}
	}
}

func TestRoleRepo_Update(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES ('test_update_role', '更新前', NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM roles WHERE name = 'test_update_role'").Scan(&id)

	role := &model.Role{ID: id, Name: "test_update_role", Description: "更新后", Permissions: datatypes.JSON(`["ticket:write"]`)}
	if err := repo.Update(ctx, role); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}

	updated, _ := repo.GetByID(ctx, id)
	if updated.Description != "更新后" {
		t.Errorf("期望 Description='更新后', 实际 '%s'", updated.Description)
	}
}

func TestRoleRepo_Delete(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, created_at, updated_at) VALUES ('test_delete_role', '待删除', NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM roles WHERE name = 'test_delete_role'").Scan(&id)

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	_, err := repo.GetByID(ctx, id)
	if err == nil {
		t.Error("删除后查询应返回 ErrRecordNotFound")
	}
}

func TestRoleRepo_Delete_ZeroID(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	err := repo.Delete(ctx, 0)
	if err == nil {
		t.Fatal("删除 id=0 应返回错误")
	}
}

func TestRoleRepo_IsBuiltinRole(t *testing.T) {
	db := setupRoleTestDB(t)
	repo := repository.NewRoleRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO roles (name, description, is_system, created_at, updated_at) VALUES ('test_system_role', '系统角色', true, NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM roles WHERE name = 'test_system_role'").Scan(&id)

	isSystem, err := repo.IsBuiltinRole(ctx, id)
	if err != nil {
		t.Fatalf("IsBuiltinRole 失败: %v", err)
	}
	if !isSystem {
		t.Error("系统角色应返回 true")
	}
}
