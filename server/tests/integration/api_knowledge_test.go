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

	kbID := ts.seedKB(t, "kb-lifecycle")

	assert.GreaterOrEqual(t, len(assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/knowledge-bases", nil))["data"].([]interface{})), 1)
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d", kbID),
		map[string]string{"name": "kb-updated", "description": "updated"}), 0)
	assert.GreaterOrEqual(t, len(assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/portal/knowledge-bases", nil))["data"].([]interface{})), 1)
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

	articles := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/knowledge-bases/%d/articles", kbID), nil))["data"].([]interface{})
	assert.GreaterOrEqual(t, len(articles), 1)

	detail := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/articles/%d", articleID), nil))["data"].(map[string]interface{})
	assert.Equal(t, "VPN FAQ", detail["title"])

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/articles/%d", articleID),
		map[string]interface{}{"title": "VPN FAQ v2", "content": "updated", "category": "net"}), 0)
	assertCode(t, ts.doAuth(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/articles/%d/submit-review", articleID), nil), 0)
	// 审核需要不同于创建人的用户 — 在 ReviewValidation 测试中单独验证

	// 发布需要 embedding/pgvector，测试环境可能不可用
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

func TestAPI_Knowledge_NotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/knowledge-bases/99999",
		map[string]string{"name": "x"}))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/articles/99999", nil))["code"])
}
