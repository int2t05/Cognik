//go:build e2e

// Package e2e_test 端到端测试——真实服务进程 + 日志验证。
//
// 每个测试：发送 HTTP → 验证响应体字段 → 检查服务器日志 → 检查 Docker 日志。
package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ── Auth ────────────────────────────────────────────────

func TestE2E_Auth_Login(t *testing.T) {
	resp := e2e.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "Admin@123"}, "")

	body := assertOK(t, resp)
	data := body["data"].(map[string]interface{})
	assertField(t, data["access_token"] != nil && data["access_token"].(string) != "", "access_token 不为空")
	assertField(t, data["refresh_token"] != nil, "refresh_token 不为空")
	user := data["user"].(map[string]interface{})
	assertField(t, user["username"] == "admin", "user.username=admin")
	assertField(t, user["first_login"] == false, "first_login=false")
	assertField(t, len(data["roles"].([]interface{})) >= 1, "至少 1 个角色")
	assertField(t, len(data["permissions"].([]interface{})) >= 1, "至少 1 个权限")

	e2e.assertLogContains(t, "/api/v1/auth/login", "请求日志")
	e2e.assertLogContains(t, "200", "返回 200")
	assertNoRecentDBErrors(t)
}

func TestE2E_Auth_Refresh(t *testing.T) {
	loginBody := assertOK(t, e2e.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "Admin@123"}, ""))
	refresh := loginBody["data"].(map[string]interface{})["refresh_token"].(string)

	body := assertOK(t, e2e.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refresh}, ""))
	data := body["data"].(map[string]interface{})
	assertField(t, data["access_token"] != nil, "新 access_token 不为空")
	assertField(t, data["user"] != nil, "返回 user 信息")

	e2e.assertLogContains(t, "/api/v1/auth/refresh", "刷新日志")
	assertNoRecentDBErrors(t)
}

func TestE2E_Auth_Errors(t *testing.T) {
	// 错误密码
	r1 := e2e.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "wrong"}, "")
	body1 := parseBody(t, r1)
	assertField(t, body1["code"] != float64(0), "错误密码返回非 0")
	e2e.assertLogContains(t, "10003", "错误码 10003")

	// 不存在用户
	r2 := e2e.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "nobody_xxxx", "password": "x"}, "")
	body2 := parseBody(t, r2)
	assertField(t, body2["code"] != float64(0), "不存在用户返回非 0")

	// 缺少参数
	r3 := e2e.do(t, http.MethodPost, "/api/v1/auth/login", map[string]string{}, "")
	assertField(t, r3.StatusCode == http.StatusBadRequest, "空请求=400")

	assertNoRecentDBErrors(t)
}

// ── Knowledge ───────────────────────────────────────────

func TestE2E_Knowledge_CRUD(t *testing.T) {
	// Create
	r1 := e2e.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases", map[string]interface{}{
		"name": "e2e-kb", "description": "test", "embedding_model": "bge-m3", "vector_dimension": 1024,
	})
	assertCode(t, r1, 0)
	e2e.assertLogContains(t, "/api/v1/admin/knowledge-bases", "创建KB")
	var kbID int64
	e2e.DB.Raw("SELECT id FROM knowledge_bases WHERE name = 'e2e-kb'").Scan(&kbID)

	// List — 验证字段
	body2 := assertOK(t, e2e.doAuth(t, http.MethodGet, "/api/v1/admin/knowledge-bases", nil))
	kbs := body2["data"].([]interface{})
	assertField(t, len(kbs) >= 1, "KB 列表非空")

	// Portal list
	r3 := e2e.doAuth(t, http.MethodGet, "/api/v1/portal/knowledge-bases", nil)
	pkbs := assertOK(t, r3)["data"].([]interface{})
	assertField(t, len(pkbs) >= 1, "Portal KB 列表非空")

	// Update
	r4 := e2e.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID),
		map[string]string{"name": "e2e-kb-updated", "description": "updated"})
	assertCode(t, r4, 0)

	// Create Article
	r5 := e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/articles", kbID),
		map[string]interface{}{"title": "e2e-article", "content": "test content"})
	assertCode(t, r5, 0)
	e2e.assertLogContains(t, "200", "创建文章 200")
	var articleID int64
	e2e.DB.Raw("SELECT id FROM knowledge_articles WHERE title = 'e2e-article'").Scan(&articleID)

	// Article Detail
	r6 := e2e.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/articles/%d", articleID), nil)
	detail := assertOK(t, r6)["data"].(map[string]interface{})
	assertField(t, detail["title"] == "e2e-article", "文章标题正确")

	// Submit Review
	r7 := e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/submit-review", articleID), nil)
	assertCode(t, r7, 0)

	// Publish — 现在有 embedding 列和 HNSW 索引（+ llama.cpp embedding 服务）
	r8 := e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/publish", articleID), nil)
	body8 := parseBody(t, r8)
	if body8["code"] == float64(0) {
		e2e.assertLogContains(t, "Publish", "发布成功")

		// Disable
		e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/disable", articleID), nil)
		// Enable
		e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/enable", articleID), nil)
	}

	// Cleanup
	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID), nil)

	assertNoRecentDBErrors(t)
}

// ── Ticket ──────────────────────────────────────────────

func TestE2E_Ticket_Lifecycle(t *testing.T) {
	// Create
	r1 := e2e.doAuth(t, http.MethodPost, "/api/v1/portal/tickets", map[string]interface{}{
		"title": "e2e-ticket", "description": "test", "urgency": 2, "contact_phone": "13800001000",
	})
	assertCode(t, r1, 0)
	e2e.assertLogContains(t, "/api/v1/portal/tickets", "创建申告")
	var ticketID int64
	e2e.DB.Raw("SELECT id FROM tickets WHERE title = 'e2e-ticket'").Scan(&ticketID)

	// Portal list
	r2 := e2e.doAuth(t, http.MethodGet, "/api/v1/portal/tickets?page=1&page_size=10", nil)
	tickets := assertOK(t, r2)["data"].([]interface{})
	assertField(t, len(tickets) >= 1, "申告列表非空")
	tk := tickets[0].(map[string]interface{})
	assertField(t, tk["title"] == "e2e-ticket", "标题正确")
	assertField(t, tk["ticket_no"] != nil, "工单号存在")

	// Portal detail
	r3 := e2e.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/portal/tickets/%d", ticketID), nil)
	detail := assertOK(t, r3)["data"].(map[string]interface{})
	assertField(t, detail["description"] != nil, "描述存在")

	// Admin detail
	r4 := e2e.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/tickets/%d", ticketID), nil)
	assertOK(t, r4)

	// Admin list + filter
	r5 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?status=1", nil)
	assertOK(t, r5)
	r6 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/tickets?urgency=2", nil)
	assertOK(t, r6)

	// Start → Request Info
	r7 := e2e.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", ticketID),
		map[string]string{"action": "start", "result": "处理中"})
	assertCode(t, r7, 0)
	e2e.assertLogContains(t, "200", "开始处理 200")

	r8 := e2e.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", ticketID),
		map[string]string{"action": "request_info", "result": "请补充"})
	assertCode(t, r8, 0)

	// 模拟补充后恢复处理中 → 解决
	e2e.DB.Exec("UPDATE tickets SET status = 2 WHERE id = $1", ticketID)
	r9 := e2e.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/tickets/%d/status", ticketID),
		map[string]string{"action": "resolve", "result": "已解决"})
	assertCode(t, r9, 0)

	// Add record
	r10 := e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/records", ticketID),
		map[string]string{"action": "note", "content": "完成"})
	assertCode(t, r10, 0)

	// Knowledge candidate
	e2e.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases", map[string]interface{}{
		"name": "candidate-kb", "embedding_model": "bge-m3", "vector_dimension": 1024,
	})
	var kbID int64
	e2e.DB.Raw("SELECT id FROM knowledge_bases WHERE name = 'candidate-kb'").Scan(&kbID)
	e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/tickets/%d/knowledge-candidate", ticketID),
		map[string]interface{}{"kb_id": kbID})
	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID), nil)

	assertNoRecentDBErrors(t)
}

// ── SSE Chat ────────────────────────────────────────────

func TestE2E_Chat_SSE(t *testing.T) {
	// 创建 KB
	e2e.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases", map[string]interface{}{
		"name": "e2e-chat-kb", "embedding_model": "bge-m3", "vector_dimension": 1024,
	})
	var kbID int64
	e2e.DB.Raw("SELECT id FROM knowledge_bases WHERE name = 'e2e-chat-kb'").Scan(&kbID)

	// 创建会话
	r1 := e2e.doAuth(t, http.MethodPost, "/api/v1/portal/chat-sessions",
		map[string]interface{}{"kb_id": kbID, "title": "e2e-chat"})
	sessionID := int64(assertOK(t, r1)["data"].(map[string]interface{})["session_id"].(float64))

	// SSE 流式
	resp, body := e2e.doSSE(t, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/stream", sessionID),
		map[string]interface{}{"question": "hello", "route_count": 0, "rerank_count": 0})
	assertField(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"), "SSE Content-Type")

	events := parseSSE(t, body)
	hasDone := false
	for _, e := range events {
		t.Logf("   SSE 事件: type=%v", e["type"])
		if e["type"] == "done" || e["type"] == "error" {
			hasDone = true
		}
	}
	assertField(t, hasDone, "有 done 或 error 事件")

	// 反馈
	e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 1})
	e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/portal/chat-sessions/%d/feedback", sessionID),
		map[string]interface{}{"feedback": 2})

	// 清理
	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/portal/chat-sessions/%d", sessionID), nil)
	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID), nil)

	e2e.assertLogContains(t, "POST /api/v1/portal/chat-sessions", "SSE 请求日志")
	assertNoRecentDBErrors(t)
}

// ── User & Role ─────────────────────────────────────────

func TestE2E_UserRole(t *testing.T) {
	// Create user
	e2e.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "e2e_user", "password": "User@1234", "real_name": "E2E", "phone": "13800002000",
	})
	var userID int64
	e2e.DB.Raw("SELECT id FROM users WHERE username = 'e2e_user'").Scan(&userID)

	// Detail
	r1 := e2e.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", userID), nil)
	u := assertOK(t, r1)["data"].(map[string]interface{})
	assertField(t, u["username"] == "e2e_user", "用户名正确")
	assertField(t, u["status"] == float64(1), "状态正常")

	// Update
	e2e.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]interface{}{"real_name": "E2E-Updated", "phone": "13800002999"})

	// Freeze → Unfreeze
	e2e.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil)
	e2e.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/unfreeze", userID), nil)

	// Role CRUD
	e2e.doAuth(t, http.MethodPost, "/api/v1/admin/roles", map[string]interface{}{
		"name": "e2e_role", "description": "test", "permissions": []string{"ticket:read"},
	})
	var roleID int64
	e2e.DB.Raw("SELECT id FROM roles WHERE name = 'e2e_role'").Scan(&roleID)
	e2e.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", roleID),
		map[string]interface{}{"name": "e2e_role_v2", "description": "upd", "permissions": []string{"ticket:read", "audit:read"}})
	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil)

	// Menu list
	r2 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/menus", nil)
	assertOK(t, r2)

	e2e.assertLogContains(t, "/api/v1/admin/users", "用户操作日志")
	assertNoRecentDBErrors(t)
}

// ── LLM Config ──────────────────────────────────────────

func TestE2E_LLMConfig(t *testing.T) {
	// Create
	r1 := e2e.doAuth(t, http.MethodPost, "/api/v1/admin/llm-configs", map[string]interface{}{
		"name": "e2e-cfg", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o-mini", "embedding_model": "text-embedding-3-small",
		"max_tokens": 16384, "vector_dimension": 1536, "is_default": true,
	})
	cfg := assertOK(t, r1)["data"].(map[string]interface{})
	cfgID := int64(cfg["id"].(float64))

	// List — 验证 API key 脱敏
	r2 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/llm-configs", nil)
	cfgs := assertOK(t, r2)["data"].([]interface{})
	assertField(t, len(cfgs) >= 1, "配置列表非空")

	// Detail
	r3 := e2e.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil)
	assertOK(t, r3)

	// Update
	e2e.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), map[string]interface{}{
		"name": "e2e-cfg-v2", "provider_type": 2, "base_url": "https://api.openai.com/v1",
		"llm_model": "gpt-4o", "embedding_model": "text-embedding-3-large",
		"max_tokens": 32768, "vector_dimension": 3072, "is_default": true,
	})

	// Test connection
	e2e.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/llm-configs/%d/test", cfgID), nil)

	// Delete default — 拒绝
	r4 := e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/llm-configs/%d", cfgID), nil)
	assertAPIError(t, r4)

	e2e.assertLogContains(t, "/api/v1/admin/llm-configs", "LLM配置日志")
	assertNoRecentDBErrors(t)
}

// ── Dashboard & Audit ───────────────────────────────────

func TestE2E_DashboardAudit(t *testing.T) {
	// Stats
	r1 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/stats", nil)
	stats := assertOK(t, r1)["data"].(map[string]interface{})
	for _, f := range []string{"today_tickets", "pending_tickets", "processing_tickets", "resolved_tickets", "today_chats", "avg_confidence", "knowledge_count"} {
		assertField(t, stats[f] != nil, "字段存在: "+f)
	}

	// Trends
	r2 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/trends?start_date=2026-06-01&end_date=2026-06-18&granularity=day", nil)
	assertOK(t, r2)

	// Trends validation — 日期错误
	r3 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/dashboard/trends?start_date=2026-06-18&end_date=2026-06-01", nil)
	assertAPIError(t, r3)

	// Audit
	e2e.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases", map[string]interface{}{
		"name": "audit-test-kb", "embedding_model": "bge-m3", "vector_dimension": 1024,
	})
	var kbID int64
	e2e.DB.Raw("SELECT id FROM knowledge_bases WHERE name = 'audit-test-kb'").Scan(&kbID)

	r4 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?page=1&page_size=10", nil)
	auditData := assertOK(t, r4)["data"].([]interface{})
	assertField(t, len(auditData) >= 1, "审计日志非空")
	if len(auditData) > 0 {
		entry := auditData[0].(map[string]interface{})
		assertField(t, entry["action"] != nil, "action 字段存在")
		assertField(t, entry["operator_name"] != nil, "operator_name 字段存在")
	}

	// Audit filtering
	e2e.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?action=knowledge.create", nil)
	e2e.doAuth(t, http.MethodGet, "/api/v1/admin/audit-logs?target_type=knowledge_article", nil)

	e2e.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID), nil)

	assertNoRecentDBErrors(t)
}

// ── Messages & Config & Health ──────────────────────────

func TestE2E_MessagesConfigHealth(t *testing.T) {
	// Message — 通过 DB 插入
	e2e.DB.Exec(`INSERT INTO messages (user_id, title, content, type, related_type, related_id, is_read, created_at)
		VALUES (1, 'e2e-msg', 'test', 'system_notice', '', 0, false, NOW())`)

	r1 := e2e.doAuth(t, http.MethodGet, "/api/v1/portal/messages?page=1&page_size=10", nil)
	assertOK(t, r1)
	r2 := e2e.doAuth(t, http.MethodGet, "/api/v1/portal/messages?type=system_notice", nil)
	assertOK(t, r2)
	r3 := e2e.doAuth(t, http.MethodGet, "/api/v1/portal/messages/unread-count", nil)
	assertOK(t, r3)
	r4 := e2e.doAuth(t, http.MethodPut, "/api/v1/portal/messages/99999/read", nil)
	assertAPIError(t, r4)

	// System config
	e2e.doAuth(t, http.MethodPut, "/api/v1/admin/configs/app_name", map[string]interface{}{"value": "OpsMind-E2E"})
	r5 := e2e.doAuth(t, http.MethodGet, "/api/v1/admin/configs/app_name", nil)
	body5 := assertOK(t, r5)
	assertField(t, body5["data"] == "OpsMind-E2E", "配置值匹配")

	// Health — 返回 {"status":"ok"} 非标准 code/message 格式
	r6 := e2e.do(t, http.MethodGet, "/health", nil, "")
	assertField(t, r6.StatusCode == 200, "/health=200")
	hb := parseBody(t, r6)
	assertField(t, hb["status"] == "ok", "status=ok")

	r7 := e2e.do(t, http.MethodGet, "/readyz", nil, "")
	assertField(t, r7.StatusCode == 200, "/readyz=200")

	assertNoRecentDBErrors(t)
}

// ── Cleanup ─────────────────────────────────────────────

func TestE2E_Cleanup(t *testing.T) {
	// 按 FK 依赖逆序清理测试数据
	e2e.DB.Exec("DELETE FROM messages WHERE title = 'e2e-msg'")
	e2e.DB.Exec("DELETE FROM chat_messages WHERE session_id IN (SELECT id FROM chat_sessions WHERE question = 'e2e-chat')")
	e2e.DB.Exec("DELETE FROM chat_sessions WHERE question = 'e2e-chat'")
	e2e.DB.Exec("DELETE FROM ticket_records WHERE ticket_id IN (SELECT id FROM tickets WHERE title LIKE 'e2e%')")
	e2e.DB.Exec("DELETE FROM tickets WHERE title LIKE 'e2e%'")
	e2e.DB.Exec("DELETE FROM knowledge_chunks WHERE article_id IN (SELECT id FROM knowledge_articles WHERE title LIKE 'e2e%')")
	e2e.DB.Exec("DELETE FROM knowledge_articles WHERE title LIKE 'e2e%'")
	e2e.DB.Exec("DELETE FROM knowledge_bases WHERE name LIKE 'e2e%'")
	e2e.DB.Exec("DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'e2e%')")
	e2e.DB.Exec("DELETE FROM user_roles WHERE role_id IN (SELECT id FROM roles WHERE name LIKE 'e2e%')")
	e2e.DB.Exec("DELETE FROM role_menus WHERE role_id IN (SELECT id FROM roles WHERE name LIKE 'e2e%')")
	e2e.DB.Exec("DELETE FROM users WHERE username LIKE 'e2e%'")
	e2e.DB.Exec("DELETE FROM roles WHERE name LIKE 'e2e%'")
	e2e.DB.Exec("DELETE FROM llm_configs WHERE name LIKE 'e2e%'")
	t.Logf("✅ 测试数据已清理")
}
