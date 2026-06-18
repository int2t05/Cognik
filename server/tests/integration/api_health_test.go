//go:build integration

// api_health_test.go — 健康检查接口集成测试。
//
// 测试端点：
//   - GET /health
package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Health_Liveness(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodGet, "/health", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "/health 应返回 200")
	assert.Equal(t, "ok", parseBody(t, resp)["status"], "/health status 应为 ok")
}

func TestAPI_Health_NoAuth(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 健康检查不需要认证
	resp := ts.do(t, http.MethodGet, "/health", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
