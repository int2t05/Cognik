//go:build integration

package service_test

import (
	"encoding/json"
	"strings"
	"testing"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"
	"opsmind/pkg/crypto"
)

const testEncryptionKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// setupLLMConfigService 使用真实 DB 创建 LLMConfigService 实例。
func setupLLMConfigService(t *testing.T) *service.LLMConfigService {
	t.Helper()
	knowledgeSvcDB.Exec("DELETE FROM llm_configs")
	repo := repository.NewLlmConfigRepo(knowledgeSvcDB)
	svc, err := service.NewLLMConfigService(repo, knowledgeSvcDB, nil)
	if err != nil {
		t.Fatalf("NewLLMConfigService 失败: %v", err)
	}
	return svc
}

// TestLLMConfigService_CreateDefault 验证创建默认配置并可通过 GetConfig 读取。
func TestLLMConfigService_CreateDefault(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, err := svc.CreateConfig(bgCtx, "llama.cpp 本地", "http://llama-cpp:8080/v1", "", "", "", "qwen3-4b", "bge-m3", "", 8192, 1024, true)
	if err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}

	mgr := svc.GetManager()
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig 应返回默认配置, 实际 nil")
	}
	if cfg.LLMBaseURL != "http://llama-cpp:8080/v1" {
		t.Errorf("BaseURL = %q, 期望 http://llama-cpp:8080/v1", cfg.LLMBaseURL)
	}
}

// TestLLMConfigService_DefaultUnique 验证 is_default 唯一性约束（真实 DB）。
func TestLLMConfigService_DefaultUnique(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "默认1", "http://a:8080/v1", "", "", "", "m1", "e1", "", 8192, 1024, true)
	_, err := svc.CreateConfig(bgCtx, "默认2", "http://b:8080/v1", "key", "", "key", "m2", "e2", "", 4096, 1536, true)
	if err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}

	mgr := svc.GetManager()
	cfg := mgr.GetConfig()
	if cfg.LLMModel != "m2" {
		t.Errorf("新默认应为 m2, 实际 %s", cfg.LLMModel)
	}

	configs, _ := svc.ListConfigs(bgCtx)
	defaults := 0
	for _, c := range configs {
		if c.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("is_default=true 的配置数应为 1, 实际 %d", defaults)
	}
}

// TestLLMConfigService_DeleteDefault 验证删除默认配置被拒绝（真实 DB）。
func TestLLMConfigService_DeleteDefault(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "默认", "http://x:8080/v1", "", "", "", "m", "e", "", 8192, 1024, true)

	configs, _ := svc.ListConfigs(bgCtx)
	err := svc.DeleteConfig(bgCtx, configs[0].ID)
	if err == nil {
		t.Error("删除默认配置应返回错误")
	}
}

// TestLLMConfigService_UpdateHotReload 验证更新后热替换即时生效（真实 DB）。
func TestLLMConfigService_UpdateHotReload(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "默认", "http://a:8080/v1", "", "", "", "m1", "e1", "", 8192, 1024, true)

	configs, _ := svc.ListConfigs(bgCtx)
	id := configs[0].ID

	updated := &model.LlmConfig{
		ID: id, Name: "默认更新",
		LLMBaseURL: "https://api.openai.com/v1", LLMAPIKey: "sk-key",
		LLMModel: "gpt-4o", EmbeddingModel: "text-embedding-3-small",
		MaxTokens: 4096, VectorDimension: 1536, IsDefault: true,
	}
	if err := svc.UpdateConfig(bgCtx, updated); err != nil {
		t.Fatalf("UpdateConfig 失败: %v", err)
	}

	mgr := svc.GetManager()
	cfg := mgr.GetConfig()
	if cfg.LLMBaseURL != "https://api.openai.com/v1" {
		t.Errorf("热替换后 BaseURL = %q, 期望 https://api.openai.com/v1", cfg.LLMBaseURL)
	}
}

// TestLLMConfigService_ListConfigs 验证列出全部配置（真实 DB）。
func TestLLMConfigService_ListConfigs(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "cfg1", "http://a:8080/v1", "", "", "", "m1", "e1", "", 8192, 1024, false)
	_, _ = svc.CreateConfig(bgCtx, "cfg2", "http://b:8080/v1", "k", "", "k", "m2", "e2", "", 4096, 1536, false)

	configs, err := svc.ListConfigs(bgCtx)
	if err != nil {
		t.Fatalf("ListConfigs 失败: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("期望 2 条配置, 实际 %d", len(configs))
	}
}

// TestLLMConfigService_NoDefaultFallback 验证无默认配置时的降级行为（真实 DB）。
func TestLLMConfigService_NoDefaultFallback(t *testing.T) {
	svc := setupLLMConfigService(t)

	mgr := svc.GetManager()
	cfg := mgr.GetConfig()
	if cfg != nil {
		t.Error("无默认配置时 GetConfig 应返回 nil")
	}
}

// TestLLMConfigManager_ZeroLockReads 验证 GetConfig 零锁读取（真实 DB）。
func TestLLMConfigManager_ZeroLockReads(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "默认", "http://x:8080/v1", "", "", "", "m", "e", "", 8192, 1024, true)

	mgr := svc.GetManager()
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			cfg := mgr.GetConfig()
			if cfg == nil || cfg.LLMModel != "m" {
				t.Errorf("并发读取返回异常值: %v", cfg)
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestLLMConfigService_APIKeyMasked 验证 API Key 脱敏（真实 DB）。
func TestLLMConfigService_APIKeyMasked(t *testing.T) {
	svc := setupLLMConfigService(t)

	_, _ = svc.CreateConfig(bgCtx, "openai", "https://api.openai.com/v1", "sk-1234567890abcdef", "", "sk-1234567890abcdef", "gpt-4o", "text-3-small", "", 4096, 1536, false)

	configs, _ := svc.ListConfigs(bgCtx)
	if len(configs) == 0 {
		t.Fatal("应有配置")
	}

	apiKey := configs[0].LLMAPIKey
	if apiKey == "sk-1234567890abcdef" {
		t.Error("列表中 API Key 应脱敏显示, 不能返回完整值")
	}
	if len(apiKey) == 0 {
		t.Error("API Key 脱敏后不应为空")
	}
}

// TestLLMConfigService_UpdateWithoutAPIKeyDoesNotDoubleEncrypt verifies that editing
// non-secret fields keeps the existing API key readable when encryption is enabled.
func TestLLMConfigService_UpdateWithoutAPIKeyDoesNotDoubleEncrypt(t *testing.T) {
	if err := crypto.Init(testEncryptionKey); err != nil {
		t.Fatalf("crypto.Init failed: %v", err)
	}
	defer crypto.Init("")

	svc := setupLLMConfigService(t)

	created, err := svc.CreateConfig(bgCtx, "openai", "https://api.openai.com/v1", "sk-original", "", "sk-original", "gpt-4o", "text-3-small", "", 4096, 1536, true)
	if err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}
	if created.LLMAPIKey != "sk-original" {
		t.Fatalf("created APIKey = %q, want sk-original", created.LLMAPIKey)
	}

	var rawBefore string
	if err := knowledgeSvcDB.Raw("SELECT api_key FROM llm_configs WHERE id = ?", created.ID).Scan(&rawBefore).Error; err != nil {
		t.Fatalf("query raw api_key failed: %v", err)
	}
	if !strings.HasPrefix(rawBefore, "cipher:") {
		t.Fatalf("stored api_key should be prefixed ciphertext, got %q", rawBefore)
	}

	updated := &model.LlmConfig{
		ID:              created.ID,
		Name:            "openai updated",
		LLMBaseURL:      "https://api.openai.com/v1",
		LLMAPIKey:       "",
		LLMModel:        "gpt-4o-mini",
		EmbeddingModel:  "text-3-small",
		MaxTokens:       2048,
		VectorDimension: 1536,
		IsDefault:       true,
	}
	if err := svc.UpdateConfig(bgCtx, updated); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	var rawAfter string
	if err := knowledgeSvcDB.Raw("SELECT api_key FROM llm_configs WHERE id = ?", created.ID).Scan(&rawAfter).Error; err != nil {
		t.Fatalf("query updated raw api_key failed: %v", err)
	}
	if !strings.HasPrefix(rawAfter, "cipher:") {
		t.Fatalf("updated api_key should remain prefixed ciphertext, got %q", rawAfter)
	}

	cfg, err := svc.GetConfig(bgCtx, created.ID)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if cfg.LLMAPIKey != "sk-original" {
		t.Fatalf("APIKey after non-secret update = %q, want sk-original", cfg.LLMAPIKey)
	}
	if mgrCfg := svc.GetManager().GetConfig(); mgrCfg == nil || mgrCfg.LLMAPIKey != "sk-original" {
		t.Fatalf("manager APIKey = %#v, want sk-original", mgrCfg)
	}
}

// TestLLMConfigResponse_MarshalJSON_MasksAPIKey 验证 MarshalJSON 自动脱敏。
func TestLLMConfigResponse_MarshalJSON_MasksAPIKey(t *testing.T) {
	resp := service.LlmConfigResponse{
		ID:       1,
		Name:     "openai",
		LLMAPIKey: "sk-1234567890abcdefghij",
		LLMModel: "gpt-4o",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	apiKey, ok := result["llm_api_key"].(string)
	if !ok {
		t.Fatal("api_key 字段缺失")
	}
	if apiKey == "sk-1234567890abcdefghij" {
		t.Error("JSON 序列化应自动脱敏 API Key, 不能包含完整值")
	}
	if len(apiKey) < 8 {
		t.Errorf("脱敏后的 API Key 长度不足: %q (%d)", apiKey, len(apiKey))
	}
}
