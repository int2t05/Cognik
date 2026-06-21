//go:build integration

// api_llm_config_test.go — LLM 配置接口集成测试（llm-config.md 全覆盖）。
//
// 测试端点：
//   - GET    /api/v1/admin/llm-configs
//   - POST   /api/v1/admin/llm-configs
//   - GET    /api/v1/admin/llm-configs/:id
//   - PUT    /api/v1/admin/llm-configs/:id
//   - DELETE /api/v1/admin/llm-configs/:id
//   - POST   /api/v1/admin/llm-configs/:id/test
package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── Create ───────────────────────────────────────────────

func TestAPI_LLMConfig_Create(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "test-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"api_key": "sk-test-key-12345678", "llm_model": "gpt-4o-mini",
		"embedding_model": "text-embedding-3-small", "max_tokens": 16384,
		"vector_dimension": 1536, "is_default": true,
	}))
	cfg := body["data"].(map[string]interface{})
	assert.NotZero(t, int64(cfg["id"].(float64)))
	assert.Equal(t, "test-llm", cfg["name"])
}

func TestAPI_LLMConfig_CreateMissingFields(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "incomplete",
	}))
}

func TestAPI_LLMConfig_CreateDuplicateName(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body1 := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "dup-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	}))
	id1 := int64(body1["data"].(map[string]interface{})["id"].(float64))

	// 创建同名配置 — 当前实现允许同名，验证两个配置都存在
	body2 := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "dup-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": false,
	}))
	id2 := int64(body2["data"].(map[string]interface{})["id"].(float64))
	assert.NotEqual(t, id1, id2, "同名配置应有不同的 ID")
}

// ── List ─────────────────────────────────────────────────

func TestAPI_LLMConfig_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 确保至少有 1 条非默认配置
	ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "list-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	})

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))
	cfgs := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(cfgs), 1)
}

func TestAPI_LLMConfig_KeyMasked(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "masked-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"api_key": "sk-secret-key-value-here", "llm_model": "gpt-4o-mini",
		"embedding_model": "text-embedding-3-small", "max_tokens": 8192, "vector_dimension": 1536,
		"is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	// 列表中的 api_key 应脱敏
	cfgs := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))["data"].([]interface{})
	var found map[string]interface{}
	for _, c := range cfgs {
		m := c.(map[string]interface{})
		if int64(m["id"].(float64)) == cfgID {
			found = m
			break
		}
	}
	if apiKey, ok := found["api_key"].(string); ok {
		assert.False(t, strings.Contains(apiKey, "secret-key-value"), "api_key 应脱敏, got: %s", apiKey)
	}
}

// ── Detail ───────────────────────────────────────────────

func TestAPI_LLMConfig_Detail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "detail-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil))
	cfg := detail["data"].(map[string]interface{})
	assert.Equal(t, "detail-llm", cfg["name"])
	assert.Equal(t, "gpt-4o", cfg["llm_model"])
}

func TestAPI_LLMConfig_DetailNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs/99999", nil))
}

// ── Update ───────────────────────────────────────────────

func TestAPI_LLMConfig_Update(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "update-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 16384, "vector_dimension": 1536, "is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), map[string]interface{}{
		"name": "update-llm-v2", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": false,
	}), 0)
}

func TestAPI_LLMConfig_UpdateNonExistent(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/llm-configs/99999", map[string]interface{}{
		"name": "ghost", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "e", "max_tokens": 1, "vector_dimension": 1,
	}))
}

// ── Delete ───────────────────────────────────────────────

func TestAPI_LLMConfig_Delete(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "delete-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil), 0)
}

func TestAPI_LLMConfig_DeleteDefault(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "default-llm", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": true,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	resp := ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil)
	body2 := parseBody(t, resp)
	assert.NotEqual(t, float64(0), body2["code"], "默认配置不能删除")
}

func TestAPI_LLMConfig_DeleteNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodDelete, "/api/v1/admin/llm-configs/99999", nil))
}

// ── Test Connection ──────────────────────────────────────

func TestAPI_LLMConfig_TestConnection(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "test-conn", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"api_key": "", "llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	// 测试连接可能成功（网络可达）或 20001（不可达），两种情况都是预期行为
	resp := ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/llm-configs/%d/test", cfgID), nil)
	b := parseBody(t, resp)
	c := b["code"].(float64)
	assert.True(t, c == 0 || c == 20001, "测试连接应返回 0 或 20001, got code=%v", c)
}

// ── Invalid Provider Type ──────────────────────────────────

func TestAPI_LLMConfig_CreateInvalidProviderType(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "invalid-provider", "provider_type": 99, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	})
	assertBadRequest(t, resp)
}

// ── Preserve API Key on Update ─────────────────────────────

func TestAPI_LLMConfig_UpdatePreservesAPIKey(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 创建配置时包含 api_key
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "preserve-key", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"api_key": "sk-test-preserve-key-12345", "llm_model": "gpt-4o-mini",
		"embedding_model": "text-embedding-3-small", "max_tokens": 8192, "vector_dimension": 1536,
		"is_default": false,
	}))
	cfgID := int64(body["data"].(map[string]interface{})["id"].(float64))

	// 更新时不传 api_key，仅修改名称
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), map[string]interface{}{
		"name": "preserve-key-v2", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": false,
	}), 0)

	// 验证 api_key 仍被保留（脱敏显示或非空）
	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil))
	cfg := detail["data"].(map[string]interface{})
	assert.NotEmpty(t, cfg["api_key"], "更新后 api_key 应被保留")
}

// ── Default Auto-Switch ────────────────────────────────────

func TestAPI_LLMConfig_CreateDefaultAutoSwitch(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 创建第一个默认配置
	assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "default-1", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 8192, "vector_dimension": 1536, "is_default": true,
	}))

	// 创建第二个默认配置（应自动将第一个改为非默认）
	assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "default-2", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": true,
	}))

	// 验证只有第二个配置是默认，且只有一个默认配置
	cfgs := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))["data"].([]interface{})
	var defaultCount int
	for _, c := range cfgs {
		m := c.(map[string]interface{})
		isDefault, ok := m["is_default"].(bool)
		if ok && isDefault {
			defaultCount++
			assert.Equal(t, "default-2", m["name"], "默认配置应为刚刚创建的 default-2")
		}
	}
	assert.Equal(t, 1, defaultCount, "系统中应恰好有 1 个默认配置")
}
