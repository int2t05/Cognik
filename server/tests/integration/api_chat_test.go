//go:build integration

// api_chat_test.go — 智能问答接口集成测试（chat.md 全覆盖）。
//
// 测试端点：
//   - POST   /api/v1/portal/chat-sessions
//   - GET    /api/v1/portal/chat-sessions
//   - GET    /api/v1/portal/chat-sessions/:id
//   - DELETE /api/v1/portal/chat-sessions/:id
//   - POST   /api/v1/portal/chat-sessions/:id/stream
//   - POST   /api/v1/portal/chat-sessions/:id/feedback
package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Create ───────────────────────────────────────────────

func TestAPI_Chat_CreateSession(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "chat-kb")

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": kbID, "title": "VPN trouble",
	}))
	data := body["data"].(map[string]interface{})
	assert.NotZero(t, int64(data["session_id"].(float64)), "session_id 不应为 0")
	assert.NotEmpty(t, data["created_at"], "应有 created_at")
}

func TestAPI_Chat_CreateSessionNoKB(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"title": "no kb",
	}))
}

func TestAPI_Chat_CreateSessionKBNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": 99999, "title": "ghost kb",
	})
	assertNotFound(t, resp)
}

func TestAPI_Chat_CreateSessionDefaultTitle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "default-title-kb")
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": kbID,
	}))
	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["question"], "应自动设置默认标题")
}

// ── List ─────────────────────────────────────────────────

func TestAPI_Chat_ListSessions(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "list-kb")
	ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{"kb_id": kbID, "title": "list test"})

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions?page=1&page_size=10", nil))
	sessions := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(sessions), 1)
	s := sessions[0].(map[string]interface{})
	assert.NotEmpty(t, s["question"], "应含 question 字段")
	assert.NotNil(t, s["created_at"], "应含 created_at")
}

// ── Detail ───────────────────────────────────────────────

func TestAPI_Chat_SessionDetail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "detail-kb"), "title": "detail test",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil))
	data := detail["data"].(map[string]interface{})
	assert.Equal(t, "detail test", data["question"])
	assert.NotNil(t, data["created_at"], "应含 created_at")
}

func TestAPI_Chat_SessionDetailNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions/99999", nil))
}

func TestAPI_Chat_SessionDetailInvalidID(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions/abc", nil))
}

// ── Delete ───────────────────────────────────────────────

func TestAPI_Chat_DeleteSession(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "del-kb"), "title": "delete me",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil), 0)

	// 删除后详情应 404
	assertNotFound(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil))
}

func TestAPI_Chat_DeleteSessionNonOwner(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// admin 创建会话
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "perm-kb"), "title": "admin's session",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	// reporter 尝试删除 → 应拒绝
	assertForbidden(t, ts.doReporter(t, http.MethodDelete, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil))
}

// ── SSE Stream ───────────────────────────────────────────

func TestAPI_Chat_StreamSSE(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "sse-kb"), "title": "SSE test",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	resp, respBody := ts.doSSE(t, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/stream", sessionID),
		map[string]interface{}{"question": "hello?", "route_count": 0, "rerank_count": 0})

	assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"), "Content-Type 应为 text/event-stream")
	assert.NotEmpty(t, respBody, "SSE 响应体不应为空")

	events := parseSSE(t, respBody)
	hasDone := false
	for _, e := range events {
		if e["type"] == "done" || e["type"] == "error" {
			hasDone = true
			break
		}
	}
	assert.True(t, hasDone, "SSE 流应包含 done 或 error 事件")
}

func TestAPI_Chat_StreamInvalidSession(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions/abc/stream",
		map[string]interface{}{"question": "hello?"}))
}

func TestAPI_Chat_StreamMissingQuestion(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "stream-q-kb"), "title": "no question",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/stream", sessionID),
		map[string]interface{}{}))
}

// ── Feedback ─────────────────────────────────────────────

func TestAPI_Chat_Feedback(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "fb-kb"), "title": "feedback test",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	// Like
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 1}), 0)
	// Dislike (覆盖前次)
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 2}), 0)
}

func TestAPI_Chat_FeedbackInvalid(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "fb-inv-kb"), "title": "bad feedback",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 99}))
}

// ── Non-Owner ────────────────────────────────────────────

func TestAPI_Chat_SessionDetailNonOwner(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// admin 创建会话
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "detail-nonowner-kb"), "title": "admin session detail",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	// reporter 查看 → 应拒绝
	assertForbidden(t, ts.doReporter(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil))
}

func TestAPI_Chat_FeedbackNonOwner(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// admin 创建会话
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "fb-nonowner-kb"), "title": "admin session feedback",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	// reporter 提交反馈 → 应拒绝
	assertForbidden(t, ts.doReporter(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 1}))
}

// ── SSE Edge Cases ───────────────────────────────────────

func TestAPI_Chat_StreamSessionNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// valid-format 但不存在 session ID → SSE 应通过 error 事件返回
	resp, respBody := ts.doSSE(t, "/api/v1/portal/chat-sessions/99999/stream",
		map[string]interface{}{"question": "hello?"})

	assert.Equal(t, http.StatusOK, resp.StatusCode, "SSE 应返回 200（即使出错）")

	events := parseSSE(t, respBody)
	require.GreaterOrEqual(t, len(events), 1, "应有至少一个 SSE 事件")
	assert.Equal(t, "error", events[0]["type"], "不存在的会话应返回 error 事件")
}

func TestAPI_Chat_StreamQuestionTooLong(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions", map[string]interface{}{
		"kb_id": ts.seedKB(t, "long-q-kb"), "title": "long question",
	}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	// 2001 字符问题（max=2000）
	longQuestion := strings.Repeat("a", 2001)
	assertBadRequest(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/stream", sessionID),
		map[string]interface{}{"question": longQuestion}))
}

// ── 辅助 ─────────────────────────────────────────────────

func parseSSE(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
			var evt map[string]interface{}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt) == nil {
				events = append(events, evt)
			}
		}
	}
	require.NoError(t, scanner.Err())
	return events
}
