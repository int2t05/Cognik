//go:build integration

// api_config_test.go — 系统配置接口集成测试（audit-log.md 配置部分）。
//
// 测试端点：
//   - GET /api/v1/admin/configs/:key
//   - PUT /api/v1/admin/configs/:key
package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Config_SetAndGet(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// Set
	assertCode(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/configs/app_name",
		map[string]interface{}{"value": "OpsMind-Test"}), 0)

	// Get 验证
	data := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil))["data"]
	assert.Equal(t, "OpsMind-Test", data)
}

func TestAPI_Config_UpdateReadBack(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertCode(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/configs/app_name",
		map[string]interface{}{"value": "First"}), 0)
	assertCode(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/configs/app_name",
		map[string]interface{}{"value": "Second"}), 0)

	data := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil))["data"]
	assert.Equal(t, "Second", data)
}

func TestAPI_Config_GetNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/configs/nonexistent_key", nil))
}

func TestAPI_Config_UpdateEmptyKey(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPut, "/api/v1/admin/configs/", map[string]interface{}{"value": "x"})
	// 空 key 应该返回 404 或 400
	assert.True(t, resp.StatusCode == 404 || resp.StatusCode == 400, "空 key 应返回 400 或 404")
}

func TestAPI_Config_NoAuth(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil, "")
	assertUnauthorized(t, resp)
}
