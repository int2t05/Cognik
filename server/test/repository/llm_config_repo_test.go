//go:build integration

// Package repository_test 验证 LlmConfigRepo 数据访问层。
package repository_test

import (
	"context"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/gorm"
)

func setupLLMConfigRepoTestDB(t *testing.T) *gorm.DB {
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
	db.AutoMigrate(&model.LlmConfig{}, &model.KnowledgeBase{})
	db.Exec("DELETE FROM knowledge_bases WHERE name LIKE 'test_llmcfg_%'")
	db.Exec("DELETE FROM llm_configs WHERE name LIKE 'test_llmcfg_%'")
	return db
}

func TestLlmConfigRepo_Create(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	cfg := &model.LlmConfig{
		Name: "test_llmcfg_create", LLMBaseURL: "http://localhost:8080/v1",
		LLMModel: "test-model", EmbeddingModel: "test-embed",
	}
	err := repo.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if cfg.ID == 0 {
		t.Error("期望 ID 被填充")
	}
}

func TestLlmConfigRepo_FindByID(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, created_at, updated_at)
		VALUES ('test_llmcfg_find', 'http://localhost:8080/v1', 'm', 'e', 8192, 1024, NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM llm_configs WHERE name = 'test_llmcfg_find'").Scan(&id)

	cfg, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID 失败: %v", err)
	}
	if cfg.Name != "test_llmcfg_find" {
		t.Errorf("期望 test_llmcfg_find, 实际 %s", cfg.Name)
	}
}

func TestLlmConfigRepo_FindByID_NotFound(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, 99999)
	if err == nil {
		t.Fatal("期望 ErrRecordNotFound")
	}
}

func TestLlmConfigRepo_FindDefault(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, is_default, created_at, updated_at)
		VALUES ('test_llmcfg_def', 'http://localhost:8080/v1', 'm', 'e', 8192, 1024, true, NOW(), NOW())`)

	cfg, err := repo.FindDefault(ctx)
	if err != nil {
		t.Fatalf("FindDefault 失败: %v", err)
	}
	if !cfg.IsDefault {
		t.Error("期望 IsDefault=true")
	}
}

func TestLlmConfigRepo_List(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, created_at, updated_at) VALUES
		('test_llmcfg_list1', 1, 'http://a:8080/v1', 'm1', 'e1', 8192, 1024, NOW(), NOW()),
		('test_llmcfg_list2', 1, 'http://b:8080/v1', 'm2', 'e2', 8192, 1024, NOW(), NOW())`)

	configs, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(configs) < 2 {
		t.Errorf("期望 >=2 条, 实际 %d", len(configs))
	}
}

func TestLlmConfigRepo_Update(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, created_at, updated_at)
		VALUES ('test_llmcfg_upd', 'http://old:8080/v1', 'old-m', 'old-e', 8192, 1024, NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM llm_configs WHERE name = 'test_llmcfg_upd'").Scan(&id)

	cfg := &model.LlmConfig{ID: id, Name: "test_llmcfg_upd", LLMBaseURL: "http://new:8080/v1",
		LLMModel: "new-m", EmbeddingModel: "new-e"}
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}

	updated, _ := repo.FindByID(ctx, id)
	if updated.LLMModel != "new-m" {
		t.Errorf("期望 new-m, 实际 %s", updated.LLMModel)
	}
}

func TestLlmConfigRepo_Delete(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, created_at, updated_at)
		VALUES ('test_llmcfg_del', 'http://x:8080/v1', 'm', 'e', 8192, 1024, NOW(), NOW())`)
	var id int64
	db.Raw("SELECT id FROM llm_configs WHERE name = 'test_llmcfg_del'").Scan(&id)

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	_, err := repo.FindByID(ctx, id)
	if err == nil {
		t.Error("删除后查询应返回 ErrRecordNotFound")
	}
}

func TestLlmConfigRepo_ClearDefault(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, is_default, created_at, updated_at) VALUES
		('test_llmcfg_clr1', 1, 'http://a:8080/v1', 'm1', 'e1', 8192, 1024, true, NOW(), NOW()),
		('test_llmcfg_clr2', 1, 'http://b:8080/v1', 'm2', 'e2', 8192, 1024, true, NOW(), NOW())`)

	if err := repo.ClearDefault(ctx); err != nil {
		t.Fatalf("ClearDefault 失败: %v", err)
	}

	// 验证所有 is_default 均为 false
	var count int64
	db.Raw("SELECT COUNT(*) FROM llm_configs WHERE name LIKE 'test_llmcfg_clr%' AND is_default = true").Scan(&count)
	if count != 0 {
		t.Errorf("ClearDefault 后应无默认配置, 实际 %d 条", count)
	}
}

func TestLlmConfigRepo_CountReferencingKBs(t *testing.T) {
	db := setupLLMConfigRepoTestDB(t)
	repo := repository.NewLlmConfigRepo(db)
	ctx := context.Background()

	db.Exec(`INSERT INTO llm_configs (id, name, llm_base_url, llm_model, embedding_model, max_tokens, vector_dimension, created_at, updated_at)
		VALUES (100, 'test_llmcfg_ref', 'http://x:8080/v1', 'm', 'e', 8192, 1024, NOW(), NOW())`)
	db.Exec(`INSERT INTO knowledge_bases (name, llm_config_id, embedding_model, vector_dimension, created_by, created_at, updated_at)
		VALUES ('test_llmcfg_kb1', 100, 'bge-m3', 1024, 1, NOW(), NOW()),
		       ('test_llmcfg_kb2', 100, 'bge-m3', 1024, 1, NOW(), NOW())`)

	count, err := repo.CountReferencingKBs(ctx, 100)
	if err != nil {
		t.Fatalf("CountReferencingKBs 失败: %v", err)
	}
	if count != 2 {
		t.Errorf("期望 2 个引用, 实际 %d", count)
	}
}
