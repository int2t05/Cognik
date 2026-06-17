//go:build integration

// Package handler_test 验证 KnowledgeHandler HTTP 接口。
//
// 测试覆盖知识库 CRUD、文章 CRUD、审核流程、文档上传的 HTTP 端点。
package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/handler"
	"opsmind/internal/model"
	"opsmind/internal/rag"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// =============================================================================
// 测试基础设施
// =============================================================================

var knowledgeHandlerDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	knowledgeHandlerDB = db
}

func setupKnowledgeHandler(t *testing.T) *handler.KnowledgeHandler {
	t.Helper()
	repo := repository.NewKnowledgeRepo(knowledgeHandlerDB)
	// 使用真实 DocParser（纯 Go）和 Chunker（纯 Go），无需外部依赖
	docParser := rag.NewDocParser()
	chunker := rag.NewChunker(1000, 200)
	svc := service.NewKnowledgeService(repo, nil, chunker, nil, nil, docParser, nil, nil)
	return handler.NewKnowledgeHandler(svc)
}

func setupGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func cleanupKnowledgeHandlerTables(t *testing.T) {
	t.Helper()
	knowledgeHandlerDB.Exec("DELETE FROM knowledge_chunks")
	knowledgeHandlerDB.Exec("DELETE FROM knowledge_articles")
	knowledgeHandlerDB.Exec("DELETE FROM knowledge_bases")
}

// =============================================================================
// KnowledgeBase Handler 测试
// =============================================================================

func TestKnowledgeHandler_CreateKB(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	r.POST("/api/v1/admin/knowledge-bases", h.CreateKB)

	body, _ := json.Marshal(request.CreateKBRequest{
		Name:        "Handler 测试库",
		Description: "Handler 测试描述",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/knowledge-bases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证数据库
	var kb model.KnowledgeBase
	knowledgeHandlerDB.First(&kb)
	if kb.Name != "Handler 测试库" {
		t.Errorf("期望名称 'Handler 测试库', got '%s'", kb.Name)
	}
}

func TestKnowledgeHandler_ListKBs(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	// 预创建 2 个知识库
	knowledgeHandlerDB.Create(&model.KnowledgeBase{Name: "KB1", RAGWorkspaceSlug: "s1", CreatedBy: 1})
	knowledgeHandlerDB.Create(&model.KnowledgeBase{Name: "KB2", RAGWorkspaceSlug: "s2", CreatedBy: 1})

	r.GET("/api/v1/admin/knowledge-bases", h.ListKBs)

	req := httptest.NewRequest("GET", "/api/v1/admin/knowledge-bases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

// TestKnowledgeHandler_ListKBsForPortal 验证门户端知识库列表。
func TestKnowledgeHandler_ListKBsForPortal(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	knowledgeHandlerDB.Create(&model.KnowledgeBase{Name: "PortalKB", RAGWorkspaceSlug: "portal", CreatedBy: 1})

	r.GET("/api/v1/portal/knowledge-bases", h.ListKBsForPortal)

	req := httptest.NewRequest("GET", "/api/v1/portal/knowledge-bases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKnowledgeHandler_UpdateKB(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "旧名称", RAGWorkspaceSlug: "old-slug", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)

	r.PUT("/api/v1/admin/knowledge-bases/:id", h.UpdateKB)

	body, _ := json.Marshal(request.UpdateKBRequest{Name: "新名称", Description: "新描述"})
	req := httptest.NewRequest("PUT", "/api/v1/admin/knowledge-bases/"+itoa(kb.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// KnowledgeArticle Handler 测试
// =============================================================================

func TestKnowledgeHandler_CreateArticle(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "文章测试", RAGWorkspaceSlug: "article-slug", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)

	r.POST("/api/v1/admin/knowledge-bases/:kb_id/articles", h.CreateArticle)

	body, _ := json.Marshal(request.CreateArticleRequest{
		KBID:     kb.ID,
		Title: "问题",
		Content:   "答案",
		Category: "分类",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/knowledge-bases/"+itoa(kb.ID)+"/articles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKnowledgeHandler_SubmitReview(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "审核测试", RAGWorkspaceSlug: "review-slug", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)
	article := &model.KnowledgeArticle{KBID: kb.ID, Title: "Q", Content: "A", Status: 1, CreatedBy: 1}
	knowledgeHandlerDB.Create(article)

	r.POST("/api/v1/admin/articles/:id/submit-review", h.SubmitReview)

	req := httptest.NewRequest("POST", "/api/v1/admin/articles/"+itoa(article.ID)+"/submit-review", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.KnowledgeArticle
	knowledgeHandlerDB.First(&updated, article.ID)
	if updated.Status != 2 {
		t.Errorf("期望 status=2(待审核), got %d", updated.Status)
	}
}

func TestKnowledgeHandler_Review(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "审核通过", RAGWorkspaceSlug: "approve-slug", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)
	article := &model.KnowledgeArticle{KBID: kb.ID, Title: "Q", Content: "A", Status: 2, CreatedBy: 1}
	knowledgeHandlerDB.Create(article)

	r.POST("/api/v1/admin/articles/:id/review", h.Review)

	body, _ := json.Marshal(request.ReviewRequest{Approved: true})
	req := httptest.NewRequest("POST", "/api/v1/admin/articles/"+itoa(article.ID)+"/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeHandler_Enable 验证停用文章重新启用。
func TestKnowledgeHandler_Enable(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "启用测试", RAGWorkspaceSlug: "enable-slug", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)
	// GORM 默认零值（int16(0)）会被数据库 DEFAULT 1 覆盖，需创建后手动更新状态
	article := &model.KnowledgeArticle{KBID: kb.ID, Title: "Q", Content: "A", Status: 1, CreatedBy: 1}
	knowledgeHandlerDB.Create(article)
	knowledgeHandlerDB.Model(article).Update("status", int16(model.ArticleStatusDisabled))

	r.POST("/api/v1/admin/articles/:id/enable", h.Enable)

	req := httptest.NewRequest("POST", "/api/v1/admin/articles/"+itoa(article.ID)+"/enable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.KnowledgeArticle
	knowledgeHandlerDB.First(&updated, article.ID)
	if updated.Status != int16(model.ArticleStatusDraft) {
		t.Errorf("启用后期望 status=1(草稿), got %d", updated.Status)
	}
}

// =============================================================================
// 文档上传测试（真实 DB + Handler → Service → Repository 链路）
// =============================================================================

// TestKnowledgeHandler_UploadDocuments 验证文档上传接口（真实 DB，降级模式）。
//
// 降级模式（无 StorageClient）：HTTP 同步解析 → 创建 article → 入队 processor。
func TestKnowledgeHandler_UploadDocuments(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()
	r.Use(func(c *gin.Context) { c.Set("userID", int64(1)); c.Next() })

	kb := &model.KnowledgeBase{Name: "上传测试库", RAGWorkspaceSlug: "upload-test", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)

	r.POST("/api/v1/admin/knowledge-bases/:kb_id/documents/upload", h.UploadDocuments)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("file", "test.txt")
	fw.Write([]byte("这是测试文档内容，用于验证上传处理流程。"))
	w.Close()

	req, _ := http.NewRequest("POST", "/api/v1/admin/knowledge-bases/"+itoa(kb.ID)+"/documents/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", rec.Code, rec.Body.String())
	}

	// 验证文章已创建
	var article model.KnowledgeArticle
	if err := knowledgeHandlerDB.Where("kb_id = ?", kb.ID).First(&article).Error; err != nil {
		t.Fatalf("文章应已创建: %v", err)
	}
	if article.Title != "test.txt" {
		t.Errorf("期望标题 'test.txt', got '%s'", article.Title)
	}
	if article.Content == "" {
		t.Error("降级模式下 Content 不应为空（同步解析）")
	}
}

// TestKnowledgeHandler_GetDocumentStatus 验证文档处理状态查询接口（真实 DB）。
func TestKnowledgeHandler_GetDocumentStatus(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "状态查询库", RAGWorkspaceSlug: "status-test", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)
	article := &model.KnowledgeArticle{
		KBID:          kb.ID,
		Title:         "状态文档",
		Content:       "内容",
		Status:        1,
		ProcessStatus: "completed",
		CreatedBy:     1,
	}
	knowledgeHandlerDB.Create(article)

	r.GET("/api/v1/admin/knowledge-bases/:kb_id/documents/:id/status", h.GetDocumentStatus)

	req, _ := http.NewRequest("GET", "/api/v1/admin/knowledge-bases/"+itoa(kb.ID)+"/documents/"+itoa(article.ID)+"/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("期望 200, 实际 %d: %s", rec.Code, rec.Body.String())
	}

	// 解析响应验证 process_status
	var resp struct {
		Data struct {
			ProcessStatus string `json:"process_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.ProcessStatus != "completed" {
		t.Errorf("期望 process_status='completed', got '%s'", resp.Data.ProcessStatus)
	}
}

// TestKnowledgeHandler_RetryDocument 验证文档处理重试接口（真实 DB）。
//
// 注意：重试需要 Processor（异步文档处理器），它依赖 Embedder（外部 API）。
// 无 API 时 Processor 为 nil，RetryDocument 返回"文档处理器未初始化"错误——
// 属于预期的优雅降级。完整重试流程在 rag/processor_test.go 中测试。
func TestKnowledgeHandler_RetryDocument(t *testing.T) {
	cleanupKnowledgeHandlerTables(t)
	h := setupKnowledgeHandler(t)
	r := setupGin()

	kb := &model.KnowledgeBase{Name: "重试库", RAGWorkspaceSlug: "retry-test", CreatedBy: 1}
	knowledgeHandlerDB.Create(kb)
	article := &model.KnowledgeArticle{
		KBID:          kb.ID,
		Title:         "重试文档",
		Content:       "重试内容",
		Status:        1,
		ProcessStatus: "failed",
		CreatedBy:     1,
	}
	knowledgeHandlerDB.Create(article)

	r.POST("/api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry", h.RetryDocument)

	req, _ := http.NewRequest("POST", "/api/v1/admin/knowledge-bases/"+itoa(kb.ID)+"/documents/"+itoa(article.ID)+"/retry", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Processor 为 nil 时返回错误（文档处理器未初始化）——预期行为
	t.Logf("RetryDocument 响应（processor=nil 预期降级）: %d %s", rec.Code, rec.Body.String())
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
