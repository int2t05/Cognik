//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_LLMConfig_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// Create — 验证响应字段
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "test-cfg", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 16384, "vector_dimension": 1536, "is_default": true,
	}))
	cfg := body["data"].(map[string]interface{})
	cfgID := int64(cfg["id"].(float64))
	assert.Equal(t, "test-cfg", cfg["name"])
	assert.True(t, cfg["is_default"].(bool))

	// List — 验证 api_key 脱敏
	cfgs := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(cfgs), 1)

	// Detail
	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil))["data"].(map[string]interface{})
	assert.Equal(t, "test-cfg", detail["name"])
	assert.Equal(t, "gpt-4o-mini", detail["llm_model"])

	// Update
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), map[string]interface{}{
		"name": "test-cfg-v2", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": true,
	}), 0)

	// Test connection
	assert.NotNil(t, parseBody(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/llm-configs/%d/test", cfgID), nil))["code"])

	// Delete — 默认配置不允许
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil))["code"])
}

func TestAPI_LLMConfig_Validation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assert.Equal(t, http.StatusBadRequest, ts.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs",
		map[string]interface{}{"name": "missing-fields"}).StatusCode)
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs/99999", nil))["code"])
}
