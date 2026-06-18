//go:build integration

// api_permission_test.go — 跨模块 RBAC 权限矩阵集成测试。
//
// 验证：
//   - 报障人 (Reporter) 无法访问任何后台管理端点
//   - 运维人员 (Operator) 有 ticket/knowledge 权限但无 user:manage/system:config
//   - 无 Token 的请求被拒绝
package integration_test

import (
	"net/http"
	"testing"
)

// ── Reporter (报障人 — 仅门户端) ────────────────────────

func TestAPI_Perm_Reporter_AdminUsers(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/users", nil))
}

func TestAPI_Perm_Reporter_AdminRoles(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/roles", nil))
}

func TestAPI_Perm_Reporter_AdminTickets(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/tickets", nil))
}

func TestAPI_Perm_Reporter_AdminKB(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/knowledge-bases", nil))
}

func TestAPI_Perm_Reporter_AdminLLM(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))
}

func TestAPI_Perm_Reporter_AdminDashboard(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/dashboard/stats", nil))
}

func TestAPI_Perm_Reporter_AdminAudit(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/audit-logs", nil))
}

func TestAPI_Perm_Reporter_AdminConfig(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertForbidden(t, ts.doReporter(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil))
}

// ── Operator (运维人员 — ticket+knowledge 权限) ────────

func TestAPI_Perm_Operator_TicketRead(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 运维人员应有 ticket:read 权限
	assertCode(t, ts.doOperator(t, http.MethodGet, "/api/v1/admin/tickets?page=1&page_size=10", nil), 0)
}

func TestAPI_Perm_Operator_UserManage(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 运维人员无 user:manage 权限
	assertForbidden(t, ts.doOperator(t, http.MethodGet, "/api/v1/admin/users", nil))
}

func TestAPI_Perm_Operator_SystemConfig(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 运维人员无 system:config 权限
	assertForbidden(t, ts.doOperator(t, http.MethodGet, "/api/v1/admin/llm-configs", nil))
}

// ── No Auth ──────────────────────────────────────────────

func TestAPI_Perm_NoAuth(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 无 Token 访问任何受保护端点
	assertUnauthorized(t, ts.do(t, http.MethodGet, "/api/v1/admin/users", nil, ""))
	assertUnauthorized(t, ts.do(t, http.MethodGet, "/api/v1/admin/llm-configs", nil, ""))
	assertUnauthorized(t, ts.do(t, http.MethodGet, "/api/v1/portal/tickets", nil, ""))
	assertUnauthorized(t, ts.do(t, http.MethodGet, "/api/v1/portal/chat-sessions", nil, ""))
}
