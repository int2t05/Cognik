//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Knowledge_KB_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// Create
	kbID := ts.seedKB(t, "kb-lifecycle")

	// List — 验证响应字段
	kbs := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/knowledge-bases", nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(kbs), 1)
	kb := kbs[0].(map[string]interface{})
	assert.Equal(t, "kb-lifecycle", kb["name"])
	assert.Equal(t, "bge-m3", kb["embedding_model"])
	assert.Equal(t, float64(1024), kb["vector_dimension"])

	// Update
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID),
		map[string]string{"name": "kb-updated", "description": "updated"}), 0)

	// Portal list — 仅 id/name/description
	pkbs := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/knowledge-bases", nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(pkbs), 1)
	assert.NotEmpty(t, pkbs[0].(map[string]interface{})["name"])

	// Delete
	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID), nil), 0)
}

func TestAPI_Knowledge_KB_Validation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assert.Equal(t, http.StatusBadRequest, ts.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases",
		map[string]interface{}{"embedding_model": "bge-m3"}).StatusCode)

	ts.seedKB(t, "dup-kb")
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/knowledge-bases",
		map[string]interface{}{"name": "dup-kb", "embedding_model": "bge-m3", "vector_dimension": 1024}))["code"])
}

func TestAPI_Knowledge_Article_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "article-lifecycle")
	articleID := ts.seedArticle(t, kbID, "VPN FAQ", "content here")

	// List — 验证字段
	articles := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/articles", kbID), nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(articles), 1)
	a := articles[0].(map[string]interface{})
	assert.Equal(t, "VPN FAQ", a["title"])
	assert.Equal(t, float64(1), a["status"]) // 草稿

	// Detail — 验证核心字段
	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/articles/%d", articleID), nil))["data"].(map[string]interface{})
	assert.Equal(t, "VPN FAQ", detail["title"])
	assert.NotEmpty(t, detail["content"])
	assert.Equal(t, float64(1), detail["source_type"])

	// Update
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/articles/%d", articleID),
		map[string]interface{}{"title": "VPN FAQ v2", "content": "updated", "category": "net"}), 0)
	// Submit review
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/submit-review", articleID), nil), 0)

	// Publish — 需要 embedding/pgvector
	body := parseBody(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/publish", articleID), nil))
	if body["code"] == float64(0) {
		assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/disable", articleID), nil), 0)
		assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/enable", articleID), nil), 0)
	}
}

func TestAPI_Knowledge_ReviewValidation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	articleID := ts.seedArticle(t, ts.seedKB(t, "review-kb"), "test", "content")
	// 草稿直接审核 → 拒绝
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/review", articleID),
		map[string]interface{}{"approved": true}))["code"])
}

func TestAPI_Knowledge_DocumentUpload(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "upload-kb")
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/documents/upload", kbID), nil))["code"])
}

func TestAPI_Knowledge_DocumentStatus(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "doc-status-kb")
	// 通过 DB 创建文档记录以测试状态查询端点
	ts.DB.Exec(`INSERT INTO knowledge_articles (kb_id, title, content, source_type, process_status, status, created_by, created_at, updated_at)
		VALUES ($1, 'test.pdf', 'content', 2, 'pending', 1, $2, NOW(), NOW())`, kbID, ts.AdminID)
	var docID int64
	ts.DB.Raw("SELECT id FROM knowledge_articles WHERE title = 'test.pdf'").Scan(&docID)
	assert.NotZero(t, docID)

	resp := assertOK(t, ts.doAuth(t, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/documents/%d/status", kbID, docID), nil))
	status := resp["data"].(map[string]interface{})
	assert.Equal(t, "test.pdf", status["file_name"])
	assert.Equal(t, "pending", status["process_status"])
}

func TestAPI_Knowledge_DocumentRetry(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	kbID := ts.seedKB(t, "doc-retry-kb")
	ts.DB.Exec(`INSERT INTO knowledge_articles (kb_id, title, content, source_type, process_status, status, created_by, created_at, updated_at)
		VALUES ($1, 'failed.pdf', 'content', 2, 'failed', 1, $2, NOW(), NOW())`, kbID, ts.AdminID)
	var docID int64
	ts.DB.Raw("SELECT id FROM knowledge_articles WHERE title = 'failed.pdf'").Scan(&docID)

	// retry 需要 processor 初始化 — 测试环境可能不可用
	body := parseBody(t, ts.doAuth(t, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/documents/%d/retry", kbID, docID), nil))
	if body["code"] == float64(0) {
		// 成功后状态变为 pending，再次重试应被拒绝
		assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost,
			fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/documents/%d/retry", kbID, docID), nil))["code"])
	}
}

func TestAPI_Knowledge_NotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/knowledge-bases/99999",
		map[string]string{"name": "x"}))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/articles/99999", nil))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/knowledge-bases/1/documents/99999/status", nil))["code"])
}
