//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── Dashboard ───────────────────────────────────────────

func TestAPI_Dashboard_Stats(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	data := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/stats", nil))["data"].(map[string]interface{})
	for _, f := range []string{"today_tickets", "pending_tickets", "processing_tickets", "resolved_tickets", "today_chats", "avg_confidence", "knowledge_count"} {
		_, ok := data[f]
		assert.True(t, ok, "missing field: %s", f)
	}
}

func TestAPI_Dashboard_Trends(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/trends?start_date=2026-06-01&end_date=2026-06-18&granularity=day", nil))
	assert.True(t, body["data"].(map[string]interface{})["data_points"] != nil)
}

func TestAPI_Dashboard_TrendsValidation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/trends?start_date=2026-06-18&end_date=2026-06-01", nil))["code"])
}

// ── Audit ───────────────────────────────────────────────

func TestAPI_Audit_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	ts.seedKB(t, "audit-kb") // 产生审计日志
	data, ok := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?page=1&page_size=10", nil))["data"].([]interface{})
	assert.True(t, ok)
	if len(data) > 0 {
		entry := data[0].(map[string]interface{})
		assert.NotEmpty(t, entry["action"])
		assert.NotEmpty(t, entry["operator_name"])
	}
}

func TestAPI_Audit_Filtering(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?action=knowledge.create", nil))
	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?target_type=knowledge_article", nil))
}

// ── Messages ────────────────────────────────────────────

func TestAPI_Message_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 通过 DB 插入测试消息
	ts.DB.Exec(`INSERT INTO messages (user_id, title, content, type, related_type, related_id, is_read, created_at)
		VALUES ($1, 'test', 'body', 'system_notice', '', 0, false, NOW())`, ts.AdminID)

	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?page=1&page_size=10", nil))
	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?type=system_notice", nil))
	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages/unread-count", nil))
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPut, "/api/v1/portal/messages/99999/read", nil))["code"])
}

// ── Config ──────────────────────────────────────────────

func TestAPI_Config_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertCode(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/configs/app_name", map[string]interface{}{"value": "OpsMind-Test"}), 0)
	assert.Equal(t, "OpsMind-Test", assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil))["data"])
}

// ── Health ──────────────────────────────────────────────

func TestAPI_Health_Liveness(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	resp := ts.do(t, http.MethodGet, "/health", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", parseBody(t, resp)["status"])
}

func TestAPI_Health_Readiness(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	resp := ts.do(t, http.MethodGet, "/readyz", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ready", parseBody(t, resp)["status"])
}
