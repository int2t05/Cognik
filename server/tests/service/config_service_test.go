//go:build integration

// Package service_test 验证系统配置 Repository 和 Service 的功能。
//
// 需要运行中的 PostgreSQL 实例。运行方式：
//
//	go test ./tests/service/... -tags=integration -v -run "TestConfigRepo|TestConfigService"
package service_test

import (
	"encoding/json"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// jsonEqual 语义比较两个 JSON 字符串是否等价。
func jsonEqual(t *testing.T, got datatypes.JSON, expected string) {
	t.Helper()
	var a, b interface{}
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("解析实际 JSON 失败: %v, 原始值: %s", err, string(got))
	}
	if err := json.Unmarshal([]byte(expected), &b); err != nil {
		t.Fatalf("解析期望 JSON 失败: %v, 原始值: %s", err, expected)
	}
	gotBytes, _ := json.Marshal(a)
	expectedBytes, _ := json.Marshal(b)
	if string(gotBytes) != string(expectedBytes) {
		t.Errorf("Value = %s, 期望 %s", string(got), expected)
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbCfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("自动迁移 SystemConfig 失败: %v", err)
	}
	db.Exec("DELETE FROM system_configs")
	return db
}

// =============================================================================
// ConfigRepo 测试（Task 09）
// =============================================================================

func TestConfigRepo_GetByKey_Existing(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	value := datatypes.JSON(`{"threshold":0.6}`)
	cfg := &model.SystemConfig{Key: "ai.confidence_threshold", Value: value, Description: "AI 置信度阈值", UpdatedBy: 1}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
	result, err := repo.GetByKey(bgCtx, "ai.confidence_threshold")
	if err != nil {
		t.Fatalf("GetByKey 失败: %v", err)
	}
	if result.Key != "ai.confidence_threshold" {
		t.Errorf("Key = %q, 期望 ai.confidence_threshold", result.Key)
	}
}

func TestConfigRepo_GetByKey_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	_, err := repo.GetByKey(bgCtx, "nonexistent.key")
	if err == nil {
		t.Fatal("期望返回错误, 实际为 nil")
	}
}

func TestConfigRepo_Upsert_UpdateExisting(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	db.Create(&model.SystemConfig{Key: "ai.confidence_threshold", Value: datatypes.JSON(`{"threshold":0.6}`), UpdatedBy: 1})
	err := repo.Upsert(bgCtx, "ai.confidence_threshold", "AI 置信度阈值", datatypes.JSON(`{"threshold":0.8}`), 2)
	if err != nil {
		t.Fatalf("Upsert 更新失败: %v", err)
	}
	result, err := repo.GetByKey(bgCtx, "ai.confidence_threshold")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	jsonEqual(t, result.Value, `{"threshold":0.8}`)
}

func TestConfigRepo_Upsert_InsertNew(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	err := repo.Upsert(bgCtx, "system.max_retries", "系统最大重试次数", datatypes.JSON(`{"max_retries":3}`), 1)
	if err != nil {
		t.Fatalf("Upsert 插入失败: %v", err)
	}
	result, err := repo.GetByKey(bgCtx, "system.max_retries")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if result.Key != "system.max_retries" {
		t.Errorf("Key = %q, 期望 system.max_retries", result.Key)
	}
}

func TestConfigRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	configs := []model.SystemConfig{
		{Key: "ai.confidence_threshold", Value: datatypes.JSON(`{"threshold":0.6}`), UpdatedBy: 1},
		{Key: "ai.default_top_k", Value: datatypes.JSON(`{"top_k":5}`), UpdatedBy: 1},
		{Key: "system.max_retries", Value: datatypes.JSON(`{"max_retries":3}`), UpdatedBy: 2},
	}
	for i := range configs {
		db.Create(&configs[i])
	}
	result, err := repo.List(bgCtx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("List 返回 %d 条, 期望 3", len(result))
	}
}

func TestConfigRepo_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	result, err := repo.List(bgCtx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if result == nil {
		t.Fatal("List 返回 nil, 期望空切片")
	}
}

// =============================================================================
// ConfigService 测试（Task 34）
// =============================================================================

func setupConfigService(t *testing.T) *service.ConfigService {
	t.Helper()
	db := setupTestDB(t)
	repo := repository.NewConfigRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	return service.NewConfigService(repo, auditRepo)
}

// TestConfigService_GetConfig_Existing 验证获取已有配置返回正确的值。
func TestConfigService_GetConfig_Existing(t *testing.T) {
	svc := setupConfigService(t)
	svc.UpdateConfig(bgCtx, "test.key1", "value1", 1)

	val, err := svc.GetConfig(bgCtx, "test.key1")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if val != "value1" {
		t.Errorf("期望 'value1', got '%v'", val)
	}
}

// TestConfigService_GetConfig_NotFound 验证查询不存在的 key 返回明确错误。
func TestConfigService_GetConfig_NotFound(t *testing.T) {
	svc := setupConfigService(t)
	_, err := svc.GetConfig(bgCtx, "nonexistent.key")
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestConfigService_GetConfig_JSONObject 验证获取 JSON 对象类型的配置。
func TestConfigService_GetConfig_JSONObject(t *testing.T) {
	svc := setupConfigService(t)
	svc.UpdateConfig(bgCtx, "test.json_key", map[string]interface{}{"threshold": 0.6, "top_k": 5.0}, 1)

	val, err := svc.GetConfig(bgCtx, "test.json_key")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	valMap, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("期望 map[string]interface{}, got %T", val)
	}
	if valMap["threshold"] != 0.6 {
		t.Errorf("期望 threshold=0.6, got %v", valMap["threshold"])
	}
	if valMap["top_k"] != 5.0 {
		t.Errorf("期望 top_k=5.0, got %v", valMap["top_k"])
	}
}

// TestConfigService_UpdateConfig_Create 验证创建新配置。
func TestConfigService_UpdateConfig_Create(t *testing.T) {
	svc := setupConfigService(t)
	err := svc.UpdateConfig(bgCtx, "ai.top_k", 10.0, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	val, err := svc.GetConfig(bgCtx, "ai.top_k")
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if val != 10.0 {
		t.Errorf("期望 10.0, got %v", val)
	}
}

// TestConfigService_UpdateConfig_Update 验证更新已有配置。
func TestConfigService_UpdateConfig_Update(t *testing.T) {
	svc := setupConfigService(t)
	svc.UpdateConfig(bgCtx, "ai.threshold", 0.7, 1)
	err := svc.UpdateConfig(bgCtx, "ai.threshold", 0.85, 2)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	val, err := svc.GetConfig(bgCtx, "ai.threshold")
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if val != 0.85 {
		t.Errorf("期望 0.85, got %v", val)
	}
}

// TestConfigService_UpdateConfig_StringValue 验证字符串类型配置。
func TestConfigService_UpdateConfig_StringValue(t *testing.T) {
	svc := setupConfigService(t)
	err := svc.UpdateConfig(bgCtx, "app.name", "OpsMind", 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	val, err := svc.GetConfig(bgCtx, "app.name")
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if val != "OpsMind" {
		t.Errorf("期望 'OpsMind', got '%v'", val)
	}
}

// TestConfigService_UpdateConfig_NilValue 验证更新 nil 值应被拒绝。
func TestConfigService_UpdateConfig_NilValue(t *testing.T) {
	svc := setupConfigService(t)
	err := svc.UpdateConfig(bgCtx, "should.fail", nil, 1)
	if err == nil {
		t.Fatal("期望错误（nil value）, got nil")
	}
}
