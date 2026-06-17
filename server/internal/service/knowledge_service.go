// Package service 实现知识库管理业务逻辑。
//
// KnowledgeService 统一管理知识库 CRUD、文章审核发布、pgvector 管道操作和文档上传。
package service

import (
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
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
	FindKBByID(id int64) (*model.KnowledgeBase, error)
	FindArticleByID(id int64) (*model.KnowledgeArticle, error)
	CreateArticle(article *model.KnowledgeArticle) error
	UpdateArticle(article *model.KnowledgeArticle) error
	UpdateArticleStatus(id int64, status int) error
	UpdateArticleProcessStatus(id int64, processStatus, processError string) error
	UpdateArticleMetrics(id int64, wordCount, chunkCount int) error
	CreateKB(kb *model.KnowledgeBase) error
	UpdateKB(kb *model.KnowledgeBase) error
	DeleteKB(id int64) error
	ListKBs() ([]model.KnowledgeBase, error)
	ListArticles(kbID int64, status int, page, pageSize int) ([]model.KnowledgeArticle, int64, error)
	FindChunksByArticleID(articleID int64) ([]model.KnowledgeChunk, error)
}

// KnowledgeService 知识库管理服务。
//
// 所有依赖使用接口类型，便于测试 mock。
type KnowledgeService struct {
	repo      knowledgeRepo
	chunker   knowledgeChunker
	embedder  knowledgeEmbedder
	store     adapter.VectorStore
	docParser knowledgeDocParser
	processor *rag.Processor
	storage   adapter.StorageClient // MinIO 文档对象存储
}

// NewKnowledgeService 创建 KnowledgeService 实例。
//
// repo/chunker/embedder/store/docParser/processor 可以为 nil（测试或部分功能不需要时）。
// 直接使用具体接口类型，编译期校验传入类型。
func NewKnowledgeService(repo knowledgeRepo, chunker knowledgeChunker, embedder knowledgeEmbedder, store adapter.VectorStore, docParser knowledgeDocParser, processor *rag.Processor, storage adapter.StorageClient) *KnowledgeService {
	return &KnowledgeService{
		repo:      repo,
		chunker:   chunker,
		embedder:  embedder,
		store:     store,
		docParser: docParser,
		processor: processor,
		storage:   storage,
	}
}

// =============================================================================
// KnowledgeBase
// =============================================================================

// CreateKB 创建知识库（仅写 PostgreSQL）。
func (s *KnowledgeService) CreateKB(req request.CreateKBRequest, userID int64) error {
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
		CreatedBy:        userID,
	}
	return s.repo.CreateKB(kb)
}

// UpdateKB 更新知识库信息。
func (s *KnowledgeService) UpdateKB(id int64, req request.UpdateKBRequest) error {
	kb, err := s.repo.FindKBByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return err
	}
	kb.Name = req.Name
	kb.Description = req.Description
	if req.EmbeddingModel != "" {
		kb.EmbeddingModel = req.EmbeddingModel
	}
	if req.VectorDimension > 0 {
		kb.VectorDimension = req.VectorDimension
	}
	return s.repo.UpdateKB(kb)
}

// DeleteKB 删除知识库及其下所有内容。
//
// 执行顺序：pgvector 向量删除 → 文章 + KB 数据库记录删除。
// 为什么先删向量再删数据库：VectorStore 删除可能失败（DB 连接问题），
// 如果先删数据库记录再失败则向量成为孤儿数据。
// MinIO 文档文件和 BM25 缓存在后续迭代中完善。
func (s *KnowledgeService) DeleteKB(id int64) error {
	// 1. 校验知识库存在
	_, err := s.repo.FindKBByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return err
	}

	// 2. 删除 pgvector 中该知识库的所有向量分块
	if s.store != nil {
		if err := s.store.DeleteByKB(context.Background(), id); err != nil {
			slog.Warn("删除知识库向量分块失败", "kb_id", id, "error", err)
			// 向量删除失败不阻塞数据库删除，由后续清理任务处理孤儿向量
		}
	}

	// 3. 级联删除文章和知识库
	return s.repo.DeleteKB(id)
}

// ListKBs 列出全部知识库。
func (s *KnowledgeService) ListKBs() ([]response.KBResponse, error) {
	kbs, err := s.repo.ListKBs()
	if err != nil {
		return nil, err
	}
	result := make([]response.KBResponse, len(kbs))
	for i, kb := range kbs {
		result[i] = response.KBResponse{
			ID:              kb.ID,
			Name:            kb.Name,
			Description:     kb.Description,
			EmbeddingModel:  kb.EmbeddingModel,
			VectorDimension: kb.VectorDimension,
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

// CreateArticle 创建知识文章（草稿状态）。
func (s *KnowledgeService) CreateArticle(req request.CreateArticleRequest, userID int64) error {
	_, err := s.repo.FindKBByID(req.KBID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
		return err
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
	return s.repo.CreateArticle(article)
}

// UpdateArticle 更新文章（仅草稿/驳回状态可编辑）。
func (s *KnowledgeService) UpdateArticle(id int64, req request.UpdateArticleRequest, userID int64) error {
	// TODO(service/knowledge): userID 参数当前未使用。
	// 如果需要审计或作者权限校验，应在这里使用；否则从签名移除避免误导调用方。
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusDraft && article.Status != model.ArticleStatusRejected {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿和驳回状态可编辑"}
	}
	article.Title = req.Title
	article.Content = req.Content
	article.Category = req.Category
	article.Tags = marshalTags(req.Tags)
	return s.repo.UpdateArticle(article)
}

// SubmitReview 提交审核（草稿→待审核）。
func (s *KnowledgeService) SubmitReview(id int64, userID int64) error {
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusDraft {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅草稿状态可提交审核"}
	}
	return s.repo.UpdateArticleStatus(id, int(model.ArticleStatusReviewing))
}

// Review 审核文章（待审核→已通过/已驳回）。
func (s *KnowledgeService) Review(id int64, reviewerID int64, req request.ReviewRequest) error {
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusReviewing {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅待审核状态可审核"}
	}
	if article.CreatedBy == reviewerID {
		return errcode.AppError{Code: errcode.ErrParam, Message: "审核人不能是创建人"}
	}
	if req.Approved {
		article.Status = model.ArticleStatusApproved
		article.ReviewedBy = &reviewerID
		return s.repo.UpdateArticle(article)
	}
	if strings.TrimSpace(req.ReviewComment) == "" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "驳回时必须填写审核意见"}
	}
	article.Status = model.ArticleStatusRejected
	article.ReviewComment = req.ReviewComment
	article.ReviewedBy = &reviewerID
	return s.repo.UpdateArticle(article)
}

// =============================================================================
// Publish / Disable / Enable
// =============================================================================

// Publish 发布文章——分块→embedding→pgvector 写入。
//
// ctx 由调用方传入（Handler 传 c.Request.Context()），确保发布过程可被取消/超时。
//
// 流程：
//  1. 校验管道组件非空（否则返回 ErrRAGUnavailable）
//  2. 校验状态（仅审核通过 status=3 可发布）
//  3. 调用 republishFromApproved 执行分块→embedding→pgvector 写入
func (s *KnowledgeService) Publish(ctx context.Context, id int64, publisherID int64) error {
	if s.chunker == nil || s.embedder == nil || s.store == nil {
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "RAG 管道未初始化（chunker/embedder/store 为空）"}
	}

	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusApproved {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已审核通过的文章可发布"}
	}
	return s.republishFromApproved(ctx, article, publisherID)
}

// republishFromApproved 从已审核通过状态执行发布管道——分块→embedding→pgvector。
//
// 流程：
//  1. Chunker.Split → 文本分块
//  2. Embedder.Embed → 生成向量
//  3. VectorStore.BatchInsert → 写入新向量（失败时设置 process_status=failed）
//  4. VectorStore.DeleteByArticle → 清除旧向量（仅新向量写入成功后执行）
//  5. 更新文章状态为已发布 status=4
//
// 为什么先写新向量再删旧向量：
// 新旧向量的写入和删除不在同一事务中（pgvector 不支持 GORM 事务），
// 先写后删保证：写入失败时旧向量仍在（文章仍可被检索），
// 删除失败时旧向量残留但新向量有效（检索会返回新旧混合结果，优于全部丢失）。
//
// 由 Publish（Approved → Published）和 Enable（Disabled → Published）共用。
func (s *KnowledgeService) republishFromApproved(ctx context.Context, article *model.KnowledgeArticle, publisherID int64) error {
	id := article.ID

	// Step 1: 分块
	content := article.Content
	chunks := s.chunker.Split(content)
	if len(chunks) == 0 {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "分块结果为空"}
	}
	article.WordCount = len([]rune(content))
	article.ChunkCount = len(chunks)

	// Step 2: Embedding
	vectors, dimension, err := s.embedder.Embed(ctx, chunks)
	if err != nil {
		s.recordPublishFailure(article, "生成向量失败: "+err.Error())
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "生成向量失败: " + err.Error()}
	}
	if len(vectors) != len(chunks) {
		s.recordPublishFailure(article, fmt.Sprintf("向量数与分块数不匹配: %d vs %d", len(vectors), len(chunks)))
		return errcode.AppError{Code: errcode.ErrUnknown, Message: fmt.Sprintf("向量数与分块数不匹配: %d vs %d", len(vectors), len(chunks))}
	}

	// Step 3: 先写入新向量（失败时旧向量仍在，文章仍可检索）
	vc := make([]adapter.VectorChunk, len(chunks))
	for i, chunk := range chunks {
		vc[i] = adapter.VectorChunk{
			ArticleID:       id,
			KBID:            article.KBID,
			Content:         chunk,
			ChunkIndex:      i,
			Embedding:       vectors[i],
			EmbeddingModel:  article.KnowledgeBase.EmbeddingModel,
			VectorDimension: dimension,
		}
	}
	if err := s.store.BatchInsert(ctx, vc); err != nil {
		s.recordPublishFailure(article, "写入向量失败: "+err.Error())
		return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "写入向量失败: " + err.Error()}
	}

	// Step 4: 新向量写入成功后再清除旧向量（幂等——无旧向量也不报错）
	if err := s.store.DeleteByArticle(ctx, id); err != nil {
		// 旧向量删除失败不阻塞发布——新向量已生效，旧向量残留可被后续清理
		slog.Warn("发布时清除旧向量失败（新向量已写入，旧向量残留）", "article_id", id, "error", err)
	}

	// Step 5: 更新状态
	article.Status = model.ArticleStatusPublished
	article.PublishedBy = &publisherID
	return s.repo.UpdateArticle(article)
}

// recordPublishFailure 持久化发布失败状态和原因，供前端展示和重试。
//
// 为什么 publish 失败要写 process_status：
// 文章停留在"审核通过"状态时，用户无法区分"还没发布"和"发布失败"。
// process_status=failed + process_error 让前端可展示失败原因并提供重试按钮。
func (s *KnowledgeService) recordPublishFailure(article *model.KnowledgeArticle, errMsg string) {
	if err := s.repo.UpdateArticleProcessStatus(article.ID, "failed", errMsg); err != nil {
		slog.Warn("记录发布失败状态时出错", "article_id", article.ID, "error", err)
	}
}

// Disable 停用文章——从 pgvector 删除向量并更新状态。
//
// 状态机：仅 Published → Disabled。停用前必须先经过审核发布流程，
// 草稿/待审核/审核通过/已驳回状态不应直接 Disable（应通过驳回或回退路径处理）。
func (s *KnowledgeService) Disable(ctx context.Context, id int64) error {
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusPublished {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已发布状态可停用"}
	}

	if s.store != nil {
		if err := s.store.DeleteByArticle(ctx, id); err != nil {
			return errcode.AppError{Code: errcode.ErrRAGUnavailable, Message: "删除向量失败: " + err.Error()}
		}
	}

	article.Status = model.ArticleStatusDisabled
	return s.repo.UpdateArticle(article)
}

// Enable 启用已停用文章——重新执行分块→embedding→pgvector 写入并发布。
//
// 状态机：仅 Disabled → Published。停用时向量已删除，启用必须重建向量。
// 复用 Publish 内部状态机校验之外的逻辑：状态校验在本函数入口完成，
// 剩余分块/embedding/写入路径与 Publish 共用。
func (s *KnowledgeService) Enable(ctx context.Context, id int64, publisherID int64) error {
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return err
	}
	if article.Status != model.ArticleStatusDisabled {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅已停用状态的文章可启用"}
	}
	// 直接走发布管道；绕开 Publish 的状态校验（已由本函数完成）
	article.Status = model.ArticleStatusApproved
	return s.republishFromApproved(ctx, article, publisherID)
}

// =============================================================================
// List / Detail
// =============================================================================

// ListArticles 分页查询文章列表。
func (s *KnowledgeService) ListArticles(kbID int64, status int, page, pageSize int) (*response.ArticleListResponse, error) {
	// TODO(service/knowledge): source_type/process_status 筛选未实现。
	// 调用方可能传这些参数，后端忽略会导致列表筛选失效。
	articles, total, err := s.repo.ListArticles(kbID, status, page, pageSize)
	if err != nil {
		return nil, err
	}

	result := make([]response.ArticleResponse, len(articles))
	for i, a := range articles {
		result[i] = response.ArticleResponse{
			ID:            a.ID,
			KBID:          a.KBID,
			KBName:        a.KnowledgeBase.Name,
			Title:         a.Title,
			Content:       a.Content,
			Category:      a.Category,
			Tags:          unmarshalTags(a.Tags),
			Status:        a.Status,
			StatusText:    statusText(a.Status),
			SourceType:    a.SourceType,
			FileType:      a.FileType,
			MinioPath:     a.MinioPath,
			WordCount:     a.WordCount,
			ChunkCount:    a.ChunkCount,
			ProcessStatus: a.ProcessStatus,
			ProcessError:  a.ProcessError,
			CreatedBy:     a.CreatedBy,
			ReviewedBy:    a.ReviewedBy,
			PublishedBy:   a.PublishedBy,
			ReviewComment: a.ReviewComment,
			CreatedAt:     a.CreatedAt,
			UpdatedAt:     a.UpdatedAt,
		}
	}

	return &response.ArticleListResponse{
		Articles: result,
		Total:    total,
	}, nil
}

// GetArticleDetail 获取文章详情。
func (s *KnowledgeService) GetArticleDetail(id int64) (*response.ArticleDetailResponse, error) {
	// TODO(service/knowledge): 详情响应仍返回 sync_status/sync_error/synced_at。
	// TODO(service/knowledge): 应改为 process_status/process_error/chunk_index。
	article, err := s.repo.FindArticleByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
		}
		return nil, err
	}

	chunks, err := s.repo.FindChunksByArticleID(id)
	if err != nil {
		return nil, err
	}

	chunkResponses := make([]response.ChunkResponse, len(chunks))
	for i, c := range chunks {
		chunkResponses[i] = response.ChunkResponse{
			ID:              c.ID,
			KBID:            c.KBID,
			Content:         c.Content,
			ChunkIndex:      c.ChunkIndex,
			EmbeddingModel:  c.EmbeddingModel,
			VectorDimension: c.VectorDimension,
		}
	}

	return &response.ArticleDetailResponse{
		ArticleResponse: response.ArticleResponse{
			ID:            article.ID,
			KBID:          article.KBID,
			KBName:        article.KnowledgeBase.Name,
			Title:         article.Title,
			Content:       article.Content,
			Category:      article.Category,
			Tags:          unmarshalTags(article.Tags),
			Status:        article.Status,
			StatusText:    statusText(article.Status),
			SourceType:    article.SourceType,
			FileType:      article.FileType,
			MinioPath:     article.MinioPath,
			WordCount:     article.WordCount,
			ChunkCount:    article.ChunkCount,
			ProcessStatus: article.ProcessStatus,
			ProcessError:  article.ProcessError,
			CreatedBy:     article.CreatedBy,
			ReviewedBy:    article.ReviewedBy,
			PublishedBy:   article.PublishedBy,
			ReviewComment: article.ReviewComment,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
		},
		Chunks: chunkResponses,
	}, nil
}

// =============================================================================
// 文档上传与处理
// =============================================================================

// UploadDocuments 上传文档到知识库（解析→创建文章→入队异步处理）。
//
// fileSize 用于大小上限校验（最大 50MB），fileType 用于格式白名单校验。
func (s *KnowledgeService) UploadDocuments(kbID int64, userID int64, filename string, fileType string, fileSize int64, content io.Reader) (*model.KnowledgeArticle, error) {
	allowedTypes := map[string]bool{"pdf": true, "docx": true, "md": true, "txt": true}
	if !allowedTypes[fileType] {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "不支持的文件格式: " + fileType + "（支持: pdf/docx/md/txt）"}
	}

	const maxSize = 50 * 1024 * 1024
	if fileSize > maxSize {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件大小超过限制（最大 50MB）"}
	}

	article := &model.KnowledgeArticle{
		KBID:      kbID,
		Title:     filename,
		Content:   "",
		Category:  "文档上传",
		FileType:  fileType,
		Status:    1,
		CreatedBy: userID,
	}

	// 读取文件内容到内存（MinIO 上传和降级解析都需要）
	data, err := io.ReadAll(io.LimitReader(content, maxSize))
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "读取上传文件失败: " + err.Error()}
	}
	if len(data) == 0 {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文件内容为空"}
	}

	var task rag.ProcessTask
	if s.storage != nil {
		// 写入 MinIO 对象存储，processor 异步下载解析
		bucket := "opsmind-documents"
		key := fmt.Sprintf("documents/%d_%s", time.Now().UnixNano(), filename)
		if _, err := s.storage.Upload(context.Background(), bucket, key, strings.NewReader(string(data)), int64(len(data)), ""); err != nil {
			slog.Error("上传文件到 MinIO 失败", "bucket", bucket, "key", key, "error", err)
			return nil, errcode.AppError{Code: errcode.ErrStorageUnavailable, Message: "上传文件到对象存储失败"}
		}
		article.MinioPath = fmt.Sprintf("%s/%s", bucket, key)
		task = rag.ProcessTask{
			ArticleID: article.ID,
			KBID:      kbID,
			Bucket:    bucket,
			Key:       key,
			FileType:  fileType,
			OnStatusChange: func(aID int64, status, errMsg string) {
				_ = s.repo.UpdateArticleProcessStatus(aID, status, errMsg)
			},
			OnMetrics: func(aID int64, wordCount, chunkCount int) {
				_ = s.repo.UpdateArticleMetrics(aID, wordCount, chunkCount)
			},
		}
	} else {
		// 无 StorageClient 时降级：同步解析文本，processor 直接分块
		if s.docParser == nil {
			return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "文档解析器未初始化"}
		}
		text, err := s.docParser.Parse(strings.NewReader(string(data)), fileType)
		if err != nil {
			return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档解析失败: " + err.Error()}
		}
		if strings.TrimSpace(text) == "" {
			return nil, errcode.AppError{Code: errcode.ErrParam, Message: "文档内容为空"}
		}
		article.Content = text
		task = rag.ProcessTask{
			ArticleID: article.ID,
			KBID:      kbID,
			Content:   text,
			OnStatusChange: func(aID int64, status, errMsg string) {
				_ = s.repo.UpdateArticleProcessStatus(aID, status, errMsg)
			},
			OnMetrics: func(aID int64, wordCount, chunkCount int) {
				_ = s.repo.UpdateArticleMetrics(aID, wordCount, chunkCount)
			},
		}
	}

	if err := s.repo.CreateArticle(article); err != nil {
		if s.storage != nil && article.MinioPath != "" {
			b, k := splitMinioPath(article.MinioPath)
			if delErr := s.storage.Delete(context.Background(), b, k); delErr != nil {
				slog.Warn("清理 MinIO 孤立文件失败", "path", article.MinioPath, "error", delErr)
			}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建文章失败: " + err.Error()}
	}

	if s.processor != nil {
		if err := s.processor.Submit(task); err != nil {
			return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "提交处理任务失败: " + err.Error()}
		}
	}

	return article, nil
}

// GetDocumentStatus 查询文档处理状态。
func (s *KnowledgeService) GetDocumentStatus(articleID int64) (string, error) {
	article, err := s.repo.FindArticleByID(articleID)
	if err != nil {
		return "", errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
	}
	return mapArticleToProcessStatus(article), nil
}

// RetryDocument 重试文档处理（重新入队）。
//
// 状态机：仅允许 ProcessStatus == "failed" 的文章重试（避免对已成功或处理中文档重复入队）。
// 重试不修改 Article.Status（审核状态），仅清空 ProcessError 让前端重新展示错误。
func (s *KnowledgeService) RetryDocument(articleID int64) error {
	article, err := s.repo.FindArticleByID(articleID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "文章不存在"}
	}
	if article.ProcessStatus != "failed" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅处理失败的文章可重试"}
	}
	if s.processor == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "文档处理器未初始化"}
	}

	// 重置 process_status 为 pending，processor 在新一轮处理开始时会再覆盖
	if err := s.repo.UpdateArticleProcessStatus(articleID, "pending", ""); err != nil {
		slog.Warn("重置处理状态失败，不阻断主流程", "article_id", articleID, "error", err)
	}
	var task rag.ProcessTask
	if article.MinioPath != "" {
		bucket, key := splitMinioPath(article.MinioPath)
		task = rag.ProcessTask{
			ArticleID: articleID,
			KBID:      article.KBID,
			Bucket:    bucket,
			Key:       key,
			FileType:  article.FileType,
			OnStatusChange: func(aID int64, status, errMsg string) {
				_ = s.repo.UpdateArticleProcessStatus(aID, status, errMsg)
			},
			OnMetrics: func(aID int64, wordCount, chunkCount int) {
				_ = s.repo.UpdateArticleMetrics(aID, wordCount, chunkCount)
			},
		}
	} else {
		task = rag.ProcessTask{
			ArticleID: articleID,
			KBID:      article.KBID,
			Content:   article.Content,
			OnStatusChange: func(aID int64, status, errMsg string) {
				_ = s.repo.UpdateArticleProcessStatus(aID, status, errMsg)
			},
			OnMetrics: func(aID int64, wordCount, chunkCount int) {
				_ = s.repo.UpdateArticleMetrics(aID, wordCount, chunkCount)
			},
		}
	}
	if err := s.processor.Submit(task); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "提交处理任务失败: " + err.Error()}
	}
	return nil
}

// =============================================================================
// 辅助函数
// =============================================================================

func marshalTags(tags []string) datatypes.JSON {
	// TODO(service/knowledge): tags 应 trim、去重、限制数量和单个长度。
	// 标签直接写入 JSONB 会让前端和检索筛选承受脏数据。
	if len(tags) == 0 {
		return datatypes.JSON(`[]`)
	}
	data, _ := json.Marshal(tags)
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

func statusText(status int16) string {
	switch status {
	case 0:
		return "已停用"
	case 1:
		return "草稿"
	case 2:
		return "待审核"
	case 3:
		return "已通过"
	case 4:
		return "已发布"
	case 5:
		return "已驳回"
	default:
		return "未知"
	}
}

// splitMinioPath 将 "bucket/key" 格式的 MinioPath 拆分为 bucket 和 key。
func splitMinioPath(path string) (string, string) {
	idx := strings.Index(path, "/")
	if idx < 0 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}

// mapProcessStatus 已废弃：文档处理阶段不再写入 Article.Status（审核状态机）。
//
// 历史说明：早期实现把 Processor 阶段映射为 Article.Status（草稿/审核通过），
// 造成"审核状态"和"处理状态"两个状态机共用同一字段，前端无法可靠区分。
// 2026-06-17 重构后，处理进度仅通过 ProcessStatus 字段表达（独立字符串枚举），
// Article.Status 仅承载人工审核流程（Draft/Reviewing/Approved/Published/Rejected/Disabled）。
//
// 函数体保留为 no-op 占位，避免外部 mock 引用编译失败；新代码不应再调用。
func mapProcessStatus(status string) int {
	_ = status
	return 0
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
