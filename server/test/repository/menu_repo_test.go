//go:build integration

// Package repository_test 验证 MenuRepo 数据访问层。
package repository_test

import (
	"context"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/repository"

	"gorm.io/gorm"
)

func setupMenuTestDB(t *testing.T) *gorm.DB {
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

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS menus (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL, path VARCHAR(255),
		icon VARCHAR(64), parent_id BIGINT DEFAULT 0, sort_order INT DEFAULT 0,
		type VARCHAR(32) NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS role_menus (
		role_id BIGINT NOT NULL, menu_id BIGINT NOT NULL, PRIMARY KEY (role_id, menu_id)
	)`)
	db.Exec("DELETE FROM role_menus")
	db.Exec("DELETE FROM menus WHERE name LIKE 'test_%'")
	return db
}

func TestMenuRepo_ListMenus(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type, sort_order) VALUES
		('test_menu_a', '/test/a', 'icon-a', 'menu', 2),
		('test_menu_b', '/test/b', 'icon-b', 'menu', 1)`)

	menus, err := repo.ListMenus(ctx)
	if err != nil {
		t.Fatalf("ListMenus 失败: %v", err)
	}
	if len(menus) < 2 {
		t.Errorf("期望 >=2 条, 实际 %d", len(menus))
	}
	// 验证按 sort_order 排序
	if len(menus) >= 2 && menus[0].SortOrder > menus[1].SortOrder {
		t.Error("期望按 sort_order 升序")
	}
}

func TestMenuRepo_GetRoleMenus(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type) VALUES
		('test_role_menu1', '/test/rm1', 'icon1', 'menu'),
		('test_role_menu2', '/test/rm2', 'icon2', 'menu')`)
	db.Exec(`INSERT INTO role_menus (role_id, menu_id)
		SELECT 1, id FROM menus WHERE name LIKE 'test_role_menu%'`)

	menus, err := repo.GetRoleMenus(ctx, 1)
	if err != nil {
		t.Fatalf("GetRoleMenus 失败: %v", err)
	}
	if len(menus) != 2 {
		t.Errorf("期望 2 条, 实际 %d", len(menus))
	}
}

func TestMenuRepo_BatchGetRoleMenus(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type) VALUES
		('test_batch_menu1', '/test/bm1', 'icon1', 'menu'),
		('test_batch_menu2', '/test/bm2', 'icon2', 'menu'),
		('test_batch_menu3', '/test/bm3', 'icon3', 'menu')`)
	db.Exec(`INSERT INTO role_menus (role_id, menu_id)
		SELECT 10, id FROM menus WHERE name IN ('test_batch_menu1', 'test_batch_menu2')`)
	db.Exec(`INSERT INTO role_menus (role_id, menu_id)
		SELECT 11, id FROM menus WHERE name = 'test_batch_menu3'`)

	menus, err := repo.BatchGetRoleMenus(ctx, []int64{10, 11})
	if err != nil {
		t.Fatalf("BatchGetRoleMenus 失败: %v", err)
	}
	if len(menus) != 3 {
		t.Errorf("期望 3 条（去重）, 实际 %d", len(menus))
	}
}

func TestMenuRepo_BatchGetRoleMenus_Empty(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	menus, err := repo.BatchGetRoleMenus(ctx, []int64{})
	if err != nil {
		t.Fatalf("BatchGetRoleMenus 空输入: %v", err)
	}
	if menus != nil {
		t.Error("空输入应返回 nil")
	}
}

func TestMenuRepo_ValidateMenuIDs(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type) VALUES ('test_validate_menu', '/test/vm', 'icon', 'menu')`)
	var id int64
	db.Raw("SELECT id FROM menus WHERE name = 'test_validate_menu'").Scan(&id)

	missing, err := repo.ValidateMenuIDs(ctx, []int64{id, 99999})
	if err != nil {
		t.Fatalf("ValidateMenuIDs 失败: %v", err)
	}
	if len(missing) != 1 || missing[0] != 99999 {
		t.Errorf("期望 missing=[99999], 实际 %v", missing)
	}
}

func TestMenuRepo_UpdateRoleMenus(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type) VALUES
		('test_updatemenu1', '/test/um1', 'icon1', 'menu'),
		('test_updatemenu2', '/test/um2', 'icon2', 'menu')`)
	db.Exec(`INSERT INTO role_menus (role_id, menu_id)
		SELECT 100, id FROM menus WHERE name = 'test_updatemenu1'`)

	// 更新为两个菜单
	var ids []int64
	db.Raw("SELECT id FROM menus WHERE name LIKE 'test_updatemenu%' ORDER BY name").Scan(&ids)
	err := repo.UpdateRoleMenus(ctx, 100, ids)
	if err != nil {
		t.Fatalf("UpdateRoleMenus 失败: %v", err)
	}

	menus, _ := repo.GetRoleMenus(ctx, 100)
	if len(menus) != 2 {
		t.Errorf("期望 2 条, 实际 %d", len(menus))
	}
}

func TestMenuRepo_UpdateRoleMenus_Clear(t *testing.T) {
	db := setupMenuTestDB(t)
	repo := repository.NewMenuRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO menus (name, path, icon, type) VALUES ('test_clearmenu', '/test/cm', 'icon', 'menu')`)
	db.Exec(`INSERT INTO role_menus (role_id, menu_id)
		SELECT 200, id FROM menus WHERE name = 'test_clearmenu'`)

	// 清空菜单
	err := repo.UpdateRoleMenus(ctx, 200, nil)
	if err != nil {
		t.Fatalf("UpdateRoleMenus 清空失败: %v", err)
	}
	menus, _ := repo.GetRoleMenus(ctx, 200)
	if len(menus) != 0 {
		t.Errorf("清空后期望 0 条, 实际 %d", len(menus))
	}
}
