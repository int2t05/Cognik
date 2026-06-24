// Package service 实现知识库管理业务逻辑。
//
// KnowledgeService 统一管理知识库 CRUD、文章审核发布、pgvector 管道操作和文档上传。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/dto/request"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/rag"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MaxDocumentSize 文档上传大小上限（50MB）。
const MaxDocumentSize = 50 * 1024 * 1024

// allowedDocumentTypes 支持上传的文档格式白名单。
var allowedDocumentTypes = map[string]bool{"pdf": true, "docx": true, "md": true, "txt": true}

// MinIO 两桶模型：
//   - opsmind-documents：临时桶（原始文件、草稿正文、审核中各状态）
//   - opsmind-published：已发布桶（正文已嵌入 pgvector，RAG 可检索）
//
// 状态由 DB knowledge_articles.status 管理，MinIO 不按状态分桶。
const (
	minioBucketDocs      = "opsmind-documents"
	minioBucketPublished = "opsmind-published"
)

// articleContentKey 返回文章正文在 MinIO 中的 key（{标题}.txt）。
func articleContentKey(title string) string {
	return title + ".txt"
}

// 消费者接口——KnowledgeService 仅暴露它实际使用的依赖方法，
// 遵循 Go "accept interfaces, return structs" 惯例，便于测试 mock。
type knowledgeChunker interface {
	Split(text string) []string
}

type knowledgeEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, int, error)
}

type knowledgeDocParser interface {
	Parse(reader io.Reader, fileType string) (string, error)
}

// knowledgeRepo KnowledgeService 使用的仓库方法子集。
type knowledgeRepo interface {
	FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error)
	FindArticleByID(ctx context.Context, id int64) (*model.KnowledgeArticle, error)
	CreateArticle(ctx context.Context, article *model.KnowledgeArticle) error
	UpdateArticle(ctx context.Context, article *model.KnowledgeArticle) error
	UpdateArticleStatus(ctx context.Context, id int64, status int) error
	UpdateArticleStatusCAS(ctx context.Context, id int64, expectedOld, newStatus int) (int64, error)
	UpdateArticleProcessStatus(ctx context.Context, id int64, processStatus, processError string) error
	UpdateArticleMetrics(ctx context.Context, id int64, wordCount, chunkCount int) error
	CreateKB(ctx context.Context, kb *model.KnowledgeBase) error
	UpdateKB(ctx context.Context, kb *model.KnowledgeBase) error
	DeleteKB(ctx context.Context, id int64) error
	DeleteArticle(ctx context.Context, id int64) error
	ListKBs(ctx context.Context) ([]model.KnowledgeBase, error)
	CountArticlesByKB(ctx context.Context) (map[int64]int, error)
	ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, page, pageSize int) ([]model.KnowledgeArticle, int64, error)
	FindChunksByArticleID(ctx context.Context, articleID int64) ([]model.KnowledgeChunk, error)
}

// userNameResolver 按 ID 列表批量查询用户名称（仅 id + real_name）。
type userNameResolver interface {
	FindByIDs(ctx context.Context, ids []int64) ([]model.User, error)
}

// KnowledgeService 知识库管理服务。
//
// 所有依赖使用接口类型，便于测试 mock。
type KnowledgeService struct {
	repo      knowledgeRepo
	userNames userNameResolver
	chunker   knowledgeChunker
	embedder  knowledgeEmbedder
	store     adapter.VectorStore
	docParser knowledgeDocParser
	processor *rag.Processor
	storage   adapter.StorageClient
	auditRepo *repository.AuditRepo
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
func WithStorage(sc adapter.StorageClient) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.storage = sc }
}

// WithAuditRepo 设置审计日志仓库（Publish/Disable 时写入审计记录）。
func WithAuditRepo(ar *repository.AuditRepo) KnowledgeServiceOption {
	return func(s *KnowledgeService) { s.auditRepo = ar }
}

// NewKnowledgeService 创建 KnowledgeService 实例。
//
// repo 为必需参数（所有业务操作依赖数据访问）。
// 其余依赖通过可选函数注入——调用方只传非 nil 的依赖，
// 避免 8 位置参数的可读性问题。
//
// 示例：
//
//	// 生产环境全依赖
//	svc := NewKnowledgeService(repo,
//	    WithUserNames(userRepo), WithChunker(c), WithEmbedder(e),
//	    WithVectorStore(vs), WithDocParser(dp), WithProcessor(proc), WithStorage(sc))
//
//	// 测试环境仅 repo
//	svc := NewKnowledgeService(repo)
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

// CreateKB 创建知识库（仅写 PostgreSQL）。
func (s *KnowledgeService) CreateKB(ctx context.Context, req request.CreateKBRequest, userID int64) error {
	// 生成唯一 workspace slug，避免空字符串触发唯一索引冲突
	slug := strings.TrimSpace(req.Name)
	if slug == "" {
		slug = fmt.Sprintf("kb-%d", time.Now().UnixNano())
	}
	kb := &model.KnowledgeBase{
		Name:             req.Name,
		Description:      req.Description,
		RAGWorkspaceSlug: slug,
		EmbeddingModel:   req.EmbeddingModel,
		VectorDimension:  req.VectorDimension,
		LlmConfigID:      req.LlmConfigID,
		CreatedBy:        userID,
	}
	return s.repo.CreateKB(ctx, kb)
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
// 执行顺序：pgvector 向量删除 → 文章 + KB 数据库记录删除。
// 为什么先删向量再删数据库：VectorStore 删除可能失败（DB 连接问题），
// 如果先删数据库记录再失败则向量成为孤儿数据。
// MinIO 文件和 BM25 缓存由对应适配器异步管理，DeleteKB 仅负责 DB 和向量清理。
func (s *KnowledgeService) DeleteKB(ctx context.Context, id int64) error {
	// 1. 校验知识库存在
	_, err := s.repo.FindKBByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}

	// 2. 删除 pgvector 中该知识库的所有向量分块
	if s.store != nil {
		if err := s.store.DeleteByKB(ctx, id); err != nil {
			slog.Warn("删除知识库向量分块失败", "kb_id", id, "error", err)
			// 向量删除失败不阻塞数据库删除，由后续清理任务处理孤儿向量
		}
	}

	// 3. 级联删除文章和知识库
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

// CreateArticle 创建知识文章（草稿状态），返回创建后的文章（含自动生成的 ID）。
func (s *KnowledgeService) CreateArticle(ctx context.Context, req request.CreateArticleRequest, userID int64) (*model.KnowledgeArticle, error) {
	_, err := s.repo.FindKBByID(ctx, req.KBID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询知识库失败: " + err.Error()}
	}

	tagsJSON := marshalTags(req.Tags)
	sourceType := req.SourceType
	if sourceType == 0 {
		sourceType = 1 // 默认手动创建
	}
	article := &model.KnowledgeArticle{
		KBID:       req.KBID,
		Title:      req.Title,
		Content:    req.Content,
		SourceType: sourceType,
		Category:   req.Category,
		Tags:       tagsJSON,
		Status:     1, // 草稿
		CreatedBy:  userID,
		WordCount:  len([]rune(req.Content)),
	}
	if err := s.repo.CreateArticle(ctx, article); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	// 异步上传正文到 MinIO 临时桶
	key := articleContentKey(article.Title)
	article.MinioPath = minioBucketDocs + "/" + key
	_ = s.repo.UpdateArticle(ctx, article)
	s.uploadMinioAsync(minioBucketDocs, key, req.Content)

	return article, nil
}

// UpdateArticle 更新文章（仅草稿/驳回状态可编辑）。
func (s *KnowledgeService) UpdateArticle(ctx context.Context, id int64, req request.UpdateArticleRequest) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusDraft && article.Status != model.ArticleStatusRejected {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿和驳回状态可编辑"}
	}

	oldTitle := article.Title
	article.Title = req.Title
	article.Content = req.Content
	article.Category = req.Category
	article.Tags = marshalTags(req.Tags)
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章失败: " + err.Error()}
	}

	// MinIO：标题变更时同步更新 key，旧文件成为垃圾（异步清理成本高于留存）
	if article.MinioPath != "" {
		bucket, key := splitMinioPath(article.MinioPath)
		if key == articleContentKey(oldTitle) {
			// 手动创建的文章，key 跟随标题
			newKey := articleContentKey(req.Title)
			article.MinioPath = bucket + "/" + newKey
			_ = s.repo.UpdateArticle(ctx, article)
			s.uploadMinioAsync(bucket, newKey, req.Content)
		} else {
			// 文档上传等非标准路径，原地覆盖
			s.uploadMinioAsync(bucket, key, req.Content)
		}
	}

	return nil
}

// SubmitReview 提交审核（草稿→待审核）。
func (s *KnowledgeService) SubmitReview(ctx context.Context, id int64, userID int64) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusDraft {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿状态可提交审核"}
	}

	return s.repo.UpdateArticleStatus(ctx, id, int(model.ArticleStatusReviewing))
}

// Review 审核文章（待审核→已通过/已驳回）。
func (s *KnowledgeService) Review(ctx context.Context, id int64, reviewerID int64, req request.ReviewRequest) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusReviewing {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅待审核状态可审核"}
	}
	// MVP 阶段允许创建人自审核（admin 用户可走全流程：提交→审核→发布），
	// 后续如需强制分离，可通过 RBAC 权限或配置开关控制。
	if req.Approved {
		article.Status = model.ArticleStatusApproved
		article.ReviewedBy = &reviewerID
		return s.repo.UpdateArticle(ctx, article)
	}
	if strings.TrimSpace(req.ReviewComment) == "" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "驳回时必须填写审核意见"}
	}
	article.Status = model.ArticleStatusRejected
	article.ReviewComment = req.ReviewComment
	article.ReviewedBy = &reviewerID
	return s.repo.UpdateArticle(ctx, article)
}

// =============================================================================
// Publish / Disable / Enable
// =============================================================================

// Publish 发布文章——分块→embedding→pgvector 写入。
//
// ctx 由调用方传入（Handler 传 c.Request.Context()），确保发布过程可被取消/超时。
//
// 使用 CAS（Compare-And-Swap）防止并发重复发布：
// 仅当 status == Approved(3) 时才更新为 Published(4)，否则返回冲突。
func (s *KnowledgeService) Publish(ctx context.Context, id int64, publisherID int64) error {
	if s.chunker == nil || s.embedder == nil || s.store == nil {
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "RAG 管道未初始化（chunker/embedder/store 为空）"}
	}

	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusApproved {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已审核通过的文章可发布"}
	}

	// 原子抢占：CAS 更新成功才继续，防止并发重复发布
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

// republishFromApproved 将文章正文保存到 MinIO，入队异步处理（分块→embedding→pgvector）。
//
// 为什么改为异步：同步 embedding 阻塞 HTTP 请求长达数秒甚至数十秒，
// 改为 MinIO→队列→Worker 模式后，发布接口立即返回，Worker 后台消费。
//
// 由 Publish（Approved → Published）和 Enable（Disabled → Published）共用。
func (s *KnowledgeService) republishFromApproved(ctx context.Context, article *model.KnowledgeArticle, publisherID int64) error {
	if s.chunker == nil || s.embedder == nil || s.store == nil {
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "RAG 服务未初始化（缺少 Chunker/Embedding/VectorStore）"}
	}
	if s.processor == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "异步处理器未初始化"}
	}

	id := article.ID
	content := article.Content
	if strings.TrimSpace(content) == "" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "文章内容为空，无法发布"}
	}

	// 从临时桶拷贝原始文件到已发布桶（保持原名，不转换格式）
	pubKey := s.copyToPublished(article)
	article.MinioPath = minioBucketPublished + "/" + pubKey

	// 更新文章状态 + 清除失败残留
	article.Status = model.ArticleStatusPublished
	article.PublishedBy = &publisherID
	article.ProcessStatus = "processing"
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}

	// 提交异步处理任务
	task := rag.ProcessTask{
		ArticleID:      id,
		KBID:           article.KBID,
		Content:        content,
		EmbeddingModel: article.KnowledgeBase.EmbeddingModel,
		OnStatusChange: func(aID int64, status, errMsg string) { s.onPublishComplete(ctx, aID, status, errMsg) },
		OnMetrics:      func(aID int64, wordCount, chunkCount int) { s.onProcessMetrics(ctx, aID, wordCount, chunkCount) },
	}
	if err := s.processor.Submit(task); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "提交处理任务失败: " + err.Error()}
	}

	// 审计：发布文章
	if s.auditRepo != nil {
		s.auditRepo.Create(ctx, &model.AuditLog{
			OperatorID: publisherID, Action: "knowledge.publish",
			TargetType: "knowledge_article", TargetID: id,
		})
	}
	return nil
}

// Disable 停用文章——从 pgvector 删除向量并更新状态。
//
// 状态机：仅 Published → Disabled。停用前必须先经过审核发布流程，
// 草稿/待审核/审核通过/已驳回状态不应直接 Disable（应通过驳回或回退路径处理）。
func (s *KnowledgeService) Disable(ctx context.Context, id int64, operatorID int64) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusPublished {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已发布状态可停用"}
	}

	if s.store != nil {
		if err := s.store.DeleteByArticle(ctx, id); err != nil {
			return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "删除向量失败: " + err.Error()}
		}
	}

	// 异步清理已发布桶中的文件
	go func() {
		s.deleteMinioFile(context.Background(), minioBucketPublished, articleContentKey(article.Title))
		if article.MinioPath != "" {
			b, k := splitMinioPath(article.MinioPath)
			s.deleteMinioFile(context.Background(), b, k)
		}
	}()

	article.Status = model.ArticleStatusDisabled
	if err := s.repo.UpdateArticle(ctx, article); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新文章状态失败: " + err.Error()}
	}
	// 审计：停用文章
	if s.auditRepo != nil {
		s.auditRepo.Create(ctx, &model.AuditLog{
			OperatorID: operatorID, Action: "knowledge.disable",
			TargetType: "knowledge_article", TargetID: id,
		})
	}
	return nil
}

// Enable 启用已停用文章——重新执行分块→embedding→pgvector 写入并发布。
//
// 状态机：仅 Disabled → Published。停用时向量已删除，启用必须重建向量。
// 复用 Publish 内部状态机校验之外的逻辑：状态校验在本函数入口完成，
// 剩余分块/embedding/写入路径与 Publish 共用。
func (s *KnowledgeService) Enable(ctx context.Context, id int64, publisherID int64) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}
	if article.Status != model.ArticleStatusDisabled {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已停用状态的文章可启用"}
	}
	// 直接走发布管道；绕开 Publish 的状态校验（已由本函数完成）
	article.Status = model.ArticleStatusApproved
	return s.republishFromApproved(ctx, article, publisherID)
}

// DeleteArticle 删除文章（任意状态均可删除，同时清理关联向量和 MinIO 文件）。
func (s *KnowledgeService) DeleteArticle(ctx context.Context, id int64) error {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}

	// MinIO 清理异步执行，不阻塞 DB 删除和 HTTP 响应
	go s.cleanupArticleFiles(article)

	return s.repo.DeleteArticle(ctx, id)
}
// =============================================================================
// List / Detail
// =============================================================================

// ListArticles 分页查询文章列表，支持按 sourceType/processStatus 筛选。
func (s *KnowledgeService) ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, page, pageSize int) (*response.ArticleListResponse, error) {
	articles, total, err := s.repo.ListArticles(ctx, kbID, status, sourceType, processStatus, page, pageSize)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章列表失败: " + err.Error()}
	}

	// 批量获取用户名，避免 N+1
	userNames := s.resolveUserNames(ctx, articles)

	result := make([]response.ArticleResponse, len(articles))
	for i, a := range articles {
		result[i] = response.ArticleResponse{
			ID:              a.ID,
			KBID:            a.KBID,
			KBName:          a.KnowledgeBase.Name,
			Title:           a.Title,
			Content:         a.Content,
			Category:        a.Category,
			Tags:            unmarshalTags(a.Tags),
			Status:          a.Status,
			StatusText:      model.ArticleStatusText(a.Status),
			SourceType:      a.SourceType,
			SourceTypeText:  model.ArticleSourceTypeText(a.SourceType),
			FileType:        a.FileType,
			MinioPath:       a.MinioPath,
			WordCount:       a.WordCount,
			ChunkCount:      a.ChunkCount,
			ProcessStatus:   a.ProcessStatus,
			ProcessError:    a.ProcessError,
			CreatedBy:       a.CreatedBy,
			CreatedByName:   userNames[a.CreatedBy],
			ReviewedBy:      a.ReviewedBy,
			PublishedBy:     a.PublishedBy,
			PublishedByName: userNames[ptrVal(a.PublishedBy)],
			ReviewComment:   a.ReviewComment,
			CreatedAt:       a.CreatedAt,
			UpdatedAt:       a.UpdatedAt,
		}
	}

	return &response.ArticleListResponse{
		Articles: result,
		Total:    total,
	}, nil
}

// GetArticleDetail 获取文章详情（含切片）。
func (s *KnowledgeService) GetArticleDetail(ctx context.Context, id int64) (*response.ArticleDetailResponse, error) {
	article, err := s.repo.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询文章失败: " + err.Error()}
	}

	chunks, err := s.repo.FindChunksByArticleID(ctx, id)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "查询切片失败: " + err.Error()}
	}

	userNames := s.resolveUserNames(ctx, []model.KnowledgeArticle{*article})

	chunkResponses := make([]response.ChunkResponse, len(chunks))
	for i, c := range chunks {
		chunkResponses[i] = response.ChunkResponse{
			ID:              c.ID,
			KBID:            c.KBID,
			Content:         c.Content,
			ChunkIndex:      c.ChunkIndex,
			EmbeddingModel:  c.EmbeddingModel,
			VectorDimension: c.VectorDimension,
			CreatedAt:       c.CreatedAt,
		}
	}

	return &response.ArticleDetailResponse{
		ArticleResponse: response.ArticleResponse{
			ID:              article.ID,
			KBID:            article.KBID,
			KBName:          article.KnowledgeBase.Name,
			Title:           article.Title,
			Content:         article.Content,
			Category:        article.Category,
			Tags:            unmarshalTags(article.Tags),
			Status:          article.Status,
			StatusText:      model.ArticleStatusText(article.Status),
			SourceType:      article.SourceType,
			SourceTypeText:  model.ArticleSourceTypeText(article.SourceType),
			FileType:        article.FileType,
			MinioPath:       article.MinioPath,
			WordCount:       article.WordCount,
			ChunkCount:      article.ChunkCount,
			ProcessStatus:   article.ProcessStatus,
			ProcessError:    article.ProcessError,
			CreatedBy:       article.CreatedBy,
			CreatedByName:   userNames[article.CreatedBy],
			ReviewedBy:      article.ReviewedBy,
			PublishedBy:     article.PublishedBy,
			PublishedByName: userNames[ptrVal(article.PublishedBy)],
			ReviewComment:   article.ReviewComment,
			CreatedAt:       article.CreatedAt,
			UpdatedAt:       article.UpdatedAt,
		},
		Chunks: chunkResponses,
	}, nil
}

// =============================================================================
// 文档上传与处理
// =============================================================================

// UploadDocuments 上传文档到知识库（解析→创建文章）。
//
// 文件名去掉后缀作为文章标题，正文由后端同步解析后返回前端。
// 分块→embedding→pgvector 不在此阶段执行——
// 统一推迟到 Publish（发布）环节，避免草稿文章向量污染检索结果和重复 embed。
func (s *KnowledgeService) UploadDocuments(ctx context.Context, kbID int64, userID int64, filename string, fileType string, fileSize int64, content io.Reader) (*model.KnowledgeArticle, error) {
	_, err := s.repo.FindKBByID(ctx, kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return nil, err
	}

	if !allowedDocumentTypes[fileType] {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "不支持的文件格式: " + fileType + "（支持: pdf/docx/md/txt）"}
	}

	if fileSize > MaxDocumentSize {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件大小超过限制（最大 50MB）"}
	}

	// 解析标题：文件名去掉扩展名
	title := strings.TrimSuffix(filename, "."+fileType)
	if title == "" {
		title = filename
	}

	// 读取文件内容到内存
	data, err := io.ReadAll(io.LimitReader(content, MaxDocumentSize))
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "读取上传文件失败: " + err.Error()}
	}
	if len(data) == 0 {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件内容为空"}
	}

	// 同步解析文档正文
	if s.docParser == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "文档解析器未初始化"}
	}
	text, err := s.docParser.Parse(bytes.NewReader(data), fileType)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档解析失败: " + err.Error()}
	}
	if strings.TrimSpace(text) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档内容为空"}
	}

	article := &model.KnowledgeArticle{
		KBID:      kbID,
		Title:     title,
		Content:   text,
		Category:  "文档上传",
		FileType:  fileType,
		WordCount: len([]rune(text)),
		Status:    1,
		CreatedBy: userID,
	}

	if err := s.repo.CreateArticle(ctx, article); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	// 原始文件异步写入 MinIO（解析后正文在 DB，不额外存一份）
	if s.storage != nil {
		rawKey := fmt.Sprintf("documents/%d_%s", time.Now().UnixNano(), filename)
		article.MinioPath = minioBucketDocs + "/" + rawKey
		_ = s.repo.UpdateArticle(ctx, article)
		rawData := make([]byte, len(data))
		copy(rawData, data)
		go func() {
			if _, err := s.storage.Upload(context.Background(), minioBucketDocs, rawKey, bytes.NewReader(rawData), int64(len(rawData)), ""); err != nil {
				slog.Warn("原始文件上传 MinIO 失败", "key", rawKey, "error", err)
			}
		}()
	}

	return article, nil
}

// GetDocumentStatus 查询文档处理状态。
//
// kbID 用于校验 URL 资源层级一致性——文章必须属于指定知识库，否则返回 ErrNotFound。
func (s *KnowledgeService) GetDocumentStatus(ctx context.Context, kbID int64, articleID int64) (*response.DocumentStatusResponse, error) {
	article, err := s.repo.FindArticleByID(ctx, articleID)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
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

// RetryDocument 重试文档处理（重新入队）。
//
// kbID 用于校验 URL 资源层级一致性——文章必须属于指定知识库。
// 状态机：仅允许 ProcessStatus == "failed" 的文章重试（避免对已成功或处理中文档重复入队）。
// 重试不修改 Article.Status（审核状态），仅清空 ProcessError 让前端重新展示错误。
func (s *KnowledgeService) RetryDocument(ctx context.Context, kbID int64, articleID int64) error {
	article, err := s.repo.FindArticleByID(ctx, articleID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
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

	// 重置 process_status 为 pending，processor 在新一轮处理开始时会再覆盖
	if err := s.repo.UpdateArticleProcessStatus(ctx, articleID, "pending", ""); err != nil {
		slog.Warn("重置处理状态失败，不阻断主流程", "article_id", articleID, "error", err)
	}
	task := rag.ProcessTask{
		ArticleID:      articleID,
		KBID:           article.KBID,
		Content:        article.Content,
		EmbeddingModel: article.KnowledgeBase.EmbeddingModel,
		OnStatusChange: func(aID int64, status, errMsg string) { s.onProcessStatusChange(ctx, aID, status, errMsg) },
		OnMetrics:      func(aID int64, wordCount, chunkCount int) { s.onProcessMetrics(ctx, aID, wordCount, chunkCount) },
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

// onPublishComplete 发布异步处理完成回调——成功时清除 processing 状态，失败时记录原因。
func (s *KnowledgeService) onPublishComplete(ctx context.Context, aID int64, status, errMsg string) {
	if status == "completed" {
		_ = s.repo.UpdateArticleProcessStatus(ctx, aID, "completed", "")
	} else {
		_ = s.repo.UpdateArticleProcessStatus(ctx, aID, "failed", errMsg)
	}
}

// onProcessMetrics 更新文档指标（字数/分块数），失败时记录日志但不阻塞流程。
func (s *KnowledgeService) onProcessMetrics(ctx context.Context, aID int64, wordCount, chunkCount int) {
	if err := s.repo.UpdateArticleMetrics(ctx, aID, wordCount, chunkCount); err != nil {
		slog.Warn("更新文档指标失败", "article_id", aID, "error", err)
	}
}

// resolveUserNames 批量解析文章关联的用户名（创建人 + 发布人）。
//
// 为什么一次查询而非每个文章单独查：列表页 20 篇文章会产生 ~40 次用户查询，
// 批量去重后一次搞定，避免 N+1 往返。
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

// splitMinioPath 将 "bucket/key" 格式的 MinioPath 拆分为 bucket 和 key。
func splitMinioPath(path string) (string, string) {
	idx := strings.Index(path, "/")
	if idx < 0 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}

// mapArticleToProcessStatus 返回文章的处理状态字符串。
//
// 仅读取 ProcessStatus 字段，不再从 Status 反推（历史兼容逻辑已删除）。
// ProcessStatus 取值见 rag.Processor 文档：pending/chunking/embedding/indexing/completed/failed/disabled。
func mapArticleToProcessStatus(article *model.KnowledgeArticle) string {
	if article.ProcessStatus != "" {
		return article.ProcessStatus
	}
	return "pending"
}

// cleanupArticleFiles 异步清理文章的 MinIO 文件。
func (s *KnowledgeService) cleanupArticleFiles(article *model.KnowledgeArticle) {
	if s.storage == nil {
		return
	}
	bg := context.Background()

	// 标准路径 + 实际 MinioPath（幂等删除，重复调用无害）
	s.deleteMinioFile(bg, minioBucketDocs, articleContentKey(article.Title))
	s.deleteMinioFile(bg, minioBucketPublished, articleContentKey(article.Title))
	if article.MinioPath != "" {
		b, k := splitMinioPath(article.MinioPath)
		s.deleteMinioFile(bg, b, k)
	}
}

// uploadMinioAsync 异步上传内容到 MinIO，不阻塞主流程。
func (s *KnowledgeService) uploadMinioAsync(bucket, key, content string) {
	if s.storage == nil {
		return
	}
	go func() {
		if _, err := s.storage.Upload(context.Background(), bucket, key, strings.NewReader(content), int64(len(content)), "text/plain; charset=utf-8"); err != nil {
			slog.Warn("异步上传 MinIO 失败", "bucket", bucket, "key", key, "error", err)
		}
	}()
}

// copyToPublished 从临时桶拷贝文件到已发布桶（异步，保持原名和格式）。
//
// 优先从 article.MinioPath 定位源文件；MinioPath 为空或下载失败时回退到
// article.Content 写入 {标题}.txt。
func (s *KnowledgeService) copyToPublished(article *model.KnowledgeArticle) string {
	if s.storage == nil {
		return articleContentKey(article.Title)
	}

	// 确定源 key：从 MinioPath 提取，空则回退到 {标题}.txt
	var srcKey string
	if article.MinioPath != "" {
		_, srcKey = splitMinioPath(article.MinioPath)
	}
	if srcKey == "" {
		srcKey = articleContentKey(article.Title)
	}

	content := article.Content
	bg := context.Background()

	// 尝试从 docs 桶下载原始文件
	if reader, err := s.storage.Download(bg, minioBucketDocs, srcKey); err == nil {
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		if len(data) > 0 {
			go func() {
				if _, err := s.storage.Upload(bg, minioBucketPublished, srcKey, bytes.NewReader(data), int64(len(data)), ""); err != nil {
					slog.Warn("拷贝文件到已发布桶失败", "key", srcKey, "error", err)
				}
			}()
			return srcKey
		}
	}

	// 回退：用 DB 中的正文写入 {标题}.txt
	fallbackKey := articleContentKey(article.Title)
	s.uploadMinioAsync(minioBucketPublished, fallbackKey, content)
	return fallbackKey
}

// deleteMinioFile 安全删除 MinIO 文件（bucket/key 为空或 storage 为 nil 时静默跳过）。
func (s *KnowledgeService) deleteMinioFile(ctx context.Context, bucket, key string) {
	if s.storage == nil || bucket == "" || key == "" {
		return
	}
	if err := s.storage.Delete(ctx, bucket, key); err != nil {
		slog.Warn("删除 MinIO 文件失败", "bucket", bucket, "key", key, "error", err)
	}
}
