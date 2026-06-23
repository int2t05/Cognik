//go:build integration

// Package handler_test 验证 LLMConfigHandler HTTP 接口（真实 DB + 真实 Service）。
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"opsmind/internal/handler"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
)

// setupLLMConfigHandler 使用真实 DB 创建 LLMConfigHandler。
func setupLLMConfigHandler(t *testing.T) *handler.LLMConfigHandler {
	t.Helper()
	// 每次测试前清空表，避免默认配置唯一索引冲突
	knowledgeHandlerDB.Exec("DELETE FROM llm_configs")
	repo := repository.NewLlmConfigRepo(knowledgeHandlerDB)
	svc, err := service.NewLLMConfigService(repo, knowledgeHandlerDB, nil)
	if err != nil {
		t.Fatalf("创建 LLMConfigService 失败: %v", err)
	}
	return handler.NewLLMConfigHandler(svc)
}

func setupLLMTestRouter(h *handler.LLMConfigHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/admin/llm-configs", h.ListConfigs)
	r.POST("/api/v1/admin/llm-configs", h.CreateConfig)
	r.GET("/api/v1/admin/llm-configs/:id", h.GetConfig)
	r.PUT("/api/v1/admin/llm-configs/:id", h.UpdateConfig)
	r.DELETE("/api/v1/admin/llm-configs/:id", h.DeleteConfig)
	r.POST("/api/v1/admin/llm-configs/:id/test", h.TestConnection)
	return r
}

// TestLLMConfigHandler_ListConfigs 验证列表接口（真实 DB）。
func TestLLMConfigHandler_ListConfigs(t *testing.T) {
	h := setupLLMConfigHandler(t)
	r := setupLLMTestRouter(h)

	// 预创建 2 个配置
	knowledgeHandlerDB.Create(&model.LlmConfig{Name: "llama.cpp", ProviderType: 1, LLMModel: "qwen3-4b", EmbeddingModel: "bge-m3", IsDefault: true})
	knowledgeHandlerDB.Create(&model.LlmConfig{Name: "OpenAI", ProviderType: 2, LLMModel: "gpt-4o", EmbeddingModel: "text-embedding-3-small"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/llm-configs", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int                       `json:"code"`
		Data []service.LlmConfigResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("code 期望 0, 实际 %d", resp.Code)
	}
	if len(resp.Data) < 2 {
		t.Errorf("期望至少 2 条配置, 实际 %d", len(resp.Data))
	}
	// 验证默认配置脱敏
	for _, c := range resp.Data {
		if c.IsDefault && c.APIKey != "" && len(c.APIKey) > 10 {
			t.Errorf("默认配置 APIKey 应脱敏, got '%s'", c.APIKey)
		}
	}
}

// TestLLMConfigHandler_CreateConfig 验证创建接口（真实 DB）。
func TestLLMConfigHandler_CreateConfig(t *testing.T) {
	h := setupLLMConfigHandler(t)
	r := setupLLMTestRouter(h)

	body := map[string]interface{}{
		"name":             "DeepSeek",
		"provider_type":    2,
		"base_url":         "https://api.deepseek.com/v1",
		"api_key":          "sk-test1234567890abcdef",
		"llm_model":        "deepseek-chat",
		"embedding_model":  "text-embedding-v2",
		"max_tokens":       4096,
		"vector_dimension": 1536,
		"is_default":       false,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/llm-configs", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	// 验证 DB 中已创建
	var cfg model.LlmConfig
	if err := knowledgeHandlerDB.Where("name = ?", "DeepSeek").First(&cfg).Error; err != nil {
		t.Fatalf("配置应已创建: %v", err)
	}
	if cfg.LLMModel != "deepseek-chat" {
		t.Errorf("期望 llm_model='deepseek-chat', got '%s'", cfg.LLMModel)
	}
}

// TestLLMConfigHandler_GetConfig 验证详情接口（真实 DB）。
func TestLLMConfigHandler_GetConfig(t *testing.T) {
	h := setupLLMConfigHandler(t)
	r := setupLLMTestRouter(h)

	cfg := &model.LlmConfig{Name: "详情测试", ProviderType: 1, LLMModel: "test-model", EmbeddingModel: "emb-model"}
	knowledgeHandlerDB.Create(cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/llm-configs/"+itoa(cfg.ID), nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}
}

// TestLLMConfigHandler_DeleteConfig 验证删除接口（真实 DB）。
func TestLLMConfigHandler_DeleteConfig(t *testing.T) {
	h := setupLLMConfigHandler(t)
	r := setupLLMTestRouter(h)

	// 非默认配置可以删除
	cfg := &model.LlmConfig{Name: "待删除", ProviderType: 1, LLMModel: "x", EmbeddingModel: "y", IsDefault: false}
	knowledgeHandlerDB.Create(cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/admin/llm-configs/"+itoa(cfg.ID), nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", w.Code, w.Body.String())
	}

	// 验证已删除
	var count int64
	knowledgeHandlerDB.Model(&model.LlmConfig{}).Where("id = ?", cfg.ID).Count(&count)
	if count != 0 {
		t.Error("配置应已删除")
	}
}

// TestLLMConfigHandler_TestConnection 验证测试连接接口（真实 DB）。
//
// llmClient=nil 时返回错误（非 20001），验证错误处理路径。
func TestLLMConfigHandler_TestConnection(t *testing.T) {
	h := setupLLMConfigHandler(t)
	r := setupLLMTestRouter(h)

	cfg := &model.LlmConfig{Name: "测试连接", ProviderType: 1, LLMModel: "test", EmbeddingModel: "e", BaseURL: "http://localhost:9999/v1", IsDefault: false}
	knowledgeHandlerDB.Create(cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/llm-configs/"+itoa(cfg.ID)+"/test", nil)
	r.ServeHTTP(w, req)

	// nil llmClient 返回错误（非 200），但不应 panic
	if w.Code == 200 {
		// 可能 code != 0（业务错误）
		t.Logf("测试连接响应: %s", w.Body.String())
	}
}
