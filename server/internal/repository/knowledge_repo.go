// Package repository 提供知识库的数据访问层。
//
// KnowledgeRepo 封装 knowledge_bases、knowledge_articles、knowledge_chunks
// 三张表的 CRUD 操作，供 KnowledgeService 调用。
package repository

import (
	"context"

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

func (r *KnowledgeRepo) CreateKB(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Create(kb).Error
}

func (r *KnowledgeRepo) FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *KnowledgeRepo) UpdateKB(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Save(kb).Error
}

func (r *KnowledgeRepo) ListKBs(ctx context.Context) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	err := r.db.WithContext(ctx).Order("id ASC").Find(&kbs).Error
	if err != nil {
		return nil, err
	}
	if kbs == nil {
		kbs = []model.KnowledgeBase{}
	}
	return kbs, nil
}

func (r *KnowledgeRepo) CountArticlesByKB(ctx context.Context) (map[int64]int, error) {
	type row struct {
		KBID  int64
		Count int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
		Select("kb_id, COUNT(*) as count").
		Where("status != ?", model.ArticleStatusDisabled).
		Group("kb_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[int64]int, len(rows))
	for _, r := range rows {
		m[r.KBID] = r.Count
	}
	return m, nil
}

// =============================================================================
// KnowledgeArticle
// =============================================================================

func (r *KnowledgeRepo) CreateArticle(ctx context.Context, article *model.KnowledgeArticle) error {
	return r.db.WithContext(ctx).Create(article).Error
}

func (r *KnowledgeRepo) FindArticleByID(ctx context.Context, id int64) (*model.KnowledgeArticle, error) {
	var article model.KnowledgeArticle
	err := r.db.WithContext(ctx).Preload("KnowledgeBase").Where("id = ?", id).First(&article).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *KnowledgeRepo) UpdateArticle(ctx context.Context, article *model.KnowledgeArticle) error {
	return r.db.WithContext(ctx).Save(article).Error
}

func (r *KnowledgeRepo) ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, page, pageSize int) ([]model.KnowledgeArticle, int64, error) {
	var articles []model.KnowledgeArticle
	var total int64

	query := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("kb_id = ?", kbID).Preload("KnowledgeBase")
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if sourceType > 0 {
		query = query.Where("source_type = ?", sourceType)
	}
	if processStatus != "" {
		query = query.Where("process_status = ?", processStatus)
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

func (r *KnowledgeRepo) UpdateArticleStatus(ctx context.Context, id int64, status int) error {
	res := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Update("status", status)
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateArticleStatusCAS 原子状态转换（Compare-And-Swap）。
// 仅当前状态为 expectedOld 时才更新为 newStatus，返回受影响行数。
// 0 行表示并发冲突——其他请求已修改了状态。
func (r *KnowledgeRepo) UpdateArticleStatusCAS(ctx context.Context, id int64, expectedOld, newStatus int) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
		Where("id = ? AND status = ?", id, expectedOld).
		Update("status", newStatus)
	return res.RowsAffected, res.Error
}

func (r *KnowledgeRepo) UpdateArticleProcessStatus(ctx context.Context, id int64, processStatus, processError string) error {
	updates := map[string]interface{}{
		"process_status": processStatus,
	}
	if processError != "" {
		updates["process_error"] = processError
	}
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(updates).Error
}

func (r *KnowledgeRepo) UpdateArticleMetrics(ctx context.Context, id int64, wordCount, chunkCount int) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"word_count":  wordCount,
		"chunk_count": chunkCount,
	}).Error
}

// DeleteArticle 删除文章（含关联的 knowledge_chunks 向量数据）。
func (r *KnowledgeRepo) DeleteArticle(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("article_id = ?", id).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.KnowledgeArticle{}).Error
	})
}

func (r *KnowledgeRepo) DeleteKB(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删 chunks（FK 到 articles），再删 articles，最后删 kb
		if err := tx.Where("kb_id = ?", id).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", id).Delete(&model.KnowledgeArticle{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).Delete(&model.KnowledgeBase{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// =============================================================================
// KnowledgeChunk
// =============================================================================

func (r *KnowledgeRepo) FindChunksByArticleID(ctx context.Context, articleID int64) ([]model.KnowledgeChunk, error) {
	var chunks []model.KnowledgeChunk
	err := r.db.WithContext(ctx).Where("article_id = ?", articleID).Order("id ASC").Find(&chunks).Error
	if err != nil {
		return nil, err
	}
	if chunks == nil {
		chunks = []model.KnowledgeChunk{}
	}
	return chunks, nil
}
