// Package knowledge 知识库领域业务逻辑、数据访问与 HTTP 处理。
package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"opsmind/internal/infra/adapter"
	"opsmind/internal/infra/storage"
	"opsmind/internal/parser"
	"opsmind/internal/rag"
	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MaxDocumentSize 文档上传大小上限（50MB）。
const MaxDocumentSize = 50 * 1024 * 1024

// allowedDocumentTypes 支持上传的文档格式白名单。
var allowedDocumentTypes = map[string]bool{"pdf": true, "docx": true, "xlsx": true, "pptx": true, "md": true, "txt": true}

// 存储两桶模型：
//   - opsmind-documents：临时桶（原始文件、草稿正文、审核中各状态）
//   - opsmind-published：已发布桶（正文已嵌入 pgvector，RAG 可检索）
//
// 状态由 DB knowledge_articles.status 管理，存储不按状态分桶。
const (
	minioBucketDocs      = "opsmind-documents"
	minioBucketPublished = "opsmind-published"
)

// articleContentKey 返回文章在存储中的目录名（title 清洗特殊字符，无扩展名）。
func articleContentKey(title string) string {
	safe := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	).Replace(title)
	return safe
}

// formatArticleText 正文前附 markdown 一级标题，写入 MinIO 和 embedding 时统一使用。
func formatArticleText(title, content string) string {
	return "# " + title + "\n\n" + content
}

// AuditWriter 审计日志写入接口（消费者接口，避免跨领域循环依赖）。
type AuditWriter interface {
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
}

type knowledgeChunker interface {
	Split(text string) []string
}

type knowledgeEmbedder interface {
	Embed(ctx context.Context, texts []string, model string) ([][]float32, int, error)
}

type knowledgeDocParser interface {
	Parse(reader io.Reader, fileType string) (*parser.ParseResult, error)
}

// knowledgeRepo KnowledgeService 使用的仓库方法子集。
type knowledgeRepo interface {
	FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error)
	FindArticleByID(ctx context.Context, id int64) (*model.KnowledgeArticle, error)
	CreateArticle(ctx context.Context, article *model.KnowledgeArticle) error
	UpdateArticle(ctx context.Context, article *model.KnowledgeArticle) error
	UpdateArticleStatus(ctx context.Context, id int64, status int) error
	UpdateArticleReview(ctx context.Context, id int64, status int, reviewerID int64, reviewComment string) error
	UpdateArticleDisable(ctx context.Context, id int64) error
	UpdateArticleMinioPath(ctx context.Context, id int64, path string) error
	UpdateArticleStatusCAS(ctx context.Context, id int64, expectedOld, newStatus int) (int64, error)
	UpdateArticleProcessStatus(ctx context.Context, id int64, processStatus, processError string) error
	UpdateArticleMetrics(ctx context.Context, id int64, wordCount, chunkCount int) error
	CreateKB(ctx context.Context, kb *model.KnowledgeBase) error
	UpdateKB(ctx context.Context, kb *model.KnowledgeBase) error
	DeleteKB(ctx context.Context, id int64) error
	DeleteArticle(ctx context.Context, id int64) error
	ExistsByTitle(ctx context.Context, kbID int64, title string, excludeID int64) (bool, error)
	ListKBs(ctx context.Context) ([]model.KnowledgeBase, error)
	CountArticlesByKB(ctx context.Context) (map[int64]int, error)
	ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, keyword string, page, pageSize int) ([]model.KnowledgeArticle, int64, error)
	FindChunksByArticleID(ctx context.Context, articleID int64) ([]model.KnowledgeChunk, error)
}

// userNameResolver 按 ID 列表批量查询用户名称（仅 id + real_name）。
type userNameResolver interface {
	FindByIDs(ctx context.Context, ids []int64) ([]model.User, error)
}

// knowledgeMsgNotifier 知识库消息通知接口（MessageService 的子集）。
type knowledgeMsgNotifier interface {
	NotifyKnowledgeReviewed(ctx context.Context, articleID int64, articleTitle string, userID int64, approved bool, comment string) error
}

// KnowledgeService 知识库管理服务。
//
// 所有依赖使用接口类型，消费者接口模式，依赖最小化。
type KnowledgeService struct {
	repo                  knowledgeRepo
	userNames             userNameResolver
	chunker               knowledgeChunker
	embedder              knowledgeEmbedder
	store                 adapter.VectorStore
	docParser             knowledgeDocParser
	processor             *rag.Processor
	storage               storage.StorageClient
	auditWriter           AuditWriter
	onKBChanged           func(kbID int64) // publish/disable 后触发 BM25 重建等
	defaultEmbeddingModel string           // 当前默认嵌入模型名
	msgSvc                knowledgeMsgNotifier
}

// KnowledgeServiceOption 函数选项模式——仅设置非零值，其余保持 nil。
type KnowledgeServiceOption func(*KnowledgeService)

// WithUserNames 设置用户名解析器（用于列表/详情填充 created_by_name 等字段）。
func WithUserNames(u userNameResolver) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.userNames = u }
}

// WithChunker 设置文本分块器（发布/启用文章时使用）。
func WithChunker(c knowledgeChunker) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.chunker = c }
}

// WithEmbedder 设置向量嵌入器（发布/启用文章时使用）。
func WithEmbedder(e knowledgeEmbedder) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.embedder = e }
}

// WithVectorStore 设置 pgvector 向量存储（发布/启用/停用/删除时使用）。
func WithVectorStore(vs adapter.VectorStore) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.store = vs }
}

// WithDocParser 设置文档解析器（上传时非 MinIO 降级路径使用）。
func WithDocParser(dp knowledgeDocParser) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.docParser = dp }
}

// WithProcessor 设置文档异步处理器（上传时入队异步分块/embedding）。
func WithProcessor(p *rag.Processor) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.processor = p }
}

// WithStorage 设置对象存储客户端（上传时写入 MinIO）。
func WithStorage(sc storage.StorageClient) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.storage = sc }
}

// WithAuditWriter 设置审计日志写入器（Publish/Disable 时写入审计记录）。
func WithAuditWriter(aw AuditWriter) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.auditWriter = aw }
}

// WithOnKBChanged 设置知识库变更回调（publish/disable 后触发，用于 BM25 索引重建等）。
func WithOnKBChanged(fn func(kbID int64)) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.onKBChanged = fn }
}

// WithDefaultEmbeddingModel 设置全局默认 embedding 模型（KB 未配置时回退使用）。
func WithDefaultEmbeddingModel(model string) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.defaultEmbeddingModel = model }
}

// WithMessageNotifier 注入消息通知服务（审核结果通知文章作者）。
func WithMessageNotifier(msg knowledgeMsgNotifier) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.msgSvc = msg }
}

// SetDefaultEmbeddingConfig 热更新全局默认 embedding 模型名（OnChange 回调调用）。
func (s *KnowledgeService) SetDefaultEmbeddingConfig(model string) {
	s.defaultEmbeddingModel = model
}

// validateKBEmbeddingConfig 校验当前默认嵌入模型与 KB 绑定的模型是否一致。
//
// 维度固定 1024——所有 embedding 模型必须输出 1024 维向量。
// 不一致则拒绝操作，提示用户切换回原模型或更新 KB 配置。
func (s *KnowledgeService) validateKBEmbeddingConfig(kb *model.KnowledgeBase) error {
	if kb.EmbeddingModel != "" && kb.EmbeddingModel != s.defaultEmbeddingModel {
		return errcode.AppError{Code: errcode.ErrParam, Message: fmt.Sprintf(
			"当前默认嵌入模型（%s）与知识库绑定的模型（%s）不一致，请切换回 %s 或更新知识库配置",
			s.defaultEmbeddingModel, kb.EmbeddingModel, kb.EmbeddingModel)}
	}
	return nil
}

// effectiveEmbeddingModel 返回 KB 配置的模型，空则回退到全局默认。
func (s *KnowledgeService) effectiveEmbeddingModel(kbModel string) string {
	if kbModel != "" {
		return kbModel
	}
	return s.defaultEmbeddingModel
}

// NewKnowledgeService 创建 KnowledgeService 实例（repo 必需，其余通过选项注入）。
func NewKnowledgeService(repo knowledgeRepo, opts ...KnowledgeServiceOption) *KnowledgeService {
	s := &KnowledgeService{repo: repo}
	for _, o := range opts {
		o(s)
	}
	return s
}

// =============================================================================
// KnowledgeBase
// =============================================================================

// isPgUniqueViolation 判断是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
// 用于将数据库级别的唯一约束错误转换为用户友好的中文提示。
func isPgUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// CreateKB 创建知识库（仅写 PostgreSQL）。
func (s *KnowledgeService) CreateKB(ctx context.Context, req request.CreateKBRequest, userID int64) error {
	slug := strings.TrimSpace(req.Name)
	if slug == "" {
		slug = fmt.Sprintf("kb-%d", time.Now().UnixNano())
	}
	// 绑定当前默认嵌入模型名。向量维度固定 1024。
	embModel := req.EmbeddingModel
	if embModel == "" {
		embModel = s.defaultEmbeddingModel
	}
	kb := &model.KnowledgeBase{
		Name:             req.Name,
		Description:      req.Description,
		RAGWorkspaceSlug: slug,
		EmbeddingModel:   embModel,
		VectorDimension:  1024,
		LlmConfigID:      req.LlmConfigID,
		CreatedBy:        userID,
	}
	if err := s.repo.CreateKB(ctx, kb); err != nil {
		if isPgUniqueViolation(err) {
			return errcode.AppError{Code: errcode.ErrConflict, Message: "知识库名称已存在: " + req.Name}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "创建知识库失败: " + err.Error()}
	}
	return nil
}

// UpdateKB 更新知识库信息。
func (s *KnowledgeService) UpdateKB(ctx context.Context, id int64, req request.UpdateKBRequest) error {
	kb, err := s.repo.FindKBByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}
	kb.Name = req.Name
	kb.Description = req.Description
	if req.EmbeddingModel != "" {
		kb.EmbeddingModel = req.EmbeddingModel
	}
	if req.VectorDimension > 0 {
		kb.VectorDimension = req.VectorDimension
	}
	if err := s.repo.UpdateKB(ctx, kb); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新知识库失败: " + err.Error()}
	}
	return nil
}

// DeleteKB 删除知识库及其下所有内容。
//
// 先删向量再删数据库记录，避免向量删除失败产生孤儿数据。
// MinIO 文件和 BM25 缓存由适配器异步管理。
func (s *KnowledgeService) DeleteKB(ctx context.Context, id int64) error {
	_, err := s.repo.FindKBByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}

	// 删除 pgvector 向量分块
	if s.store != nil {
		if err := s.store.DeleteByKB(ctx, id); err != nil {
			slog.Warn("删除知识库向量分块失败", "kb_id", id, "error", err)
			// 向量删除失败不阻塞数据库删除，由后续清理任务处理孤儿向量
		}
	}

	return s.repo.DeleteKB(ctx, id)
}

// ListKBs 列出全部知识库（含文章数量统计）。
func (s *KnowledgeService) ListKBs(ctx context.Context) ([]response.KBResponse, error) {
	kbs, err := s.repo.ListKBs(ctx)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库列表失败: " + err.Error()}
	}

	// 批量获取文章数量，避免 N+1 查询
	counts := map[int64]int{}
	if s.repo != nil {
		var countErr error
		counts, countErr = s.repo.CountArticlesByKB(ctx)
		if countErr != nil {
			slog.Warn("批量获取文章计数失败，所有 KB 计数将显示为 0", "error", countErr)
		}
	}

	result := make([]response.KBResponse, len(kbs))
	for i, kb := range kbs {
		result[i] = response.KBResponse{
			ID:              kb.ID,
			Name:            kb.Name,
			Description:     kb.Description,
			EmbeddingModel:  kb.EmbeddingModel,
			VectorDimension: kb.VectorDimension,
			LlmConfigID:     kb.LlmConfigID,
			ArticleCount:    counts[kb.ID],
			CreatedBy:       kb.CreatedBy,
			CreatedAt:       kb.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:       kb.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return result, nil
}

// =============================================================================
// KnowledgeArticle CRUD
// =============================================================================

// checkTitleUnique 同 KB 下标题不可重复。
func (s *KnowledgeService) checkTitleUnique(ctx context.Context, kbID int64, title string, excludeID int64) error {
	exists, err := s.repo.ExistsByTitle(ctx, kbID, title, excludeID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "检查标题唯一性失败: " + err.Error()}
	}
	if exists {
		return errcode.AppError{Code: errcode.ErrConflict, Message: "文章标题已存在: " + title}
	}
	return nil
}

// findArticle 查询文章，GORM ErrRecordNotFound 自动转为 AppError。
func (s *KnowledgeService) findArticle(ctx context.Context, id int64) (*model.KnowledgeArticle, error) {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	return article, nil
}

// CreateArticle 创建知识文章（草稿状态），返回创建后的文章（含自动生成的 ID）。
func (s *KnowledgeService) CreateArticle(ctx context.Context, req request.CreateArticleRequest, userID int64) (*model.KnowledgeArticle, error) {
	kb, err := s.repo.FindKBByID(ctx, req.KBID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}
	if err := s.validateKBEmbeddingConfig(kb); err != nil {
		return nil, err
	}

	if err := s.checkTitleUnique(ctx, req.KBID, req.Title, 0); err != nil {
		return nil, err
	}

	sourceType := req.SourceType
	if sourceType == 0 {
		sourceType = 1
	}
	article := &model.KnowledgeArticle{
		KBID:       req.KBID,
		Title:      req.Title,
		Content:    req.Content,
		SourceType: sourceType,
		Tags:       marshalTags(req.Tags),
		Status:     1,
		CreatedBy:  userID,
		WordCount:  len([]rune(req.Content)),
		MinioPath:  minioBucketDocs + "/" + articleContentKey(req.Title),
	}
	if err := s.repo.CreateArticle(ctx, article); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	s.uploadArticleFilesAsync(minioBucketDocs, articleContentKey(req.Title), formatArticleText(req.Title, req.Content), nil)
	return article, nil
}

// UpdateArticle 更新文章（仅草稿/驳回/停用状态可编辑）。
func (s *KnowledgeService) UpdateArticle(ctx context.Context, id int64, req request.UpdateArticleRequest) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusDraft && article.Status != model.ArticleStatusRejected && article.Status != model.ArticleStatusDisabled {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿、驳回和停用状态可编辑"}
	}

	newStatus := article.Status
	if article.Status == model.ArticleStatusDisabled {
		newStatus = model.ArticleStatusDraft
	}
	article.Title = req.Title
	article.Content = req.Content
	article.WordCount = len([]rune(req.Content))
	article.Tags = marshalTags(req.Tags)
	article.Status = newStatus
	article.MinioPath = minioBucketDocs + "/" + articleContentKey(req.Title)
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章失败: " + err.Error()}
	}

	s.uploadArticleFilesAsync(minioBucketDocs, articleContentKey(req.Title), formatArticleText(req.Title, req.Content), nil)
	return nil
}

// SubmitReview 提交审核（草稿→待审核）。
func (s *KnowledgeService) SubmitReview(ctx context.Context, id int64, userID int64) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusDraft {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿状态可提交审核"}
	}
	return s.repo.UpdateArticleStatus(ctx, id, int(model.ArticleStatusReviewing))
}

// Review 审核文章（待审核→已通过/已驳回，精确字段更新避免脏写）。
func (s *KnowledgeService) Review(ctx context.Context, id int64, reviewerID int64, req request.ReviewRequest) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusReviewing {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅待审核状态可审核"}
	}
	if req.Approved {
		if err := s.repo.UpdateArticleReview(ctx, id, int(model.ArticleStatusApproved), reviewerID, ""); err != nil {
			return err
		}
		if s.msgSvc != nil {
			if err := s.msgSvc.NotifyKnowledgeReviewed(ctx, id, article.Title, article.CreatedBy, true, ""); err != nil {
				slog.Warn("审核通过通知失败", "article_id", id, "error", err)
			}
		}
		return nil
	}
	if strings.TrimSpace(req.ReviewComment) == "" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "驳回时必须填写审核意见"}
	}
	if err := s.repo.UpdateArticleReview(ctx, id, int(model.ArticleStatusRejected), reviewerID, req.ReviewComment); err != nil {
		return err
	}
	if s.msgSvc != nil {
		if err := s.msgSvc.NotifyKnowledgeReviewed(ctx, id, article.Title, article.CreatedBy, false, req.ReviewComment); err != nil {
			slog.Warn("审核驳回通知失败", "article_id", id, "error", err)
		}
	}
	return nil
}

// =============================================================================
// Publish / Disable / Enable
// =============================================================================

// Publish 发布文章——分块→embedding→pgvector 写入（CAS 防并发重复发布）。
func (s *KnowledgeService) Publish(ctx context.Context, id int64, publisherID int64) error {
	if s.chunker == nil || s.embedder == nil || s.store == nil {
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "RAG 管道未初始化（chunker/embedder/store 为空）"}
	}

	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusApproved {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已审核通过的文章可发布"}
	}

	rows, err := s.repo.UpdateArticleStatusCAS(ctx, id, int(model.ArticleStatusApproved), int(model.ArticleStatusPublished))
	if err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}
	if rows == 0 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "文章状态已变更，请刷新后重试"}
	}

	article.Status = model.ArticleStatusPublished
	return s.republishFromApproved(ctx, article, publisherID)
}

// republishFromApproved 上传正文到存储并入队异步处理（分块→embedding→pgvector）。Publish/Enable 共用。
func (s *KnowledgeService) republishFromApproved(ctx context.Context, article *model.KnowledgeArticle, publisherID int64) error {
	if s.chunker == nil || s.embedder == nil || s.store == nil {
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "RAG 服务未初始化（缺少 Chunker/Embedding/VectorStore）"}
	}
	if s.processor == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "异步处理器未初始化"}
	}
	if err := s.validateKBEmbeddingConfig(&article.KnowledgeBase); err != nil {
		return err
	}

	id := article.ID
	content := article.Content
	if strings.TrimSpace(content) == "" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "文章内容为空，无法发布"}
	}

	pubKey := articleContentKey(article.Title)
	formatted := formatArticleText(article.Title, content)

	// 同步上传正文到存储，确保处理器取任务时文件已就位
	if s.storage != nil {
		if err := s.storage.UploadFile(ctx, minioBucketDocs, pubKey, "markdown.md", strings.NewReader(formatted), int64(len(formatted)), "text/markdown"); err != nil {
			return errcode.AppError{Code: errcode.ErrStorageUnavailable, Message: "上传文章正文失败: " + err.Error()}
		}
	}

	// 更新文章状态（MinioPath 先指向临时桶，嵌入成功后再改指向已发布桶）
	article.Status = model.ArticleStatusPublished
	article.PublishedBy = &publisherID
	article.ProcessStatus = "processing"
	article.MinioPath = minioBucketDocs + "/" + pubKey
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}

	// 提交异步处理任务
	task := rag.ProcessTask{
		ArticleID:      id,
		KBID:           article.KBID,
		Bucket:         minioBucketDocs,
		Key:            pubKey,
		FileType:       "txt",
		EmbeddingModel: s.effectiveEmbeddingModel(article.KnowledgeBase.EmbeddingModel),
		OnStatusChange: func(aID int64, status, errMsg string) {
			s.onPublishComplete(context.Background(), aID, status, errMsg)
			if status == "completed" {
				// 嵌入成功 → 迁移到已发布桶 → 更新路径 → 触发 BM25 重建
				s.moveArticleDir(minioBucketDocs, minioBucketPublished, pubKey)
				_ = s.repo.UpdateArticleMinioPath(context.Background(), aID, minioBucketPublished+"/"+pubKey)
				if s.onKBChanged != nil {
					s.onKBChanged(article.KBID)
				}
			}
		},
		OnMetrics: func(aID int64, wordCount, chunkCount int) {
			s.onProcessMetrics(context.Background(), aID, wordCount, chunkCount)
		},
	}
	if err := s.processor.Submit(task); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "提交处理任务失败: " + err.Error()}
	}

	// 审计：发布文章
	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, publisherID, "knowledge.publish", "knowledge_article", id, "")
	}
	return nil
}

// Disable 停用文章——从 pgvector 删除向量，清零 chunk 计数，触发 BM25 重建。
func (s *KnowledgeService) Disable(ctx context.Context, id int64, operatorID int64) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusPublished {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已发布状态可停用"}
	}

	// 停用时不删向量——搜索侧通过 status=4 过滤，保留以支持增量 embedding。
	// 从已发布桶移回临时桶。
	s.moveArticleDir(minioBucketPublished, minioBucketDocs, articleContentKey(article.Title))

	if err := s.repo.UpdateArticleDisable(ctx, id); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}

	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, operatorID, "knowledge.disable", "knowledge_article", id, "")
	}
	if s.onKBChanged != nil {
		go s.onKBChanged(article.KBID)
	}
	return nil
}

// Enable 启用已停用文章——重新执行分块→embedding→pgvector 写入并发布。
func (s *KnowledgeService) Enable(ctx context.Context, id int64, publisherID int64) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	if article.Status != model.ArticleStatusDisabled {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已停用状态的文章可启用"}
	}
	article.Status = model.ArticleStatusApproved
	return s.republishFromApproved(ctx, article, publisherID)
}

// DeleteArticle 删除文章（任意状态均可删除，MinIO 清理异步执行）。
func (s *KnowledgeService) DeleteArticle(ctx context.Context, id int64) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	go s.cleanupArticleFiles(article)
	return s.repo.DeleteArticle(ctx, id)
}

// =============================================================================
// List / Detail
// =============================================================================

// toArticleResponse 将 model 转为 API 响应结构（复用 ListArticles 和 GetArticleDetail）。
func toArticleResponse(a model.KnowledgeArticle, userNames map[int64]string) response.ArticleResponse {
	return response.ArticleResponse{
		ID: a.ID, KBID: a.KBID, KBName: a.KnowledgeBase.Name,
		Title: a.Title, Content: a.Content, Tags: unmarshalTags(a.Tags),
		Status: a.Status, StatusText: model.ArticleStatusText(a.Status),
		SourceType: a.SourceType, SourceTypeText: model.ArticleSourceTypeText(a.SourceType),
		FileType: a.FileType, MinioPath: a.MinioPath,
		WordCount: a.WordCount, ChunkCount: a.ChunkCount,
		ProcessStatus: a.ProcessStatus, ProcessError: a.ProcessError,
		CreatedBy: a.CreatedBy, CreatedByName: userNames[a.CreatedBy],
		ReviewedBy: a.ReviewedBy, PublishedBy: a.PublishedBy,
		PublishedByName: userNames[ptrVal(a.PublishedBy)],
		ReviewComment:   a.ReviewComment,
		CreatedAt:       a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// ListArticles 分页查询文章列表，支持 keyword 搜索（标题/标签模糊匹配）。
func (s *KnowledgeService) ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, keyword string, page, pageSize int) (*response.ArticleListResponse, error) {
	articles, total, err := s.repo.ListArticles(ctx, kbID, status, sourceType, processStatus, keyword, page, pageSize)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章列表失败: " + err.Error()}
	}

	userNames := s.resolveUserNames(ctx, articles)
	result := make([]response.ArticleResponse, len(articles))
	for i, a := range articles {
		result[i] = toArticleResponse(a, userNames)
	}

	return &response.ArticleListResponse{Articles: result, Total: total}, nil
}

// GetArticleDetail 获取文章详情（含切片）。
func (s *KnowledgeService) GetArticleDetail(ctx context.Context, id int64) (*response.ArticleDetailResponse, error) {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return nil, err
	}

	chunks, err := s.repo.FindChunksByArticleID(ctx, id)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询切片失败: " + err.Error()}
	}

	userNames := s.resolveUserNames(ctx, []model.KnowledgeArticle{*article})
	chunkResponses := make([]response.ChunkResponse, len(chunks))
	for i, c := range chunks {
		chunkResponses[i] = response.ChunkResponse{
			ID: c.ID, KBID: c.KBID, Content: c.Content, ChunkIndex: c.ChunkIndex,
			EmbeddingModel: c.EmbeddingModel, VectorDimension: c.VectorDimension,
			CreatedAt: c.CreatedAt,
		}
	}

	return &response.ArticleDetailResponse{
		ArticleResponse: toArticleResponse(*article, userNames),
		Chunks:          chunkResponses,
	}, nil
}

// =============================================================================
// 文档上传与处理
// =============================================================================

// UploadDocuments 上传文档到知识库（解析→创建文章，分块/embedding 推迟到 Publish）。
func (s *KnowledgeService) UploadDocuments(ctx context.Context, kbID int64, userID int64, filename string, fileType string, fileSize int64, tags []string, content io.Reader) (*model.KnowledgeArticle, error) {
	_, err := s.repo.FindKBByID(ctx, kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}

	if !allowedDocumentTypes[fileType] {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "不支持的文件格式: " + fileType + "（支持: pdf/docx/xlsx/pptx/md/txt）"}
	}

	if fileSize > MaxDocumentSize {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件大小超过限制（最大 50MB）"}
	}

	title := strings.TrimSuffix(filename, "."+fileType)
	if title == "" {
		title = filename
	}

	if err := s.checkTitleUnique(ctx, kbID, title, 0); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(content, MaxDocumentSize))
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "读取上传文件失败: " + err.Error()}
	}
	if len(data) == 0 {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件内容为空"}
	}

	if s.docParser == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "文档解析器未初始化"}
	}
	result, err := s.docParser.Parse(bytes.NewReader(data), fileType)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档解析失败: " + err.Error()}
	}
	text := result.Markdown
	if strings.TrimSpace(text) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档内容为空"}
	}

	tagsJSON := marshalTags(tags)
	article := &model.KnowledgeArticle{
		KBID:       kbID,
		Title:      title,
		Content:    text,
		Tags:       tagsJSON,
		SourceType: 2, // 文档上传
		FileType:   fileType,
		WordCount:  len([]rune(text)),
		Status:     1,
		CreatedBy:  userID,
	}

	if err := s.repo.CreateArticle(ctx, article); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	key := articleContentKey(article.Title)
	article.MinioPath = minioBucketDocs + "/" + key
	_ = s.repo.UpdateArticle(ctx, article)
	s.uploadArticleFilesAsync(minioBucketDocs, key, formatArticleText(title, text), result.Images)

	return article, nil
}

// GetDocumentStatus 查询文档处理状态。
func (s *KnowledgeService) GetDocumentStatus(ctx context.Context, kbID int64, articleID int64) (*response.DocumentStatusResponse, error) {
	article, err := s.findArticle(ctx, articleID)
	if err != nil {
		return nil, err
	}
	if article.KBID != kbID {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不属于指定知识库"}
	}
	return &response.DocumentStatusResponse{
		ArticleID:     article.ID,
		FileName:      article.Title,
		ProcessStatus: mapArticleToProcessStatus(article),
		ProcessError:  article.ProcessError,
	}, nil
}

// RetryDocument 重试文档处理（仅 process_status=failed 可重试）。
func (s *KnowledgeService) RetryDocument(ctx context.Context, kbID int64, articleID int64) error {
	article, err := s.findArticle(ctx, articleID)
	if err != nil {
		return err
	}
	if article.KBID != kbID {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不属于指定知识库"}
	}
	if article.ProcessStatus != "failed" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅处理失败的文章可重试"}
	}
	if s.processor == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "文档处理器未初始化"}
	}

	if err := s.repo.UpdateArticleProcessStatus(ctx, articleID, "pending", ""); err != nil {
		slog.Warn("重置处理状态失败，不阻断主流程", "article_id", articleID, "error", err)
	}
	task := rag.ProcessTask{
		ArticleID:      articleID,
		KBID:           article.KBID,
		Content:        article.Content,
		EmbeddingModel: s.effectiveEmbeddingModel(article.KnowledgeBase.EmbeddingModel),
		OnStatusChange: func(aID int64, status, errMsg string) {
			s.onProcessStatusChange(context.Background(), aID, status, errMsg)
		},
		OnMetrics: func(aID int64, wordCount, chunkCount int) {
			s.onProcessMetrics(context.Background(), aID, wordCount, chunkCount)
		},
	}
	if err := s.processor.Submit(task); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "提交处理任务失败: " + err.Error()}
	}
	return nil
}

// =============================================================================
// 辅助函数
// =============================================================================

// onProcessStatusChange 更新文档处理状态，失败时记录日志但不阻塞流程。
func (s *KnowledgeService) onProcessStatusChange(ctx context.Context, aID int64, status, errMsg string) {
	if err := s.repo.UpdateArticleProcessStatus(ctx, aID, status, errMsg); err != nil {
		slog.Warn("更新文档处理状态失败", "article_id", aID, "status", status, "error", err)
	}
}

// onPublishComplete 发布完成回调（仅终态写入 process_status）。
func (s *KnowledgeService) onPublishComplete(ctx context.Context, aID int64, status, errMsg string) {
	switch status {
	case "completed":
		_ = s.repo.UpdateArticleProcessStatus(ctx, aID, "completed", "")
	case "failed":
		_ = s.repo.UpdateArticleProcessStatus(ctx, aID, "failed", errMsg)
	}
}

// onProcessMetrics 更新文档指标（字数/分块数），失败时记录日志但不阻塞流程。
func (s *KnowledgeService) onProcessMetrics(ctx context.Context, aID int64, wordCount, chunkCount int) {
	if err := s.repo.UpdateArticleMetrics(ctx, aID, wordCount, chunkCount); err != nil {
		slog.Warn("更新文档指标失败", "article_id", aID, "error", err)
	}
}

// resolveUserNames 批量解析用户名（去重后一次查询，避免 N+1）。
func (s *KnowledgeService) resolveUserNames(ctx context.Context, articles []model.KnowledgeArticle) map[int64]string {
	if s.userNames == nil || len(articles) == 0 {
		return map[int64]string{}
	}
	ids := make(map[int64]bool)
	for _, a := range articles {
		if a.CreatedBy > 0 {
			ids[a.CreatedBy] = true
		}
		if a.PublishedBy != nil && *a.PublishedBy > 0 {
			ids[*a.PublishedBy] = true
		}
	}
	if len(ids) == 0 {
		return map[int64]string{}
	}
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	users, err := s.userNames.FindByIDs(ctx, idList)
	if err != nil {
		slog.Warn("批量查询用户名失败", "error", err)
		return map[int64]string{}
	}
	m := make(map[int64]string, len(users))
	for _, u := range users {
		m[u.ID] = u.RealName
	}
	return m
}

func ptrVal(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// maxTagCount 标签数量上限，防止 JSONB 膨胀。
const maxTagCount = 10

func marshalTags(tags []string) datatypes.JSON {
	if len(tags) == 0 {
		return datatypes.JSON(`[]`)
	}
	seen := make(map[string]bool, len(tags))
	clean := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		clean = append(clean, t)
		if len(clean) >= maxTagCount {
			break
		}
	}
	if len(clean) == 0 {
		return datatypes.JSON(`[]`)
	}
	data, _ := json.Marshal(clean)
	return datatypes.JSON(data)
}

func unmarshalTags(data datatypes.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	var tags []string
	_ = json.Unmarshal(data, &tags)
	if tags == nil {
		return []string{}
	}
	return tags
}

// mapArticleToProcessStatus 返回文章处理状态（取值见 rag.Processor）。
func mapArticleToProcessStatus(article *model.KnowledgeArticle) string {
	if article.ProcessStatus != "" {
		return article.ProcessStatus
	}
	return "pending"
}

// cleanupArticleFiles 异步清理文章在两个桶中的 {标题}.txt。
func (s *KnowledgeService) cleanupArticleFiles(article *model.KnowledgeArticle) {
	if s.storage == nil {
		return
	}
	bg := context.Background()
	key := articleContentKey(article.Title)
	s.deleteArticleDir(bg, minioBucketDocs, key)
	s.deleteArticleDir(bg, minioBucketPublished, key)
}

// uploadArticleFilesAsync 异步上传正文和图片到存储（目录式 markdown.md + images/{name}），不阻塞主流程。
func (s *KnowledgeService) uploadArticleFilesAsync(bucket, dir, content string, images map[string][]byte) {
	if s.storage == nil {
		return
	}
	go func() {
		bg := context.Background()
		if err := s.storage.UploadFile(bg, bucket, dir, "markdown.md", strings.NewReader(content), int64(len(content)), "text/markdown"); err != nil {
			slog.Warn("异步上传文章失败", "bucket", bucket, "dir", dir, "error", err)
		}
		for name, data := range images {
			if err := s.storage.UploadFile(bg, bucket, dir, "images/"+name, bytes.NewReader(data), int64(len(data)), imageContentType(name)); err != nil {
				slog.Warn("异步上传图片失败", "bucket", bucket, "dir", dir, "name", name, "error", err)
			}
		}
	}()
}

// moveArticleDir 从 srcBucket 移动到 dstBucket（下载目录→上传→删除源，尽力而为不阻塞主流程）。
func (s *KnowledgeService) moveArticleDir(srcBucket, dstBucket, dir string) {
	if s.storage == nil || srcBucket == "" || dstBucket == "" || dir == "" {
		return
	}
	go func() {
		bg := context.Background()
		files, err := s.storage.DownloadDir(bg, srcBucket, dir)
		if err != nil {
			slog.Warn("moveArticleDir 下载失败", "src", srcBucket, "dir", dir, "error", err)
			return
		}
		uploadFailed := false
		for filename, reader := range files {
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				slog.Warn("moveArticleDir 读取失败", "filename", filename, "error", err)
				uploadFailed = true
				continue
			}
			if err := s.storage.UploadFile(bg, dstBucket, dir, filename, bytes.NewReader(data), int64(len(data)), "text/markdown"); err != nil {
				slog.Warn("moveArticleDir 上传失败", "dst", dstBucket, "filename", filename, "error", err)
				uploadFailed = true
			}
		}
		if !uploadFailed {
			if err := s.storage.DeleteDir(bg, srcBucket, dir); err != nil {
				slog.Warn("moveArticleDir 删除源失败", "src", srcBucket, "dir", dir, "error", err)
			}
		}
	}()
}

// deleteArticleDir 安全删除文章目录（bucket/dir 为空或 storage 为 nil 时静默跳过）。
func (s *KnowledgeService) deleteArticleDir(ctx context.Context, bucket, dir string) {
	if s.storage == nil || bucket == "" || dir == "" {
		return
	}
	if err := s.storage.DeleteDir(ctx, bucket, dir); err != nil {
		slog.Warn("删除文章目录失败", "bucket", bucket, "dir", dir, "error", err)
	}
}

// imageContentType 根据文件扩展名推断 HTTP Content-Type。
func imageContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}
