//go:build integration

// api_audit_test.go — 审计日志接口集成测试（audit-log.md 审计部分）。
//
// 测试端点：
//   - GET /api/v1/admin/audit-logs
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Audit_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 先执行一个会产生审计日志的操作
	ts.seedKB(t, "audit-test-kb")

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?page=1&page_size=10", nil))
	data, ok := body["data"].([]interface{})
	assert.True(t, ok)
	assert.NotNil(t, body["total"])
	if len(data) > 0 {
		entry := data[0].(map[string]interface{})
		assert.NotEmpty(t, entry["action"], "应含 action")
		assert.NotNil(t, entry["operator_name"], "应含 operator_name")
		assert.NotNil(t, entry["created_at"], "应含 created_at")
	}
}

func TestAPI_Audit_ListWithActionFilter(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.seedKB(t, "audit-action-kb")
	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?action=knowledge.create", nil))
	// 验证所有结果都是 knowledge.create
	for _, item := range body["data"].([]interface{}) {
		assert.Equal(t, "knowledge.create", item.(map[string]interface{})["action"])
	}
}

func TestAPI_Audit_ListWithTargetType(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?target_type=knowledge_article", nil))
	for _, item := range body["data"].([]interface{}) {
		assert.Equal(t, "knowledge_article", item.(map[string]interface{})["target_type"])
	}
}

func TestAPI_Audit_ListWithOperator(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.seedKB(t, "audit-op-kb")
	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/audit-logs?operator_id=%d", ts.AdminID), nil))
	for _, item := range body["data"].([]interface{}) {
		opID := item.(map[string]interface{})["operator_id"]
		if opID != nil {
			assert.Equal(t, float64(ts.AdminID), opID)
		}
	}
}

func TestAPI_Audit_ListWithDateRange(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertOK(t, ts.doAuth(t, http.MethodGet,
		"/api/v1/admin/audit-logs?date_from=2026-06-01&date_to=2026-06-30", nil))
}

func TestAPI_Audit_LogCreatedOnAction(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 创建用户会产生 user.create 审计日志
	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "audit_user", "password": "AuditUsr@123", "real_name": "Audit", "phone": "13800003301",
	})

	// 查询 user.create 审计日志
	body := assertOK(t, ts.doAuth(t, http.MethodGet,
		"/api/v1/admin/audit-logs?action=user.create", nil))
	t.Logf("user.create 审计日志数: %d", len(body["data"].([]interface{})))
	assert.GreaterOrEqual(t, len(body["data"].([]interface{})), 1,
		"创建用户应产生至少 1 条审计日志")
}
