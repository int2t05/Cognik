// Package handler 实现 HTTP 请求处理。
//
// knowledge.go 提供知识库管理相关接口（KB/文章/审核/发布/文档上传）。
package handler

import (
	"path/filepath"
	"strconv"
	"strings"

	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// KnowledgeHandler 知识库管理接口。
type KnowledgeHandler struct {
	svc *service.KnowledgeService
}

// NewKnowledgeHandler 创建 KnowledgeHandler 实例。
func NewKnowledgeHandler(svc *service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{svc: svc}
}

// =============================================================================
// KnowledgeBase
// =============================================================================

// ListKBsForPortal 门户端知识库列表（无需 admin 权限，供 Chat 页选择知识库下拉框使用）。
//
// GET /api/v1/portal/knowledge-bases
func (h *KnowledgeHandler) ListKBsForPortal(c *gin.Context) {
	kbs, err := h.svc.ListKBs()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	// 门户端仅返回 id/name/description，不暴露 embedding_model/vector_dimension 等管理字段
	type portalKB struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	result := make([]portalKB, len(kbs))
	for i, kb := range kbs {
		result[i] = portalKB{ID: kb.ID, Name: kb.Name, Description: kb.Description}
	}
	response.Success(c, result)
}

// CreateKB 创建知识库。
//
// POST /api/v1/admin/knowledge-bases
func (h *KnowledgeHandler) CreateKB(c *gin.Context) {
	var req request.CreateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.CreateKB(req, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// UpdateKB 更新知识库。
//
// PUT /api/v1/admin/knowledge-bases/:id
func (h *KnowledgeHandler) UpdateKB(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.UpdateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if svcErr := h.svc.UpdateKB(id, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// DeleteKB 删除知识库。
//
// DELETE /api/v1/admin/knowledge-bases/:id
func (h *KnowledgeHandler) DeleteKB(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if svcErr := h.svc.DeleteKB(id); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// ListKBs 列出全部知识库。
//
// GET /api/v1/admin/knowledge-bases
func (h *KnowledgeHandler) ListKBs(c *gin.Context) {
	kbs, err := h.svc.ListKBs()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, kbs)
}

// =============================================================================
// KnowledgeArticle
// =============================================================================

// CreateArticle 创建知识文章。
//
// POST /api/v1/admin/knowledge-bases/:kb_id/articles
func (h *KnowledgeHandler) CreateArticle(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}

	var req request.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	req.KBID = kbID

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.CreateArticle(req, userID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// UpdateArticle 更新知识文章。
//
// PUT /api/v1/admin/articles/:id
func (h *KnowledgeHandler) UpdateArticle(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.UpdateArticle(id, req, userID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// SubmitReview 提交审核。
//
// POST /api/v1/admin/articles/:id/submit-review
func (h *KnowledgeHandler) SubmitReview(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.SubmitReview(id, userID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// Review 审核文章。
//
// POST /api/v1/admin/articles/:id/review
func (h *KnowledgeHandler) Review(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.Review(id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// Publish 发布文章（分块→embedding→pgvector 写入）。
//
// POST /api/v1/admin/articles/:id/publish
func (h *KnowledgeHandler) Publish(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.Publish(c.Request.Context(), id, userID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// Disable 停用文章（从 pgvector 删除向量）。
//
// POST /api/v1/admin/articles/:id/disable
func (h *KnowledgeHandler) Disable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if svcErr := h.svc.Disable(c.Request.Context(), id); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// Enable 启用已停用文章——重新执行分块→embedding→pgvector 写入并发布。
//
// POST /api/v1/admin/articles/:id/enable
func (h *KnowledgeHandler) Enable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.Enable(c.Request.Context(), id, userID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// RetrySync 重试文档处理。
//
// POST /api/v1/admin/articles/:id/retry-sync
func (h *KnowledgeHandler) RetrySync(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if svcErr := h.svc.RetryDocument(id); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// ListArticles 分页查询文章列表。
//
// GET /api/v1/admin/knowledge-bases/:kb_id/articles
func (h *KnowledgeHandler) ListArticles(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}

	page, pageSize := parsePagination(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))

	result, svcErr := h.svc.ListArticles(kbID, status, page, pageSize)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.SuccessWithPage(c, result.Articles, result.Total, page, pageSize)
}

// GetArticleDetail 获取文章详情。
//
// GET /api/v1/admin/articles/:id
func (h *KnowledgeHandler) GetArticleDetail(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	result, svcErr := h.svc.GetArticleDetail(id)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, result)
}

// =============================================================================
// 文档上传/状态/重试
// =============================================================================

// UploadDocuments 上传文档到知识库（multipart form）。
//
// POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload
func (h *KnowledgeHandler) UploadDocuments(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}

	// TODO(handler/knowledge): 应支持 multipart 字段名 files 多文件上传，当前仅处理单个 file。
	// 应改为 MultipartForm.File["files"] 循环处理，并返回 documents 数组。
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, errcode.ErrParam, "文件上传失败: "+err.Error())
		return
	}

	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	// TODO(handler/knowledge): 不能只信任扩展名，应结合 MIME sniffing 和解析器校验。
	// 否则恶意文件改扩展名后会进入 PDF/DOCX 解析路径。

	src, err := file.Open()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	defer src.Close()

	userID, _ := getCurrentUserID(c)

	// 文件格式和大小校验在 Service 层完成
	article, err := h.svc.UploadDocuments(kbID, userID, file.Filename, fileType, file.Size, src)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, gin.H{
		"message":    "文档已接收，正在后台处理",
		"article_id": article.ID,
		"filename":   file.Filename,
		"kb_id":      kbID,
	})
}

// GetDocumentStatus 查询文档处理状态。
//
// GET /api/v1/admin/knowledge-bases/:kb_id/documents/:id/status
func (h *KnowledgeHandler) GetDocumentStatus(c *gin.Context) {
	// TODO(handler/knowledge): 未校验路径中的 kb_id 与 article.KBID 是否一致。
	// 错误 kb_id 仍可查询到其他知识库文章状态，破坏 URL 资源层级语义。
	articleID, ok := parseID(c, "id")
	if !ok {
		return
	}

	status, err := h.svc.GetDocumentStatus(articleID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	// TODO(handler/knowledge): 响应缺少 file_name/process_error/progress 字段，
	// 与 API 文档不一致。需 Service 层返回结构化状态对象而非 string。
	response.Success(c, gin.H{
		"article_id":     articleID,
		"process_status": status,
	})
}

// RetryDocument 重试文档处理（重新入队）。
//
// POST /api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry
func (h *KnowledgeHandler) RetryDocument(c *gin.Context) {
	articleID, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.RetryDocument(articleID); err != nil {
		handleServiceError(c, err)
		return
	}

	// TODO(handler/knowledge): 返回 message 与 API 文档不一致（文档："已重新加入处理队列"）。
	response.Success(c, gin.H{
		"message":    "重试已提交",
		"article_id": articleID,
	})
}
