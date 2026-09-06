// Package knowledge 知识库领域数据访问与 HTTP 处理。
//
// repository.go 封装 knowledge_bases / knowledge_articles / knowledge_chunks 表 CRUD。
package knowledge

import (
	"context"
	"time"

	"cognik/internal/shared/model"
	"cognik/internal/shared/pkg/dbutil"

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

func (r *KnowledgeRepo) ListKBs(ctx context.Context, keyword string) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	query := r.db.WithContext(ctx).Order("id ASC")
	if keyword != "" {
		like := "%" + dbutil.EscapeLike(keyword) + "%"
		query = query.Where("name ILIKE ? ESCAPE '\\' OR description ILIKE ? ESCAPE '\\'", like, like)
	}
	err := query.Find(&kbs).Error
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

func (r *KnowledgeRepo) ListArticles(ctx context.Context, kbID int64, status int, sourceType int, processStatus string, keyword string, page, pageSize int) ([]model.KnowledgeArticle, int64, error) {
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
	if keyword != "" {
		query = query.Where("(title ILIKE ? OR tags::text ILIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
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

// UpdateArticleStatusCAS 原子状态转换（CAS，0 行表示并发冲突）。
func (r *KnowledgeRepo) UpdateArticleStatusCAS(ctx context.Context, id int64, expectedOld, newStatus int) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
		Where("id = ? AND status = ?", id, expectedOld).
		Update("status", newStatus)
	return res.RowsAffected, res.Error
}

// UpdateArticleMinioPath 仅更新 minio_path（嵌入回调使用，避免 Save 脏写）。
func (r *KnowledgeRepo) UpdateArticleMinioPath(ctx context.Context, id int64, path string) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).
		Update("minio_path", path).Error
}

// UpdateArticleDisable 停用文章——状态置零 + 清除分块和处理状态。
func (r *KnowledgeRepo) UpdateArticleDisable(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":         0,
		"chunk_count":    0,
		"process_status": "",
		"process_error":  "",
	}).Error
}

// UpdateArticleReview 仅更新审核相关字段（精确更新，不触及其他列）。
func (r *KnowledgeRepo) UpdateArticleReview(ctx context.Context, id int64, status int, reviewerID int64, reviewComment string) error {
	updates := map[string]interface{}{
		"status":         status,
		"reviewed_by":    reviewerID,
		"review_comment": reviewComment,
	}
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(updates).Error
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

// UpdateArticleProcessStatusCAS 原子声明任务归属（CAS from→to），成功时清 next_retry_at。
// 用于重试扫描器/手动重试：仅首个声明者通过（rowsAffected==1），其余跳过，避免并发双写。
func (r *KnowledgeRepo) UpdateArticleProcessStatusCAS(ctx context.Context, id int64, fromStatus, toStatus, processError string) (int64, error) {
	updates := map[string]interface{}{
		"process_status": toStatus,
		"next_retry_at":  nil,
	}
	if processError != "" {
		updates["process_error"] = processError
	}
	res := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
		Where("id = ? AND process_status = ?", id, fromStatus).Updates(updates)
	return res.RowsAffected, res.Error
}

// CompleteArticleProcessing 置 process_status='completed' 并清 process_error/next_retry_at。
// 在 GORM 事务内与 ReplaceVectorsWithTx 同提交，保证向量与终态原子一致。
func (r *KnowledgeRepo) CompleteArticleProcessing(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"process_status": "completed",
		"process_error":  "",
		"next_retry_at":  nil,
	}).Error
}

// ScheduleArticleRetry 递增重试计数,保持 failed 状态,设下次重试时间。
func (r *KnowledgeRepo) ScheduleArticleRetry(ctx context.Context, id int64, errMsg string, nextRetryAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"process_status":    "failed",
		"process_error":     errMsg,
		"process_retry_count": gorm.Expr("process_retry_count + 1"),
		"next_retry_at":     nextRetryAt,
	}).Error
}

// MarkArticleFailedTerminal 写终态失败(清除 next_retry_at,扫描器不再拾取)。
func (r *KnowledgeRepo) MarkArticleFailedTerminal(ctx context.Context, id int64, errMsg string) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"process_status": "failed",
		"process_error":  errMsg,
		"next_retry_at": nil,
	}).Error
}

// ListArticlesForRetry 返回需重试的文章：到期失败文章 + 停滞超过阈值的处理中文章（worker 崩溃/重启恢复）。
// stuckBefore 为阈值截止时刻（NOW - stuckThreshold），由调用方计算后传入，避免 PG timestamp-interval 参数类型问题。
func (r *KnowledgeRepo) ListArticlesForRetry(ctx context.Context, limit int, stuckBefore time.Time) ([]model.KnowledgeArticle, error) {
	var articles []model.KnowledgeArticle
	err := r.db.WithContext(ctx).Preload("KnowledgeBase").
		Where(`(process_status = 'failed' AND next_retry_at IS NOT NULL AND next_retry_at <= NOW())
			OR (process_status IN ('processing','parsing','chunking','embedding','indexing')
				AND updated_at < ?)`, stuckBefore).
		Order("updated_at ASC").
		Limit(limit).
		Find(&articles).Error
	return articles, err
}

func (r *KnowledgeRepo) UpdateArticleMetrics(ctx context.Context, id int64, wordCount, chunkCount int) error {
	return r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"word_count":  wordCount,
		"chunk_count": chunkCount,
	}).Error
}

// ExistsByTitle 检查同 KB 下是否已存在相同标题的文章。
func (r *KnowledgeRepo) ExistsByTitle(ctx context.Context, kbID int64, title string, excludeID int64) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
		Where("kb_id = ? AND title = ?", kbID, title)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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
