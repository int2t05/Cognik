//go:build integration

// Package repository_test 验证 ConfigRepo 数据访问层。
package repository_test

import (
	"context"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupConfigRepoTestDB(t *testing.T) *gorm.DB {
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
	db.AutoMigrate(&model.SystemConfig{})
	db.Exec("DELETE FROM system_configs WHERE key LIKE 'test_%'")
	return db
}

func TestConfigRepo_Upsert_Insert(t *testing.T) {
	db := setupConfigRepoTestDB(t)
	repo := repository.NewConfigRepo(db)
	ctx := context.Background()

	err := repo.Upsert(ctx, "test_config_key", "测试配置说明", datatypes.JSON(`"test_value"`), 1)
	if err != nil {
		t.Fatalf("Upsert 插入失败: %v", err)
	}

	cfg, err := repo.GetByKey(ctx, "test_config_key")
	if err != nil {
		t.Fatalf("GetByKey 失败: %v", err)
	}
	if cfg.Description != "测试配置说明" {
		t.Errorf("期望 Description='测试配置说明', 实际 '%s'", cfg.Description)
	}
}

func TestConfigRepo_Upsert_Update(t *testing.T) {
	db := setupConfigRepoTestDB(t)
	repo := repository.NewConfigRepo(db)
	ctx := context.Background()

	repo.Upsert(ctx, "test_update_key", "原始说明", datatypes.JSON(`"old"`), 1)
	err := repo.Upsert(ctx, "test_update_key", "更新后说明", datatypes.JSON(`"new"`), 2)
	if err != nil {
		t.Fatalf("Upsert 更新失败: %v", err)
	}

	cfg, _ := repo.GetByKey(ctx, "test_update_key")
	if cfg.Description != "更新后说明" {
		t.Errorf("期望 Description='更新后说明', 实际 '%s'", cfg.Description)
	}
	if string(cfg.Value) != `"new"` {
		t.Errorf("期望 Value='\"new\"', 实际 '%s'", string(cfg.Value))
	}
}

func TestConfigRepo_GetByKey_NotFound(t *testing.T) {
	db := setupConfigRepoTestDB(t)
	repo := repository.NewConfigRepo(db)
	ctx := context.Background()

	_, err := repo.GetByKey(ctx, "nonexistent_key_xyz")
	if err == nil {
		t.Fatal("期望 ErrRecordNotFound")
	}
}

func TestConfigRepo_List(t *testing.T) {
	db := setupConfigRepoTestDB(t)
	repo := repository.NewConfigRepo(db)
	ctx := context.Background()

	repo.Upsert(ctx, "test_list_1", "", datatypes.JSON(`"a"`), 1)
	repo.Upsert(ctx, "test_list_2", "", datatypes.JSON(`"b"`), 1)

	configs, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(configs) < 2 {
		t.Errorf("期望 >=2 条, 实际 %d", len(configs))
	}
}

func TestConfigRepo_List_Empty(t *testing.T) {
	db := setupConfigRepoTestDB(t)
	repo := repository.NewConfigRepo(db)
	ctx := context.Background()

	db.Exec("DELETE FROM system_configs WHERE key LIKE 'test_%'")
	configs, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List 空表: %v", err)
	}
	if configs == nil {
		t.Error("空表应返回空切片, 而非 nil")
	}
}
