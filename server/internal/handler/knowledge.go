// Package handler 实现 HTTP 请求处理。
//
// knowledge.go 提供知识库管理相关接口（KB/文章/审核/发布/文档上传）。
package handler

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"opsmind/internal/dto/request"
	dto "opsmind/internal/dto/response"
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

	if svcErr := h.svc.UpdateArticle(id, req); svcErr != nil {
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

// ListArticles 分页查询文章列表。
//
// GET /api/v1/admin/knowledge-bases/:kb_id/articles
func (h *KnowledgeHandler) ListArticles(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}

	page, pageSize := parsePagination(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	sourceType, _ := strconv.Atoi(c.DefaultQuery("source_type", "0"))
	processStatus := c.DefaultQuery("process_status", "")

	result, svcErr := h.svc.ListArticles(kbID, status, sourceType, processStatus, page, pageSize)
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

// UploadDocuments 上传文档到知识库（multipart form，字段名 files，最多 10 个文件）。
//
// POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload
func (h *KnowledgeHandler) UploadDocuments(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		response.Error(c, errcode.ErrParam, "文件上传解析失败: "+err.Error())
		return
	}
	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		response.Error(c, errcode.ErrParam, "未选择文件（字段名: files）")
		return
	}
	if len(files) > 10 {
		response.Error(c, errcode.ErrParam, "单次最多上传 10 个文件")
		return
	}

	userID, _ := getCurrentUserID(c)
	items := make([]dto.DocumentUploadItem, 0, len(files))

	for _, fh := range files {
		fileType, reader, err := sniffFileType(fh)
		if err != nil {
			response.Error(c, errcode.ErrParam, fh.Filename+": "+err.Error())
			return
		}
		defer reader.Close()

		article, err := h.svc.UploadDocuments(kbID, userID, fh.Filename, fileType, fh.Size, reader)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		items = append(items, dto.DocumentUploadItem{
			ArticleID:     article.ID,
			FileName:      fh.Filename,
			FileSize:      fh.Size,
			FileType:      fileType,
			ProcessStatus: article.ProcessStatus,
		})
	}

	response.Success(c, dto.DocumentUploadResponse{Documents: items})
}

// sniffFileType 通过 MIME sniffing 检测文件类型。
//
// 为什么用 MIME sniffing 而非仅信任扩展名：恶意文件可通过改扩展名绕过格式白名单。
// http.DetectContentType 读取前 512 字节进行内容嗅探，比扩展名更可靠。
// 文本类型（md/txt）MIME 无法区分，回退到扩展名判断。
// 返回的 reader 包含完整文件内容（嗅探字节 + 剩余部分），调用方负责关闭。
func sniffFileType(fh *multipart.FileHeader) (string, io.ReadCloser, error) {
	src, err := fh.Open()
	if err != nil {
		return "", nil, fmt.Errorf("打开文件失败: %w", err)
	}

	sniff := make([]byte, 512)
	n, _ := io.ReadFull(src, sniff)
	sniff = sniff[:n]

	combined := io.NopCloser(io.MultiReader(bytes.NewReader(sniff), src))

	fileType := detectFileType(sniff, fh.Filename)
	if fileType == "" {
		combined.Close()
		return "", nil, fmt.Errorf("不支持的文件类型（MIME 检测失败）")
	}
	return fileType, combined, nil
}

// detectFileType 根据 MIME 嗅探结果和扩展名判断文件类型。
func detectFileType(sniff []byte, filename string) string {
	mime := http.DetectContentType(sniff)
	ext := strings.ToLower(filepath.Ext(filename))

	switch {
	case mime == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument"):
		return "docx"
	case mime == "application/zip":
		if ext == ".docx" {
			return "docx"
		}
		return ""
	default:
		// text/plain 等文本类型回退扩展名
		switch ext {
		case ".md", ".markdown":
			return "md"
		case ".txt":
			return "txt"
		default:
			return ""
		}
	}
}

// GetDocumentStatus 查询文档处理状态。
//
// GET /api/v1/admin/knowledge-bases/:kb_id/documents/:id/status
func (h *KnowledgeHandler) GetDocumentStatus(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}
	articleID, ok := parseID(c, "id")
	if !ok {
		return
	}

	result, err := h.svc.GetDocumentStatus(kbID, articleID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, result)
}

// RetryDocument 重试文档处理（重新入队）。
//
// POST /api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry
func (h *KnowledgeHandler) RetryDocument(c *gin.Context) {
	kbID, ok := parseID(c, "kb_id")
	if !ok {
		return
	}
	articleID, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.RetryDocument(kbID, articleID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
