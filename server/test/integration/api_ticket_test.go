//go:build integration

// api_ticket_test.go — 申告管理接口集成测试（tickets.md 全覆盖）。
//
// 测试端点：
//   门户端: POST /portal/tickets | GET /portal/tickets | GET /portal/tickets/:id | PATCH /portal/tickets/:id/supplement
//   后台:   GET /admin/tickets | GET /admin/tickets/:id | PATCH /admin/tickets/:id/status
//           POST /admin/tickets/:id/records | POST /admin/tickets/:id/knowledge-candidate
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── Portal Create ────────────────────────────────────────

func TestAPI_Ticket_PortalCreateFull(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doReporter(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": "full test ticket", "description": "with all fields", "urgency": 2, "impact_scope": 2,
		"affected_systems": []string{"Email", "VPN"}, "contact_phone": "13800003001",
		"contact_email": "reporter@opsmind.local",
	})
	assertCode(t, resp, 0)

	var id int64
	ts.DB.Raw("SELECT id FROM tickets WHERE title = 'full test ticket'").Scan(&id)
	assert.NotZero(t, id, "申告应被创建")
}

func TestAPI_Ticket_PortalCreateWithChatContext(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doReporter(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": "from chat", "description": "converted from chat", "urgency": 1, "contact_phone": "13800003002",
		"chat_context": map[string]interface{}{
			"session_id": 42, "question": "test?", "answer": "try steps...", "confidence": 0.45,
		},
	})
	assertCode(t, resp, 0)
}

func TestAPI_Ticket_PortalCreateMissingTitle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doReporter(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"description": "no title", "urgency": 1, "contact_phone": "13800003003",
	}))
}

func TestAPI_Ticket_PortalCreateMissingPhone(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doReporter(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": "no phone", "description": "desc", "urgency": 1,
	}))
}

// ── Portal List ──────────────────────────────────────────

func TestAPI_Ticket_PortalList(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 用 reporter token 创建申告
	ts.seedTicket(t, "reporter ticket", "test", "13800003004")

	body := assertOK(t, ts.doReporter(t, http.MethodGet, "/api/v1/portal/tickets?page=1&page_size=10", nil))
	tickets := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(tickets), 1)
	tk := tickets[0].(map[string]interface{})
	assert.NotEmpty(t, tk["ticket_no"], "应含 ticket_no")
	assert.NotEmpty(t, tk["status_text"], "应含 status_text")
	assert.NotNil(t, body["total"], "应含 total")
}

// ── Portal Detail ────────────────────────────────────────

func TestAPI_Ticket_PortalDetail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "portal detail", "test desc", "13800003005")

	body := assertOK(t, ts.doReporter(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/tickets/%d", id), nil))
	detail := body["data"].(map[string]interface{})
	assert.Equal(t, "portal detail", detail["title"])
	assert.NotEmpty(t, detail["description"])
	assert.NotEmpty(t, detail["status_text"])
}

func TestAPI_Ticket_PortalDetailNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doReporter(t, http.MethodGet, "/api/v1/portal/tickets/99999", nil))
}

// ── Portal Supplement ────────────────────────────────────

func TestAPI_Ticket_PortalSupplement(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "supplement test", "desc", "13800003006")
	// 先转为需补充信息状态
	ts.DB.Exec("UPDATE tickets SET status = 3 WHERE id = $1", id)

	assertCode(t, ts.doReporter(t, http.MethodPatch, fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id),
		map[string]string{"content": "补充信息: screenshot attached"}), 0)
}

func TestAPI_Ticket_PortalSupplementNotNeeded(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "supplement fail", "desc", "13800003007")
	// 状态=1（待处理），非需补充信息状态

	assertBadRequest(t, ts.doReporter(t, http.MethodPatch, fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id),
		map[string]string{"content": "x"}))
}

func TestAPI_Ticket_PortalSupplementEmptyContent(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "supplement empty", "desc", "13800003008")
	ts.DB.Exec("UPDATE tickets SET status = 3 WHERE id = $1", id)

	assertBadRequest(t, ts.doReporter(t, http.MethodPatch, fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id),
		map[string]string{"content": ""}))
}

// ── Admin List ───────────────────────────────────────────

func TestAPI_Ticket_AdminList(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	for i := 0; i < 2; i++ {
		ts.seedTicket(t, "admin list ticket", "desc", "13800003009")
	}

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?page=1&page_size=10", nil))
	assert.GreaterOrEqual(t, len(body["data"].([]interface{})), 2)
}

func TestAPI_Ticket_AdminListStatusFilter(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.seedTicket(t, "status filter", "desc", "13800003010")

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?status=1", nil))
	assert.GreaterOrEqual(t, len(body["data"].([]interface{})), 1, "status=1 应找到待处理申告")
}

func TestAPI_Ticket_AdminListUrgencyFilter(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?urgency=1", nil))
	// 已验证过的种子申告 urgency=1，如果结果为空也不应有 code 错误
	t.Logf("urgency=1 结果数: %d", len(body["data"].([]interface{})))
}

// ── Admin Detail ─────────────────────────────────────────

func TestAPI_Ticket_AdminDetail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "admin detail test", "test desc", "13800003011")

	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/tickets/%d", id), nil))
	detail := body["data"].(map[string]interface{})
	assert.Equal(t, "admin detail test", detail["title"])
	assert.NotEmpty(t, detail["ticket_no"])
}

// ── Admin State Machine ──────────────────────────────────

func TestAPI_Ticket_AdminStart(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "start test", "desc", "13800003012")
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "start", "result": "assigned to engineer"}), 0)
}

func TestAPI_Ticket_AdminRequestInfo(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "request info test", "desc", "13800003013")
	ts.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", id)

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "request_info", "result": "need more details"}), 0)
}

func TestAPI_Ticket_AdminResolve(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "resolve test", "desc", "13800003014")
	ts.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", id)

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "resolve", "result": "fixed"}), 0)
}

func TestAPI_Ticket_AdminClose(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "close test", "desc", "13800003015")
	// close 可从待处理(1)直接执行
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "close", "result": "not needed"}), 0)
}

func TestAPI_Ticket_AdminCloseResolved(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "resolved no close", "desc", "13800003016")
	ts.DB.Exec("UPDATE tickets SET status = 4 WHERE id = $1", id)

	// 已解决不能再关闭
	assertBadRequest(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "close", "result": "x"}))
}

func TestAPI_Ticket_AdminInvalidAction(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "invalid action", "desc", "13800003017")

	assertBadRequest(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "nonexistent"}))
}

func TestAPI_Ticket_AdminInvalidTransition(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "bad transition", "desc", "13800003018")
	// 待处理(1) → resolve 不是有效转换（需要先 start）
	assertBadRequest(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "resolve", "result": "x"}))
}

// ── Admin Records ────────────────────────────────────────

func TestAPI_Ticket_AdminAddRecord(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "add record", "desc", "13800003019")

	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/records", id),
		map[string]string{"action": "note", "content": "contacted user"}), 0)
}

func TestAPI_Ticket_AdminAddRecordInvalidAction(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "invalid record", "desc", "13800003020")

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/records", id),
		map[string]string{"action": "invalid_op", "content": "x"}))
}

func TestAPI_Ticket_AdminAddRecordMissingAction(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "record no action", "desc", "13800003021")

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/records", id),
		map[string]string{"content": "x"}))
}

// ── Knowledge Candidate ──────────────────────────────────

func TestAPI_Ticket_KnowledgeCandidate(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "candidate-kb")
	id := ts.seedTicket(t, "vpn timeout for kb", "vpn issue details", "13800003022")

	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/knowledge-candidate", id),
		map[string]interface{}{"kb_id": kbID}), 0)
}

func TestAPI_Ticket_KnowledgeCandidateNoKB(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "candidate no kb", "desc", "13800003023")

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/knowledge-candidate", id),
		map[string]interface{}{}))
}

// ── Portal Non-Owner ─────────────────────────────────────

func TestAPI_Ticket_PortalDetailNonOwner(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "portal detail non-owner", "test", "13800003024")

	// admin（非创建者）通过门户端查看 → 应拒绝
	assertForbidden(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/tickets/%d", id), nil))
}

func TestAPI_Ticket_PortalSupplementNonOwner(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "supplement non-owner", "desc", "13800003025")
	ts.DB.Exec("UPDATE tickets SET status = 3 WHERE id = $1", id)

	// admin（非创建者）通过门户端补充 → 应拒绝
	assertForbidden(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id),
		map[string]string{"content": "补充信息"}))
}

func TestAPI_Ticket_AdminRequestInfoLimit(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "request info limit", "desc", "13800003026")

	// 先转为处理中
	ts.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", id)

	// 前 3 次 request_info 应成功
	for i := 0; i < 3; i++ {
		assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
			map[string]string{"action": "request_info", "result": fmt.Sprintf("round %d", i+1)}), 0)
		// request_info 后状态变为 3（需补充信息），模拟补充后重回处理中
		ts.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", id)
	}

	// 第 4 次应拒绝（已达上限 3 次）
	assertBadRequest(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "request_info", "result": "too many"}))
}
