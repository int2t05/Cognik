// handler.go 知识库管理 HTTP 接口（参数校验、调用 Service、格式化响应）。
package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"opsmind/internal/shared/dto/request"
	dto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Handler 共享工具
// =============================================================================

// parsePagination 解析分页参数（page 默认 1，pageSize 默认 10，上限 100）。
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}

// parseID 从路径参数解析 int64 ID，失败时返回错误响应。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 从 Gin context 获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 错误（AppError 提取业务码，其余 500）。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// =============================================================================
// KnowledgeHandler
// =============================================================================

// KnowledgeHandler 知识库管理接口。
type KnowledgeHandler struct {
	svc *KnowledgeService
}

// NewKnowledgeHandler 创建 KnowledgeHandler 实例。
func NewKnowledgeHandler(svc *KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{svc: svc}
}

// =============================================================================
// KnowledgeBase
// =============================================================================

// ListKBsForPortal 门户端知识库列表（无需 admin 权限，供 Chat 页选择知识库下拉框使用）。
//
// GET /api/v1/portal/knowledge-bases
func (h *KnowledgeHandler) ListKBsForPortal(c *gin.Context) {
	kbs, err := h.svc.ListKBs(c.Request.Context(), "")
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
	if err := h.svc.CreateKB(c.Request.Context(), req, userID); err != nil {
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

	if svcErr := h.svc.UpdateKB(c.Request.Context(), id, req); svcErr != nil {
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

	if svcErr := h.svc.DeleteKB(c.Request.Context(), id); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// ListKBs 列出全部知识库。
//
// GET /api/v1/admin/knowledge-bases
func (h *KnowledgeHandler) ListKBs(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	kbs, err := h.svc.ListKBs(c.Request.Context(), keyword)
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
	article, svcErr := h.svc.CreateArticle(c.Request.Context(), req, userID)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, article)
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

	if svcErr := h.svc.UpdateArticle(c.Request.Context(), id, req); svcErr != nil {
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
	if svcErr := h.svc.SubmitReview(c.Request.Context(), id, userID); svcErr != nil {
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
	if svcErr := h.svc.Review(c.Request.Context(), id, userID, req); svcErr != nil {
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

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.Disable(c.Request.Context(), id, userID); svcErr != nil {
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

// DeleteArticle 删除文章（仅草稿/驳回状态可删除）。
//
// DELETE /api/v1/admin/articles/:id
func (h *KnowledgeHandler) DeleteArticle(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if svcErr := h.svc.DeleteArticle(c.Request.Context(), id); svcErr != nil {
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
	keyword := c.DefaultQuery("keyword", "")

	result, svcErr := h.svc.ListArticles(c.Request.Context(), kbID, status, sourceType, processStatus, keyword, page, pageSize)
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

	result, svcErr := h.svc.GetArticleDetail(c.Request.Context(), id)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, result)
}

// =============================================================================
// 文档上传/状态/重试
// =============================================================================

// UploadDocuments 并发上传文档到知识库（multipart form，字段名 files，最多 10 个文件）。
// 每个文件独立处理，部分失败不影响其他文件，逐项返回成功/失败结果。
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
	tagsRaw := c.PostForm("tags") // 逗号分隔的标签，可选
	var tags []string
	if tagsRaw != "" {
		tags = strings.Split(tagsRaw, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// 预分配结果切片：每个 goroutine 只写自己的 index，无需互斥锁
	results := make([]dto.DocumentUploadItem, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // 并发度 4，避免大文件解析打满内存与连接

	for i, fh := range files {
		wg.Add(1)
		go func(idx int, fh *multipart.FileHeader) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			item := dto.DocumentUploadItem{FileName: fh.Filename, FileSize: fh.Size}

			fileType, reader, err := sniffFileType(fh)
			if err != nil {
				item.ErrorMsg = err.Error()
				results[idx] = item
				return
			}
			defer reader.Close()
			item.FileType = fileType

			article, err := h.svc.UploadDocuments(c.Request.Context(), kbID, userID, fh.Filename, fileType, fh.Size, tags, reader)
			if err != nil {
				item.ErrorMsg = extractErrMsg(err)
				results[idx] = item
				return
			}

			item.ArticleID = article.ID
			item.ProcessStatus = article.ProcessStatus
			item.Success = true
			results[idx] = item
		}(i, fh)
	}
	wg.Wait()

	response.Success(c, dto.DocumentUploadResponse{Documents: results})
}

// GetUploadConfig 返回文档上传配置（大小上限、允许类型、文件数上限）。
//
// GET /api/v1/config/upload
func (h *KnowledgeHandler) GetUploadConfig(c *gin.Context) {
	response.Success(c, h.svc.GetUploadConfig())
}

// UploadFile 通用文件上传（文章内嵌图片/附件），存到 article-assets 目录。
// 返回 { url } 供前端 Markdown 引用，访问走 /api/v1/admin/files/article-assets/{filename}。
//
// POST /api/v1/admin/files/upload
func (h *KnowledgeHandler) UploadFile(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Error(c, errcode.ErrParam, "未选择文件（字段名: file）")
		return
	}
	src, err := fh.Open()
	if err != nil {
		response.Error(c, errcode.ErrParam, "打开文件失败: "+err.Error())
		return
	}
	defer src.Close()

	storedName, err := h.svc.UploadAsset(c.Request.Context(), fh.Filename, fh.Header.Get("Content-Type"), fh.Size, src)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, gin.H{
		"url":      "/api/v1/admin/files/article-assets/" + storedName,
		"filename": storedName,
	})
}

// ServeFile 代理下载 article-assets 目录下的文件（Local 模式 ServeFile，MinIO 预签名重定向）。
//
// GET /api/v1/admin/files/article-assets/:filename
func (h *KnowledgeHandler) ServeFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" || strings.ContainsAny(filename, "/\\") {
		response.Error(c, errcode.ErrParam, "无效的文件名")
		return
	}
	path, err := h.svc.AssetLocalPath(c.Request.Context(), filename)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	// MinIO 预签名 URL（http 开头）重定向；否则本地路径 ServeFile
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		c.Redirect(302, path)
		return
	}
	c.File(path)
}

// extractErrMsg 从 error 提取用户可读消息（AppError 取 Message，其余取 Error()）。
func extractErrMsg(err error) string {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}

// sniffFileType 通过 MIME sniffing 检测文件类型（比扩展名更可靠，文本类型回退扩展名）。
// 返回的 reader 含完整文件内容，调用方负责关闭。
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
// OOXML(docx/xlsx/pptx)本质都是 zip，MIME 无法区分内部类型，按扩展名判定；
// 图片按 image/* 前缀 + 扩展名双重确认；文本类型回退扩展名。
func detectFileType(sniff []byte, filename string) string {
	mime := http.DetectContentType(sniff)
	ext := strings.ToLower(filepath.Ext(filename))

	switch {
	case mime == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument"), mime == "application/zip":
		switch ext {
		case ".docx":
			return "docx"
		case ".xlsx":
			return "xlsx"
		case ".pptx":
			return "pptx"
		default:
			return ""
		}
	case strings.HasPrefix(mime, "image/"):
		switch ext {
		case ".jpg", ".jpeg":
			return "jpg"
		case ".png":
			return "png"
		case ".gif":
			return "gif"
		case ".bmp":
			return "bmp"
		case ".webp":
			return "webp"
		default:
			return ""
		}
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

	result, err := h.svc.GetDocumentStatus(c.Request.Context(), kbID, articleID)
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

	if err := h.svc.RetryDocument(c.Request.Context(), kbID, articleID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
