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
	"sort"
	"strings"
	"time"

	"cognos/internal/domain/system/audit"
	"cognos/internal/infra/adapter"
	"cognos/internal/infra/storage"
	"cognos/internal/parser"
	"cognos/internal/rag"
	"cognos/internal/shared/dto/request"
	"cognos/internal/shared/dto/response"
	"cognos/internal/shared/model"
	"cognos/internal/shared/pkg/errcode"
	"cognos/internal/shared/pkg/pathutil"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// allowedDocumentTypes 支持上传的文档格式白名单（图片格式仅 MinerU 引擎可解析，本地降级不支持）。
var allowedDocumentTypes = map[string]bool{
	"pdf": true, "docx": true, "xlsx": true, "pptx": true, "md": true, "txt": true,
	"jpg": true, "png": true, "gif": true, "bmp": true, "webp": true,
}

// 存储布局（单桶，扁平 md 文件 + 统一图片目录）：
//
//	cognos-documents/kb-{kbID}/{draft|published}/{slug}.md   正文（扁平文件，非目录）
//	cognos-documents/image/{hash}.{ext}                      图片（全局统一目录，内容寻址去重）
//
// 草稿与已发布由目录区分；文件名用 slug（kebab-case 标题），articleID 移入 frontmatter。
// 图片与文章解耦：任何文章/上下文都可通过 /api/v1/public/images/{name} 解析，无需 articleId。
const (
	minioBucket = "cognos-documents"
	// imageDir 图片全局统一目录名（桶下）。
	imageDir = "image"
	// maxUploadFileCount 单次上传文件数上限，handler 校验与 GetUploadConfig 返回共用。
	maxUploadFileCount = 10
)

// articleFile 返回文章 markdown 在存储中的相对路径（kb-{kbID}/{draft|published}/{slug}.md）。
// slug 从标题派生（kebab-case），articleID 移入 frontmatter 保留 DB 关联。
func articleFile(kbID int64, slug string, published bool) string {
	status := "draft"
	if published {
		status = "published"
	}
	return fmt.Sprintf("kb-%d/%s/%s.md", kbID, status, slug)
}

// articleFileDir 返回文章 markdown 所在目录（kb-{kbID}/{draft|published}），用于 UploadFile 的 dir 参数。
func articleFileDir(kbID int64, published bool) string {
	if published {
		return fmt.Sprintf("kb-%d/published", kbID)
	}
	return fmt.Sprintf("kb-%d/draft", kbID)
}

// articleFileName 返回文章 markdown 文件名（{slug}.md）。
func articleFileName(slug string) string {
	return fmt.Sprintf("%s.md", slug)
}

// formatArticleText 正文前附 markdown 一级标题，写入 MinIO 和 embedding 时统一使用。
func formatArticleText(title, content string) string {
	return "# " + title + "\n\n" + content
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
	ListKBs(ctx context.Context, keyword string) ([]model.KnowledgeBase, error)
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

// ticketCreator 工单创建接口（TicketService 的子集）。
// 用于上传文件 / LLM 元数据补全时自动提工单标记人工复核。nil-safe。
type ticketCreator interface {
	CreateSystemTicket(ctx context.Context, req request.SystemTicketRequest) error
}

// KnowledgeService 知识库管理服务。
//
// 所有依赖使用接口类型，消费者接口模式，依赖最小化。
type KnowledgeService struct {
	repo                  knowledgeRepo
	userNames             userNameResolver
	chunker               rag.TextChunker
	embedder              rag.TextEmbedder
	store                 adapter.VectorStore
	docParser             *parser.Parser
	processor             *rag.Processor
	storage               storage.StorageClient
	auditWriter           audit.AuditWriter
	onKBChanged           func(kbID int64) // publish/disable 后触发 BM25 重建等
	defaultEmbeddingModel string           // 当前默认嵌入模型名
	msgSvc                knowledgeMsgNotifier
	maxUploadSize         int64            // 上传大小上限(字节)，由 config 注入
	metaCompleter         MetadataCompleter // 发布时 LLM 补全 type/tags（nil 时降级 guide）
	embeddingDimension    int              // 向量维度，创建 KB 时写入（由 config 注入）
	ticketCreator         ticketCreator    // 上传/补全时提工单（nil-safe，setter 注入）
}

// KnowledgeServiceOption 函数选项模式——仅设置非零值，其余保持 nil。
type KnowledgeServiceOption func(*KnowledgeService)

// WithUserNames 设置用户名解析器（用于列表/详情填充 created_by_name 等字段）。
func WithUserNames(u userNameResolver) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.userNames = u }
}

// WithChunker 设置文本分块器（发布/启用文章时使用）。
func WithChunker(c rag.TextChunker) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.chunker = c }
}

// WithEmbedder 设置向量嵌入器（发布/启用文章时使用）。
func WithEmbedder(e rag.TextEmbedder) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.embedder = e }
}

// WithVectorStore 设置 pgvector 向量存储（发布/启用/停用/删除时使用）。
func WithVectorStore(vs adapter.VectorStore) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.store = vs }
}

// WithDocParser 设置文档解析器（上传时非 MinIO 降级路径使用）。
func WithDocParser(dp *parser.Parser) KnowledgeServiceOption {
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
func WithAuditWriter(aw audit.AuditWriter) KnowledgeServiceOption {
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

// WithMaxUploadSize 设置文档上传大小上限（字节，由 config 的 KB 值转换注入）。
func WithMaxUploadSize(bytes int64) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.maxUploadSize = bytes }
}

// SetMetadataCompleter 注入元数据补全器（setter 注入，晚于构造，复用 agent ChatModel）。
func (s *KnowledgeService) SetMetadataCompleter(mc MetadataCompleter) {
	s.metaCompleter = mc
}

// WithEmbeddingDimension 注入向量维度（创建 KB 时写入）。
func WithEmbeddingDimension(dim int) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.embeddingDimension = dim }
}

// SetTicketCreator 注入工单创建器（setter 注入，解决初始化顺序：ticketService 晚于 knowledgeService 创建）。
func (s *KnowledgeService) SetTicketCreator(tc ticketCreator) {
	s.ticketCreator = tc
}

// SetDefaultEmbeddingConfig 热更新全局默认 embedding 模型名（OnChange 回调调用）。
func (s *KnowledgeService) SetDefaultEmbeddingConfig(model string) {
	s.defaultEmbeddingModel = model
}

// validateKBEmbeddingConfig 校验当前默认嵌入模型与 KB 绑定的模型是否一致。
//
// 维度由 embeddingDimension 决定——所有 embedding 模型必须输出同维度向量。
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
	// 绑定当前默认嵌入模型名。向量维度由 embeddingDimension 决定。
	embModel := req.EmbeddingModel
	if embModel == "" {
		embModel = s.defaultEmbeddingModel
	}
	kb := &model.KnowledgeBase{
		Name:             req.Name,
		Description:      req.Description,
		RAGWorkspaceSlug: slug,
		EmbeddingModel:   embModel,
		VectorDimension:  s.embeddingDimension,
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
func (s *KnowledgeService) ListKBs(ctx context.Context, keyword string) ([]response.KBResponse, error) {
	kbs, err := s.repo.ListKBs(ctx, keyword)
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
		Title:       req.Title,
		Content:     req.Content,
		SourceType:  sourceType,
		ArticleType: req.ArticleType, // 留空则发布时 LLM 补全
		Tags:        marshalTags(req.Tags),
		Status:      1,
		CreatedBy:   userID,
		WordCount:   len([]rune(req.Content)),
	}
	if err := s.repo.CreateArticle(ctx, article); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	// 先建后设路径：CreateArticle 拿到 ID 后再构造存储路径，避免 ID=0 缺陷。
	fileKey := articleFile(req.KBID, slugify(article.Title), false)
	article.MinioPath = minioBucket + "/" + fileKey
	if err := s.repo.UpdateArticleMinioPath(ctx, article.ID, article.MinioPath); err != nil {
		slog.Warn("更新文章存储路径失败", "article_id", article.ID, "error", err)
	}
	// 同步写入文件（文件为真相源，确保返回前文件已落盘）。
	if err := s.uploadArticleFileSync(minioBucket, fileKey, formatArticleText(req.Title, req.Content), nil); err != nil {
		slog.Warn("同步写入文章文件失败，Content 仍存 DB 可回填", "article_id", article.ID, "error", err)
	}
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

	// 标题变更时检查唯一性（防止 slug 冲突）
	if req.Title != article.Title {
		if err := s.checkTitleUnique(ctx, article.KBID, req.Title, id); err != nil {
			return err
		}
	}

	newStatus := article.Status
	if article.Status == model.ArticleStatusDisabled {
		newStatus = model.ArticleStatusDraft
	}
	article.Title = req.Title
	article.Content = req.Content
	article.WordCount = len([]rune(req.Content))
	article.Tags = marshalTags(req.Tags)
	if req.ArticleType != "" {
		article.ArticleType = req.ArticleType
	}
	article.Status = newStatus
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章失败: " + err.Error()}
	}

	// 路径基于 ID 稳定不变，标题变更不触发路径迁移；仅重传 .md 文件（内容已变）。
	// 该函数仅允许草稿/驳回/停用态编辑，文件恒在 draft 目录。
	s.uploadArticleFilesAsync(minioBucket, articleFile(article.KBID, slugify(article.Title), false), formatArticleText(req.Title, req.Content), nil)
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

	// 元数据补全：解析 frontmatter → 若 type 缺失/非法则 LLM 推断 → 提工单标记人工复核
	meta, body := ParseArticleMeta(content)
	effectiveType := meta.Type
	if effectiveType == "" {
		effectiveType = article.ArticleType // DB 列
	}
	needsCompletion := !IsAllowedType(effectiveType) // type 缺失/非法 → 需 LLM 补全 + 提工单
	if needsCompletion && s.metaCompleter != nil {
		completed, err := s.metaCompleter.Complete(ctx, article.Title, body, meta)
		if err == nil {
			meta = completed
			effectiveType = completed.Type
		}
	}
	// 最终兜底：LLM 失败或未注入 → 降级 guide
	if !IsAllowedType(effectiveType) {
		effectiveType = "guide"
	}
	meta.Type = effectiveType
	if article.ArticleType != effectiveType {
		article.ArticleType = effectiveType
	}
	if len(meta.Tags) > 0 && len(article.Tags) == 0 {
		// LLM 补全的 tags 回写 DB（仅当 DB 无 tags 时）
		article.Tags = marshalTags(meta.Tags)
	}

	// LLM 补全触发时 → 提工单标记人工复核（nil-safe，失败仅记日志）
	if needsCompletion && s.ticketCreator != nil {
		articleID := article.ID
		kbID := article.KBID
		resolvedType := effectiveType
		if err := s.ticketCreator.CreateSystemTicket(ctx, request.SystemTicketRequest{
			Title:            fmt.Sprintf("【元数据复核】%s", article.Title),
			Description:      fmt.Sprintf("文章 (ID=%d) 发布时 type 缺失，LLM 推断为 %s。请人工复核分类准确性。", articleID, resolvedType),
			Tags:             []string{"元数据复核", "auto"},
			RelatedArticleID: &articleID,
			RelatedKBID:      &kbID,
			Reason:           "metadata_auto_completed",
		}); err != nil {
			slog.Warn("元数据复核工单创建失败", "article_id", articleID, "error", err)
		}
	}

	// 发布前文件位于 draft 目录；嵌入成功后由处理器回调移到 published 目录。
	draftFile := articleFile(article.KBID, slugify(article.Title), false)
	formatted := RenderArticleFile(meta, article.Title, body)

	// 同步上传正文到存储（扁平 .md 文件），确保处理器取任务时文件已就位
	if s.storage != nil {
		draftDir := articleFileDir(article.KBID, false)
		if err := s.storage.UploadFile(ctx, minioBucket, draftDir, articleFileName(slugify(article.Title)), strings.NewReader(formatted), int64(len(formatted)), "text/markdown"); err != nil {
			return errcode.AppError{Code: errcode.ErrStorageUnavailable, Message: "上传文章正文失败: " + err.Error()}
		}
	}

	// 更新文章状态（MinioPath 指向 draft .md；嵌入成功后回调改为 published）
	article.Status = model.ArticleStatusPublished
	article.PublishedBy = &publisherID
	article.ProcessStatus = "processing"
	article.MinioPath = minioBucket + "/" + draftFile
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}

	// 提交异步处理任务（Key 为完整 .md 文件路径，处理器用 DownloadFile 读取）
	task := rag.ProcessTask{
		ArticleID:      id,
		KBID:           article.KBID,
		Bucket:         minioBucket,
		Key:            draftFile,
		FileType:       "txt",
		EmbeddingModel: s.effectiveEmbeddingModel(article.KnowledgeBase.EmbeddingModel),
		OnStatusChange: func(aID int64, status, errMsg string) {
			s.onPublishComplete(context.Background(), aID, status, errMsg)
			if status == "completed" {
				// 嵌入成功 → draft→published 单文件移动 → 更新路径 → 触发 BM25 重建
				// 图片在全局 image/ 目录，不随发布移动（draft 与 published 共享同一图片）。
				publishedFile := articleFile(article.KBID, slugify(article.Title), true)
				if err := s.moveArticleFile(minioBucket, draftFile, publishedFile); err == nil {
					_ = s.repo.UpdateArticleMinioPath(context.Background(), aID, minioBucket+"/"+publishedFile)
				}
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

// CreateAndPublish 创建文章并自动发布（agent 自迭代闭环）。
// 绕过 SubmitReview/Review/Publish 三道人工关卡，Draft→Published 直达。
// 标题精确去重 + 语义去重（相似度 > 阈值则拒绝，提示更新已有文章）。
// 仅用于 SourceType=DeepResearch（agent 写回）；人工创建仍走 CreateArticle + 审核。
func (s *KnowledgeService) CreateAndPublish(ctx context.Context, req request.CreateArticleRequest, userID int64) (*model.KnowledgeArticle, error) {
	// CreateArticle 含校验 + 标题去重 + 写 DB(Draft) + 写 draft/ 文件。
	article, err := s.CreateArticle(ctx, req, userID)
	if err != nil {
		return nil, err
	}
	// 语义去重：embed 标题 + 首段，搜 KB 内已发布文章，相似度过高则拒绝。
	if err := s.checkSemanticDuplicate(ctx, article); err != nil {
		// 去重失败 → 删除刚建的 Draft（回滚），返回冲突提示 agent 用 update。
		_ = s.repo.DeleteArticle(ctx, article.ID)
		return nil, err
	}
	// 加载 KnowledgeBase 关联（republishFromApproved 依赖 article.KnowledgeBase）。
	full, err := s.repo.FindArticleByID(ctx, article.ID)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "重新加载文章失败: " + err.Error()}
	}
	// republishFromApproved 内部设 Status=Published + processor.Submit 异步 embed + reindex。
	if err := s.republishFromApproved(ctx, full, userID); err != nil {
		return nil, err
	}
	full.Status = model.ArticleStatusPublished
	return full, nil
}

// UpdateAndRepublish 更新已发布文章正文并重新进入发布管道（agent 自迭代闭环）。
// 放开 Published 状态的编辑限制（UpdateArticle 仅允许 Draft/Rejected/Disabled）。
// 保持 Published 状态不变，直接重走 republishFromApproved（增量 reindex，复用 hash 跳过未变 chunk）。
func (s *KnowledgeService) UpdateAndRepublish(ctx context.Context, id int64, req request.UpdateArticleRequest, userID int64) error {
	article, err := s.findArticle(ctx, id)
	if err != nil {
		return err
	}
	// 标题变更检查唯一性（防 slug 冲突）。
	if req.Title != article.Title {
		if err := s.checkTitleUnique(ctx, article.KBID, req.Title, id); err != nil {
			return err
		}
	}
	// 更新正文字段（不回退状态，保持 Published）。
	article.Title = req.Title
	article.Content = req.Content
	article.WordCount = len([]rune(req.Content))
	article.Tags = marshalTags(req.Tags)
	if req.ArticleType != "" {
		article.ArticleType = req.ArticleType
	}
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章失败: " + err.Error()}
	}
	// 重传 draft/ 正文文件（republishFromApproved 从 draft/ 读取）。
	draftFile := articleFile(article.KBID, slugify(article.Title), false)
	s.uploadArticleFilesAsync(minioBucket, draftFile, formatArticleText(req.Title, req.Content), nil)
	// 重走发布管道：chunk→embed→ReplaceVectors（增量 reindex）。
	return s.republishFromApproved(ctx, article, userID)
}

// checkSemanticDuplicate 语义去重——embed 文章标题+首段，搜 KB 内已发布文章。
// 相似度 > 0.92 视为重复，返回冲突错误（提示 agent 用 update 而非 create）。
func (s *KnowledgeService) checkSemanticDuplicate(ctx context.Context, article *model.KnowledgeArticle) error {
	if s.embedder == nil || s.store == nil {
		return nil // RAG 未初始化时跳过语义去重（降级为仅标题去重）
	}
	// embed 标题 + 首段（取前 200 字符，控制 embedding 成本）。
	probe := article.Title
	if len([]rune(article.Content)) > 200 {
		probe += " " + string([]rune(article.Content)[:200])
	} else {
		probe += " " + article.Content
	}
	kb, err := s.repo.FindKBByID(ctx, article.KBID)
	if err != nil {
		return nil // KB 查询失败不阻塞创建（降级）
	}
	model := s.effectiveEmbeddingModel(kb.EmbeddingModel)
	vecs, _, err := s.embedder.Embed(ctx, []string{probe}, model)
	if err != nil || len(vecs) == 0 {
		slog.Warn("语义去重 embedding 失败，降级跳过", "article_id", article.ID, "error", err)
		return nil
	}
	results, err := s.store.CosineSearch(ctx, article.KBID, vecs[0], 5)
	if err != nil {
		slog.Warn("语义去重检索失败，降级跳过", "article_id", article.ID, "error", err)
		return nil
	}
	for _, r := range results {
		if r.ArticleID == article.ID {
			continue // 排除自身
		}
		if r.Score >= 0.92 {
			return errcode.AppError{Code: errcode.ErrConflict,
				Message: fmt.Sprintf("存在高度相似文章 (ID=%d, score=%.3f)，请用 kb(action=update) 更新而非新建", r.ArticleID, r.Score)}
		}
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

	// 停用时不删向量——搜索侧通过 status 过滤，保留以支持增量 embedding。
	// .md 单文件异步从 published 移回 draft，移动成功后更新 MinioPath（非阻塞；图片在全局 image/ 目录不受影响）。
	publishedFile := articleFile(article.KBID, slugify(article.Title), true)
	draftFile := articleFile(article.KBID, slugify(article.Title), false)
	go func() {
		if err := s.moveArticleFile(minioBucket, publishedFile, draftFile); err == nil {
			_ = s.repo.UpdateArticleMinioPath(context.Background(), id, minioBucket+"/"+draftFile)
		} else {
			slog.Warn("停用迁移文件失败", "article_id", id, "error", err)
		}
	}()

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
		ArticleType: a.ArticleType,
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
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "不支持的文件格式: " + fileType + "（支持: pdf/docx/xlsx/pptx/md/txt/jpg/png/gif/bmp/webp）"}
	}

	if fileSize > s.maxUploadSize {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: fmt.Sprintf("文件大小超过限制（最大 %d MB）", s.maxUploadSize/1024/1024)}
	}

	title := strings.TrimSuffix(filename, "."+fileType)
	if title == "" {
		title = filename
	}

	if err := s.checkTitleUnique(ctx, kbID, title, 0); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(content, s.maxUploadSize))
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

	fileKey := articleFile(kbID, slugify(article.Title), false)
	article.MinioPath = minioBucket + "/" + fileKey
	if err := s.repo.UpdateArticleMinioPath(ctx, article.ID, article.MinioPath); err != nil {
		slog.Warn("更新文章存储路径失败", "article_id", article.ID, "error", err)
	}
	s.uploadArticleFilesAsync(minioBucket, fileKey, formatArticleText(title, text), result.Images)

	// 文件上传后提工单标记人工复核（解析内容可能残缺，需人工确认）
	if s.ticketCreator != nil {
		articleID := article.ID
		if err := s.ticketCreator.CreateSystemTicket(ctx, request.SystemTicketRequest{
			Title:            fmt.Sprintf("【文档复核】%s", title),
			Description:      fmt.Sprintf("上传文件 %s 已解析为草稿文章 (ID=%d)，请人工复核内容完整性。", filename, articleID),
			Tags:             []string{"文档复核", "auto"},
			RelatedArticleID: &articleID,
			RelatedKBID:      &kbID,
			Reason:           "uploaded_document",
		}); err != nil {
			slog.Warn("文档复核工单创建失败", "article_id", articleID, "error", err)
		}
	}

	return article, nil
}

// GetUploadConfig 返回文档上传配置（大小上限、允许类型、文件数上限）。
func (s *KnowledgeService) GetUploadConfig() response.UploadConfigResponse {
	types := make([]string, 0, len(allowedDocumentTypes))
	for t := range allowedDocumentTypes {
		types = append(types, t)
	}
	sort.Strings(types)
	return response.UploadConfigResponse{
		MaxUploadSizeKB: int(s.maxUploadSize / 1024),
		MaxUploadSize:   s.maxUploadSize,
		AllowedTypes:    types,
		MaxFiles:        maxUploadFileCount,
	}
}

// UploadAsset 上传通用文件（文章内嵌图片/附件等）到 article-assets 目录，返回存储 key 与本地路径。
// 存储路径: {documents 桶}/article-assets/{uuid}{ext}。前端通过 /api/v1/admin/files/article-assets/{filename} 访问。
func (s *KnowledgeService) UploadAsset(ctx context.Context, filename string, contentType string, size int64, reader io.Reader) (string, error) {
	if s.storage == nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "存储未初始化"}
	}
	if size > s.maxUploadSize {
		return "", errcode.AppError{Code: errcode.ErrParam, Message: fmt.Sprintf("文件大小超过限制（最大 %d MB）", s.maxUploadSize/1024/1024)}
	}
	ext := filepath.Ext(filename)
	storedName := uuid.NewString() + ext
	if err := s.storage.UploadFile(ctx, minioBucket, "article-assets", storedName, reader, size, contentType); err != nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "文件上传失败: " + err.Error()}
	}
	return storedName, nil
}

// AssetLocalPath 返回 article-assets 目录下指定文件的本地路径（供下载代理 ServeFile）。
// MinIO 模式返回预签名 URL；Local 模式返回绝对路径。
func (s *KnowledgeService) AssetLocalPath(ctx context.Context, filename string) (string, error) {
	if s.storage == nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "存储未初始化"}
	}
	return s.storage.GetFileURL(ctx, minioBucket, "article-assets", filename)
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

// cleanupArticleFiles 异步清理文章的 .md 文件（草稿与已发布两态各一份，幂等）。
// 图片在全局 image/ 目录、跨文章共享，不在此清理（孤儿清理为独立任务）。
func (s *KnowledgeService) cleanupArticleFiles(article *model.KnowledgeArticle) {
	if s.storage == nil {
		return
	}
	bg := context.Background()
	deleteArticleFile(bg, s.storage, articleFileDir(article.KBID, false), articleFileName(slugify(article.Title)))
	deleteArticleFile(bg, s.storage, articleFileDir(article.KBID, true), articleFileName(slugify(article.Title)))
}

// uploadArticleFileSync 同步上传文章 md 文件——文件优先写，确保返回前文件已落盘（文件即真相）。
func (s *KnowledgeService) uploadArticleFileSync(bucket, fileKey, content string, images map[string][]byte) error {
	if s.storage == nil {
		return nil
	}
	bg := context.Background()
	dir, filename := pathutil.SplitFileKey(fileKey)
	if err := s.storage.UploadFile(bg, bucket, dir, filename, strings.NewReader(content), int64(len(content)), "text/markdown"); err != nil {
		return fmt.Errorf("同步上传文章失败: %w", err)
	}
	for name, data := range images {
		if err := s.storage.UploadFile(bg, bucket, imageDir, name, bytes.NewReader(data), int64(len(data)), imageContentType(name)); err != nil {
			slog.Warn("同步上传图片失败", "bucket", bucket, "name", name, "error", err)
		}
	}
	return nil
}

// uploadArticleFilesAsync 异步上传文章 md 文件（非关键路径用，如更新时的图片上传）。
func (s *KnowledgeService) uploadArticleFilesAsync(bucket, fileKey, content string, images map[string][]byte) {
	if s.storage == nil {
		return
	}
	go func() {
		bg := context.Background()
		// fileKey 形如 kb-{kbID}/{draft|published}/{slug}.md，拆出 dir 与 filename
		dir, filename := pathutil.SplitFileKey(fileKey)
		if err := s.storage.UploadFile(bg, bucket, dir, filename, strings.NewReader(content), int64(len(content)), "text/markdown"); err != nil {
			slog.Warn("异步上传文章失败", "bucket", bucket, "fileKey", fileKey, "error", err)
		}
		for name, data := range images {
			if err := s.storage.UploadFile(bg, bucket, imageDir, name, bytes.NewReader(data), int64(len(data)), imageContentType(name)); err != nil {
				slog.Warn("异步上传图片失败", "bucket", bucket, "name", name, "error", err)
			}
		}
	}()
}

// GetImageURL 返回全局图片目录下指定图片的访问 URL（本地路径或 MinIO 预签名 URL）。
// 图片与文章解耦，无需 articleId。
func (s *KnowledgeService) GetImageURL(ctx context.Context, filename string) (string, error) {
	if s.storage == nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "存储服务未初始化"}
	}
	return s.storage.GetFileURL(ctx, minioBucket, imageDir, filename)
}

// moveArticleFile 在同桶内将单个 .md 文件从 srcKey 移动到 dstKey（下载→上传→删除源）。
// 发布/停用切换状态时调用；同步执行，失败返回 error，调用方据此决定是否更新 MinioPath。
// 图片在全局 image/ 目录，不随发布移动。
func (s *KnowledgeService) moveArticleFile(bucket, srcKey, dstKey string) error {
	if s.storage == nil || bucket == "" || srcKey == "" || dstKey == "" {
		return nil
	}
	if srcKey == dstKey {
		return nil
	}
	bg := context.Background()
	srcDir, srcName := pathutil.SplitFileKey(srcKey)
	reader, err := s.storage.DownloadFile(bg, bucket, srcDir, srcName)
	if err != nil {
		slog.Warn("moveArticleFile 下载失败", "bucket", bucket, "src", srcKey, "error", err)
		return err
	}
	data, rErr := io.ReadAll(reader)
	reader.Close()
	if rErr != nil {
		slog.Warn("moveArticleFile 读取失败", "src", srcKey, "error", rErr)
		return rErr
	}
	dstDir, dstName := pathutil.SplitFileKey(dstKey)
	if err := s.storage.UploadFile(bg, bucket, dstDir, dstName, bytes.NewReader(data), int64(len(data)), "text/markdown"); err != nil {
		slog.Warn("moveArticleFile 上传失败", "dst", dstKey, "error", err)
		return err
	}
	if err := s.storage.DeleteFile(bg, bucket, srcDir, srcName); err != nil {
		slog.Warn("moveArticleFile 删除源失败", "src", srcKey, "error", err)
		return err
	}
	return nil
}

// deleteArticleFile 删除单个文章 .md 文件（幂等，文件不存在不报错）。
func deleteArticleFile(ctx context.Context, sc storage.StorageClient, dir, filename string) {
	if sc == nil || dir == "" || filename == "" {
		return
	}
	if err := sc.DeleteFile(ctx, minioBucket, dir, filename); err != nil {
		slog.Warn("删除文章文件失败", "dir", dir, "filename", filename, "error", err)
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

// RebuildBM25ForKB 从 DB 加载 KB 下所有已发布文章的分块和标签，重建 BM25 索引。
func RebuildBM25ForKB(repo *KnowledgeRepo, store adapter.VectorStore, bm25 *rag.BM25Retriever, kbID int64) {
	ctx := context.Background()

	// 查询该 KB 下所有已发布文章
	articles, _, err := repo.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 10000)
	if err != nil {
		slog.Warn("BM25 索引重建：查询文章列表失败", "kb_id", kbID, "error", err)
		return
	}

	var docs []rag.BM25Document
	for _, a := range articles {
		chunks, err := store.GetChunksByArticle(ctx, a.ID)
		if err != nil {
			slog.Warn("BM25 索引重建：查询分块失败", "article_id", a.ID, "error", err)
			continue
		}

		// 解析标签 JSONB → []string
		var tagList []string
		if len(a.Tags) > 0 {
			_ = json.Unmarshal(a.Tags, &tagList)
		}

		for _, c := range chunks {
			docs = append(docs, rag.BM25Document{
				ChunkID:     c.ID,
				ArticleID:   a.ID,
				KBID:        kbID,
				Content:     c.Content,
				ChunkIndex:  c.ChunkIndex,
				Tags:        tagList,
				Title:       a.Title,
				Source:      model.ArticleSourceTypeText(a.SourceType),
				ArticleType: a.ArticleType,
			})
		}
	}

	if len(docs) == 0 {
		// 无已发布文章 → 清空索引
		bm25.BuildIndex(kbID, nil)
		return
	}

	bm25.BuildIndex(kbID, docs)
	slog.Info("BM25 索引重建完成", "kb_id", kbID, "docs", len(docs))
}
