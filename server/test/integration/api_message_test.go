//go:build integration

// api_message_test.go — 站内消息接口集成测试（audit-log.md 消息部分）。
//
// 测试端点：
//   - GET /api/v1/portal/messages
//   - PUT /api/v1/portal/messages/:id/read
//   - GET /api/v1/portal/messages/unread-count
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedMessage 通过 DB 插入测试消息。
func seedMessage(t *testing.T, ts *apiTestServer, userID int64, title, content, msgType string, isRead bool) int64 {
	t.Helper()
	ts.DB.Exec(`INSERT INTO messages (user_id, title, content, type, related_type, related_id, is_read, created_at)
		VALUES ($1, $2, $3, $4, '', 0, $5, NOW())`, userID, title, content, msgType, isRead)
	var id int64
	ts.DB.Raw("SELECT id FROM messages WHERE title = $1 ORDER BY id DESC LIMIT 1", title).Scan(&id)
	return id
}

func TestAPI_Message_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedMessage(t, ts, ts.AdminID, "test-msg", "hello", "system_notice", false)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?page=1&page_size=10", nil))
	messages := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(messages), 1)
	m := messages[0].(map[string]interface{})
	assert.NotEmpty(t, m["title"])
	assert.NotNil(t, m["is_read"])
	assert.NotNil(t, body["total"], "应含 total")
}

func TestAPI_Message_ListWithType(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedMessage(t, ts, ts.AdminID, "type-msg", "body", "ticket_supplement", false)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?type=ticket_supplement", nil))
	for _, item := range body["data"].([]interface{}) {
		m := item.(map[string]interface{})
		assert.Equal(t, "ticket_supplement", m["type"])
	}
}

func TestAPI_Message_ListWithReadStatus(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedMessage(t, ts, ts.AdminID, "unread-msg", "body", "system_notice", false)
	seedMessage(t, ts, ts.AdminID, "read-msg", "body", "system_notice", true)

	// 仅未读
	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?is_read=false", nil))
	for _, item := range body["data"].([]interface{}) {
		assert.False(t, item.(map[string]interface{})["is_read"].(bool), "is_read=false 应只返回未读")
	}

	// 仅已读
	body2 := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?is_read=true", nil))
	for _, item := range body2["data"].([]interface{}) {
		assert.True(t, item.(map[string]interface{})["is_read"].(bool), "is_read=true 应只返回已读")
	}
}

func TestAPI_Message_MarkRead(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	msgID := seedMessage(t, ts, ts.AdminID, "mark-read", "body", "system_notice", false)

	body := assertOK(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/portal/messages/%d/read", msgID), nil))
	data := body["data"].(map[string]interface{})
	assert.NotNil(t, data["unread_count"], "标记已读应返回未读计数")
}

func TestAPI_Message_MarkReadNonExistent(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPut, "/api/v1/portal/messages/99999/read", nil)
	body := parseBody(t, resp)
	assert.NotEqual(t, float64(0), body["code"], "标记不存在消息应失败")
}

func TestAPI_Message_UnreadCount(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedMessage(t, ts, ts.AdminID, "count-msg1", "body", "system_notice", false)
	seedMessage(t, ts, ts.AdminID, "count-msg2", "body", "system_notice", false)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages/unread-count", nil))
	count := body["data"].(map[string]interface{})["count"]
	t.Logf("未读消息数: %v", count)
}

func TestAPI_Message_OtherUserMessages(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// admin 创建消息给 reporter
	seedMessage(t, ts, ts.ReporterID, "reporter-msg", "for reporter", "system_notice", false)

	// admin 用自己的 token 看不到 reporter 的消息
	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/messages?page=1&page_size=50", nil))
	for _, item := range body["data"].([]interface{}) {
		m := item.(map[string]interface{})
		uid := m["user_id"].(float64)
		assert.Equal(t, float64(ts.AdminID), uid, "用户只能看到自己的消息")
	}
}
