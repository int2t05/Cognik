// Package repository 提供知识库的数据访问层。
//
// KnowledgeRepo 封装 knowledge_bases、knowledge_articles、knowledge_chunks、
// knowledge_chunks 三张表的 CRUD 操作，供 KnowledgeService 调用。
// 为什么独立于 UserRepo：知识库相关表的查询模式不同（按 kb_id 过滤分页、
// 同步状态批量更新、预加载关联表），独立 Repo 更利于职责聚焦。
package repository

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// KnowledgeRepo 知识库数据访问
type KnowledgeRepo struct {
	db *gorm.DB
}

// NewKnowledgeRepo 创建 KnowledgeRepo 实例
func NewKnowledgeRepo(db *gorm.DB) *KnowledgeRepo {
	return &KnowledgeRepo{db: db}
}

// =============================================================================
// KnowledgeBase
// =============================================================================

// CreateKB 创建知识库。
//
// 创建后 kb.ID 会被 GORM 自动填充。
// rag_workspace_slug 唯一约束由数据库保证，重复时返回 PostgreSQL 错误。
func (r *KnowledgeRepo) CreateKB(kb *model.KnowledgeBase) error {
	// TODO(repository/knowledge): 创建知识库应依赖数据库唯一索引兜底，并将重复错误转换为 ErrConflict。
	// 当前 Service 没有名称唯一性预校验，重复名称会返回底层数据库错误。
	return r.db.Create(kb).Error
}

// FindKBByID 按 ID 查询知识库。
//
// ID 是主键，查询不到时返回 gorm.ErrRecordNotFound。
func (r *KnowledgeRepo) FindKBByID(id int64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.Where("id = ?", id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

// UpdateKB 更新知识库全部字段。
//
// 为什么用 Save 而非 Updates：Save 会更新所有字段（包括零值），
// 适用于名称、描述、embedding 配置等全量覆盖场景。
func (r *KnowledgeRepo) UpdateKB(kb *model.KnowledgeBase) error {
	return r.db.Save(kb).Error
}

// ListKBs 列出全部知识库。
//
// 为什么返回空切片而非 nil：调用方可以直接 range 遍历，无需额外 nil 判断。
func (r *KnowledgeRepo) ListKBs() ([]model.KnowledgeBase, error) {
	// TODO(repository/knowledge): ListKBs 应返回 article_count，避免 Service/前端再额外查询统计。
	// API 文档已经定义 article_count 字段。
	var kbs []model.KnowledgeBase
	err := r.db.Order("id ASC").Find(&kbs).Error
	if err != nil {
		return nil, err
	}
	if kbs == nil {
		kbs = []model.KnowledgeBase{}
	}
	return kbs, nil
}

// =============================================================================
// KnowledgeArticle
// =============================================================================

// CreateArticle 创建知识文章。
//
// 创建后 article.ID 会被 GORM 自动填充。
func (r *KnowledgeRepo) CreateArticle(article *model.KnowledgeArticle) error {
	return r.db.Create(article).Error
}

// FindArticleByID 按 ID 查询文章，预加载关联的 KnowledgeBase。
//
// 为什么预加载 KnowledgeBase：文章详情需要显示所属知识库名称，
// Preload 一条 SQL 完成 JOIN 避免 N+1 查询。
func (r *KnowledgeRepo) FindArticleByID(id int64) (*model.KnowledgeArticle, error) {
	var article model.KnowledgeArticle
	err := r.db.Preload("KnowledgeBase").Where("id = ?", id).First(&article).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

// UpdateArticle 更新文章全部字段。
//
// 为什么用 Save 而非 Updates：编辑文章时会修改 question、answer、category、tags
// 等多个字段，Save 确保全部更新（包括零值字段）。
func (r *KnowledgeRepo) UpdateArticle(article *model.KnowledgeArticle) error {
	return r.db.Save(article).Error
}

// ListArticles 分页查询文章列表，支持按知识库和状态过滤。
//
// 参数说明：
//   - kbID: 知识库 ID，必传（>0）
//   - status: 文章状态，传 -1 表示不过滤，0 表示已停用
//   - page/pageSize: 分页参数
//
// 为什么按 id DESC 排序：最新创建的文章排在前面，符合管理后台使用习惯。
func (r *KnowledgeRepo) ListArticles(kbID int64, status int, page, pageSize int) ([]model.KnowledgeArticle, int64, error) {
	// TODO(repository/knowledge): Count 后复用同一个 query 继续 Offset/Limit 容易携带 Count 的状态。
	// 虽然 GORM 多数场景可用，后续可 clone session 或使用通用 Paginate 明确分离。
	var articles []model.KnowledgeArticle
	var total int64

	query := r.db.Model(&model.KnowledgeArticle{}).Where("kb_id = ?", kbID).Preload("KnowledgeBase")
	if status >= 0 {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&articles).Error; err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

// UpdateArticleStatus 更新文章状态。
//
// 为什么单独封装而非复用 UpdateArticle：仅更新 status 字段，
// 避免 Save 时意外覆盖其他字段（如 reviewed_by、review_comment 等）。
//
// 返回 gorm.ErrRecordNotFound 当目标文章不存在（RowsAffected == 0），
// 让 Service 能正确向上层返回 404。
func (r *KnowledgeRepo) UpdateArticleStatus(id int64, status int) error {
	res := r.db.Model(&model.KnowledgeArticle{}).Where("id = ?", id).Update("status", status)
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateArticleProcessStatus 更新文档的异步处理状态和错误信息。
func (r *KnowledgeRepo) UpdateArticleProcessStatus(id int64, processStatus, processError string) error {
	updates := map[string]interface{}{
		"process_status": processStatus,
	}
	if processError != "" {
		updates["process_error"] = processError
	}
	return r.db.Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateArticleMetrics 更新文档的字数和分块数。
//
// 分块完成后由 Processor 和 Publish 路径调用，确保前端实时反映处理进度。
func (r *KnowledgeRepo) UpdateArticleMetrics(id int64, wordCount, chunkCount int) error {
	return r.db.Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"word_count":  wordCount,
		"chunk_count": chunkCount,
	}).Error
}

// DeleteKB 删除知识库及其下所有文章（级联删除）。
//
// 为什么在事务中先删文章再删 KB：knowledge_articles 有 kb_id 外键约束，
// 必须先删除子记录才能删除父记录。GORM 不会自动级联 DELETE。
// 向量分块的删除由上层 KnowledgeService 通过 VectorStore.DeleteByKB 完成，
// Repository 层不持有 VectorStore 引用。
func (r *KnowledgeRepo) DeleteKB(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 删除知识库下所有文章（GORM 按 kb_id 批量删除）
		if err := tx.Where("kb_id = ?", id).Delete(&model.KnowledgeArticle{}).Error; err != nil {
			return err
		}
		// 2. 删除知识库记录
		if err := tx.Where("id = ?", id).Delete(&model.KnowledgeBase{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// =============================================================================
// KnowledgeChunk
// =============================================================================
// FindChunksByArticleID 按文章 ID 查询全部切片。
//
// 返回的切片按 ID 升序排列，保证顺序一致。
func (r *KnowledgeRepo) FindChunksByArticleID(articleID int64) ([]model.KnowledgeChunk, error) {
	var chunks []model.KnowledgeChunk
	err := r.db.Where("article_id = ?", articleID).Order("id ASC").Find(&chunks).Error
	if err != nil {
		return nil, err
	}
	if chunks == nil {
		chunks = []model.KnowledgeChunk{}
	}
	return chunks, nil
}

// =============================================================================
