//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Ticket_PortalLifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ticketID := ts.seedTicket(t, "email issue", "cannot login", "13800001111")

	// List — 验证字段
	tickets := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/tickets?page=1&page_size=10", nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(tickets), 1)
	tk := tickets[0].(map[string]interface{})
	assert.NotEmpty(t, tk["ticket_no"])
	assert.Equal(t, "email issue", tk["title"])
	assert.Equal(t, float64(1), tk["status"]) // 待处理

	// Detail — 验证核心字段
	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/tickets/%d", ticketID), nil))["data"].(map[string]interface{})
	assert.Equal(t, "email issue", detail["title"])
	assert.NotEmpty(t, detail["description"])
	assert.NotEmpty(t, detail["status_text"])

	// Supplement — 非需补充信息状态应拒绝
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", ticketID),
		map[string]string{"content": "x"}))["code"])
}

func TestAPI_Ticket_AdminDetail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "admin detail test", "test desc", "13800001999")

	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/tickets/%d", id), nil))["data"].(map[string]interface{})
	assert.Equal(t, "admin detail test", detail["title"])
	assert.NotEmpty(t, detail["ticket_no"])
	assert.NotEmpty(t, detail["status_text"])
}

func TestAPI_Ticket_AdminStateMachine(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	id := ts.seedTicket(t, "printer broken", "3rd floor", "13800001112")

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "start", "result": "assigned"}), 0)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "request_info", "result": "need model"}), 0)

	ts.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", id)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "resolve", "result": "fixed"}), 0)

	// Add record
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/records", id),
		map[string]string{"action": "note", "content": "done"}), 0)
}

func TestAPI_Ticket_AdminFiltering(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	for i := 0; i < 2; i++ {
		ts.seedTicket(t, "filter test", "desc", "13800001100")
	}
	items := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?status=1", nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(items), 2)
}

func TestAPI_Ticket_StateValidation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	id := ts.seedTicket(t, "state test", "desc", "13800001113")

	assert.Equal(t, http.StatusBadRequest, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "invalid"}).StatusCode)
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", id),
		map[string]string{"action": "resolve"}))["code"])
}

func TestAPI_Ticket_KnowledgeCandidate(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	kbID := ts.seedKB(t, "candidate-kb")
	id := ts.seedTicket(t, "vpn timeout", "vpn issue", "13800001114")
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/knowledge-candidate", id),
		map[string]interface{}{"kb_id": kbID}), 0)
}
