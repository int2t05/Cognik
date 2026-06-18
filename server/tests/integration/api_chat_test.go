//go:build integration

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

func TestAPI_Chat_SessionLifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "chat-kb")
	body := assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions",
		map[string]interface{}{"kb_id": kbID, "title": "VPN issue"}))
	sessionID := int64(body["data"].(map[string]interface{})["session_id"].(float64))

	assert.GreaterOrEqual(t, len(assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions?page=1&page_size=10", nil))["data"].([]interface{})), 1)
	assert.Equal(t, "VPN issue", assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil))["data"].(map[string]interface{})["question"])
	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil), 0)
}

func TestAPI_Chat_StreamSSE(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	sessionID := int64(assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions",
		map[string]interface{}{"kb_id": ts.seedKB(t, "sse-kb"), "title": "SSE"}))["data"].(map[string]interface{})["session_id"].(float64))

	resp, body := ts.doSSE(t, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/stream", sessionID),
		map[string]interface{}{"question": "test?", "route_count": 0, "rerank_count": 0})

	assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"))
	assert.NotEmpty(t, body)

	// 解析 SSE，验证有 done 或 error 事件
	events := parseSSE(t, body)
	hasDone := false
	for _, e := range events {
		if e["type"] == "done" || e["type"] == "error" {
			hasDone = true
			break
		}
	}
	assert.True(t, hasDone, "应有 done 或 error 事件")
}

func TestAPI_Chat_Feedback(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	sessionID := int64(assertOK(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions",
		map[string]interface{}{"kb_id": ts.seedKB(t, "feedback-kb"), "title": "fb"}))["data"].(map[string]interface{})["session_id"].(float64))

	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 1}), 0)
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 2}), 0)
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 99}))["code"])
}

func TestAPI_Chat_Validation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions",
		map[string]interface{}{"title": "no-kb"}))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions/99999", nil))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/chat-sessions/abc", nil))["code"])
}

func parseSSE(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var evt map[string]interface{}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt) == nil {
				events = append(events, evt)
			}
		}
	}
	require.NoError(t, scanner.Err())
	return events
}
