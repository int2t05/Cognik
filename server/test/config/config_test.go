package config_test

import (
	"path/filepath"
	"testing"

	"opsmind/internal/config"
)

// TestLoad_DefaultValues 验证从 config.yaml 加载默认值
func TestLoad_DefaultValues(t *testing.T) {
	// 使用项目中的 config.yaml
	cfgPath := filepath.Join("..", "..", "internal", "config", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() 失败: %v", err)
	}

	// 验证 config.yaml 中定义的默认值
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, 期望 8080", cfg.Server.Port)
	}

	if cfg.Server.Mode != "debug" {
		t.Errorf("Server.Mode = %q, 期望 debug", cfg.Server.Mode)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, 期望 localhost", cfg.Database.Host)
	}

	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, 期望 5432", cfg.Database.Port)
	}

	if cfg.Database.User != "opsmind" {
		t.Errorf("Database.User = %q, 期望 opsmind", cfg.Database.User)
	}

	if cfg.Database.DBName != "opsmind" {
		t.Errorf("Database.DBName = %q, 期望 opsmind", cfg.Database.DBName)
	}

	if cfg.Database.SSLMode != "disable" {
		t.Errorf("Database.SSLMode = %q, 期望 disable", cfg.Database.SSLMode)
	}

	if cfg.MinIO.Endpoint != "localhost:9000" {
		t.Errorf("MinIO.Endpoint = %q, 期望 localhost:9000", cfg.MinIO.Endpoint)
	}

	if cfg.MinIO.AccessKey != "minioadmin" {
		t.Errorf("MinIO.AccessKey = %q, 期望 minioadmin", cfg.MinIO.AccessKey)
	}

	if cfg.MinIO.SecretKey != "minioadmin" {
		t.Errorf("MinIO.SecretKey = %q, 期望 minioadmin", cfg.MinIO.SecretKey)
	}

	if cfg.MinIO.UseSSL != false {
		t.Error("MinIO.UseSSL = true, 期望 false")
	}

	// LLM 配置
	if cfg.LLM.BaseURL != "http://llama-cpp:8081/v1" {
		t.Errorf("LLM.BaseURL = %q, 期望 http://llama-cpp:8081/v1", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "Qwen3-4B-Q4_K_M" {
		t.Errorf("LLM.Model = %q, 期望 Qwen3-4B-Q4_K_M", cfg.LLM.Model)
	}
	if cfg.LLM.MaxTokens != 8192 {
		t.Errorf("LLM.MaxTokens = %d, 期望 8192", cfg.LLM.MaxTokens)
	}

	// Embedding 配置（与 LLM 共用 BaseURL，但独立模型名和维度）
	if cfg.Embedding.Model != "Qwen3-Embedding-0.6B-Q8_0" {
		t.Errorf("Embedding.Model = %q, 期望 Qwen3-Embedding-0.6B-Q8_0", cfg.Embedding.Model)
	}
	if cfg.Embedding.Dimension != 1024 {
		t.Errorf("Embedding.Dimension = %d, 期望 1024", cfg.Embedding.Dimension)
	}

	if cfg.AI.ConfidenceThreshold != 0.6 {
		t.Errorf("AI.ConfidenceThreshold = %f, 期望 0.6", cfg.AI.ConfidenceThreshold)
	}

	if cfg.AI.DefaultTopK != 5 {
		t.Errorf("AI.DefaultTopK = %d, 期望 5", cfg.AI.DefaultTopK)
	}
}

// TestLoad_EnvOverride 验证环境变量覆盖配置文件值
func TestLoad_EnvOverride(t *testing.T) {
	// t.Setenv 在测试结束时自动恢复原值，并标记测试不可并行
	t.Setenv("OPSMIND_SERVER_PORT", "9090")
	t.Setenv("OPSMIND_DATABASE_HOST", "remote-host")
	t.Setenv("OPSMIND_DATABASE_PORT", "5433")
	t.Setenv("OPSMIND_LLM_API_KEY", "env-api-key-override")
	t.Setenv("OPSMIND_LLM_BASE_URL", "https://api.deepseek.com/v1")
	t.Setenv("OPSMIND_EMBEDDING_MODEL", "text-embedding-3-large")
	t.Setenv("OPSMIND_EMBEDDING_API_KEY", "sk-embedding-override-key")
	t.Setenv("OPSMIND_JWT_SECRET", "test-jwt-secret-32chars-long!!!!!")

	cfgPath := filepath.Join("..", "..", "internal", "config", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() 失败: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, 期望 9090（环境变量覆盖）", cfg.Server.Port)
	}

	if cfg.Database.Host != "remote-host" {
		t.Errorf("Database.Host = %q, 期望 remote-host（环境变量覆盖）", cfg.Database.Host)
	}

	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, 期望 5433（环境变量覆盖）", cfg.Database.Port)
	}

	// 验证 LLM 配置被环境变量覆盖
	if cfg.LLM.APIKey != "env-api-key-override" {
		t.Errorf("LLM.APIKey = %q, 期望 env-api-key-override（环境变量覆盖）", cfg.LLM.APIKey)
	}
	if cfg.LLM.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("LLM.BaseURL = %q, 期望 https://api.deepseek.com/v1（环境变量覆盖）", cfg.LLM.BaseURL)
	}

	// 验证 Embedding 配置被环境变量覆盖
	if cfg.Embedding.Model != "text-embedding-3-large" {
		t.Errorf("Embedding.Model = %q, 期望 text-embedding-3-large（环境变量覆盖）", cfg.Embedding.Model)
	}
	if cfg.Embedding.APIKey != "sk-embedding-override-key" {
		t.Errorf("Embedding.APIKey = %q, 期望 sk-embedding-override-key（环境变量覆盖）", cfg.Embedding.APIKey)
	}

	if cfg.JWT.Secret != "test-jwt-secret-32chars-long!!!!!" {
		t.Errorf("JWT.Secret = %q, 期望 test-jwt-secret-32chars-long!!!!!", cfg.JWT.Secret)
	}
}

// TestLoad_StructFields 验证所有配置结构体字段被正确填充
func TestLoad_StructFields(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "internal", "config", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() 失败: %v", err)
	}

	// 验证 Server 结构体
	if cfg.Server.Port == 0 {
		t.Error("Server.Port 未填充")
	}
	if cfg.Server.Mode == "" {
		t.Error("Server.Mode 未填充")
	}

	// 验证 Database 结构体
	if cfg.Database.Host == "" {
		t.Error("Database.Host 未填充")
	}
	if cfg.Database.Port == 0 {
		t.Error("Database.Port 未填充")
	}
	if cfg.Database.User == "" {
		t.Error("Database.User 未填充")
	}
	if cfg.Database.DBName == "" {
		t.Error("Database.DBName 未填充")
	}
	if cfg.Database.SSLMode == "" {
		t.Error("Database.SSLMode 未填充")
	}

	// 验证 MinIO 结构体
	if cfg.MinIO.Endpoint == "" {
		t.Error("MinIO.Endpoint 未填充")
	}
	if cfg.MinIO.AccessKey == "" {
		t.Error("MinIO.AccessKey 未填充")
	}
	if cfg.MinIO.SecretKey == "" {
		t.Error("MinIO.SecretKey 未填充")
	}

	// 验证 LLM 结构体
	if cfg.LLM.BaseURL == "" {
		t.Error("LLM.BaseURL 未填充")
	}
	if cfg.LLM.Model == "" {
		t.Error("LLM.Model 未填充")
	}
	if cfg.LLM.MaxTokens == 0 {
		t.Error("LLM.MaxTokens 未填充")
	}

	// 验证 Embedding 结构体
	if cfg.Embedding.Model == "" {
		t.Error("Embedding.Model 未填充")
	}
	if cfg.Embedding.Dimension == 0 {
		t.Error("Embedding.Dimension 未填充")
	}
	// APIKey 可为空（本地 llama.cpp 不需要），但字段必须存在
	_ = cfg.Embedding.APIKey

	// 验证 AI 结构体
	if cfg.AI.ConfidenceThreshold == 0 {
		t.Error("AI.ConfidenceThreshold 未填充")
	}
	if cfg.AI.DefaultTopK == 0 {
		t.Error("AI.DefaultTopK 未填充")
	}
}

// TestLoad_LLMConfigHotReload 验证 LLM 配置包含热替换所需的所有字段。
//
// LLMConfigManager（M4 Service 层）使用 atomic.Value 进行零锁热替换，
// 要求 LLMConfig 包含所有调用 LLMClient 所需的字段（BaseURL/APIKey/Model/MaxTokens）。
func TestLoad_LLMConfigHotReload(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "internal", "config", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() 失败: %v", err)
	}

	// LLM 热替换关键字段：BaseURL、APIKey、Model、MaxTokens 缺一不可
	if cfg.LLM.BaseURL == "" {
		t.Error("LLM.BaseURL 为空 — 热替换 LLM 客户端需要 BaseURL")
	}
	if cfg.LLM.Model == "" {
		t.Error("LLM.Model 为空 — 热替换需要知道模型名称")
	}
	if cfg.LLM.MaxTokens <= 0 {
		t.Errorf("LLM.MaxTokens = %d, 期望 > 0", cfg.LLM.MaxTokens)
	}
	// APIKey 可以为空（llama.cpp 不需要），但字段必须存在
}
