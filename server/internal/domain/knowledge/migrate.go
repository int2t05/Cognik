// Package knowledge — migrate.go 一次性存储布局迁移工具。
//
// 将旧布局（双桶 + 标题目录）迁移到新布局（单桶 + kb-{id}/{draft|published}/article-{id}）。
// 由 cmd/migrate-storage 触发，执行一次后数据即与新代码一致；本文件可随后删除。
package knowledge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"opsmind/internal/infra/storage"
	"opsmind/internal/shared/model"

	"gorm.io/gorm"
)

// 旧布局双桶名（重构后仅保留 documents，published 桶迁移后可手动删除）。
const (
	legacyBucketDocuments = "opsmind-documents"
	legacyBucketPublished = "opsmind-published"
)

// MigrateStorageLayout 将全部文章从旧存储布局迁移到新布局（幂等，可重复执行）。
//
// 旧: {opsmind-documents|opsmind-published}/{titleKey}/{markdown.md, images/}
// 新: opsmind-documents/kb-{kbID}/{draft|published}/article-{articleID}/{markdown.md, images/}
//
// 规则: 旧 bucket=published → published/；旧 bucket=documents → draft/。
// 幂等: 新格式路径（kb- 前缀）跳过；旧目录不存在跳过。
//
// 示例: article{ID:3, KBID:1} 旧 MinioPath="opsmind-published/四六级成绩证明"
//
//	→ 新 dir="kb-1/published/article-3"，新 MinioPath="opsmind-documents/kb-1/published/article-3"
func MigrateStorageLayout(db *gorm.DB, store storage.StorageClient) error {
	if store == nil {
		return fmt.Errorf("storage 未初始化")
	}
	ctx := context.Background()

	var articles []model.KnowledgeArticle
	if err := db.WithContext(ctx).Select("id", "kb_id", "minio_path").Find(&articles).Error; err != nil {
		return fmt.Errorf("查询文章列表失败: %w", err)
	}

	migrated, skipped, failed := 0, 0, 0
	for _, a := range articles {
		oldPath := strings.TrimSpace(a.MinioPath)
		if oldPath == "" {
			continue
		}

		// 解析旧 MinioPath: {bucket}/{titleKey}
		parts := strings.SplitN(oldPath, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			slog.Warn("迁移跳过：MinioPath 格式异常", "article_id", a.ID, "minio_path", oldPath)
			skipped++
			continue
		}
		oldBucket, oldKey := parts[0], parts[1]

		// 已迁移（新布局 dir 以 kb- 开头）跳过
		if strings.HasPrefix(oldKey, "kb-") {
			skipped++
			continue
		}

		published := oldBucket == legacyBucketPublished
		newDir := articleDir(a.KBID, a.ID, published)
		newPath := minioBucket + "/" + newDir

		if err := migrateOneArticleDir(ctx, store, oldBucket, oldKey, minioBucket, newDir); err != nil {
			slog.Error("迁移文章目录失败", "article_id", a.ID, "old", oldPath, "new", newPath, "error", err)
			failed++
			continue
		}

		if err := db.WithContext(ctx).Model(&model.KnowledgeArticle{}).
			Where("id = ?", a.ID).
			Update("minio_path", newPath).Error; err != nil {
			slog.Error("更新 MinioPath 失败", "article_id", a.ID, "new", newPath, "error", err)
			failed++
			continue
		}
		slog.Info("迁移成功", "article_id", a.ID, "old", oldPath, "new", newPath)
		migrated++
	}

	slog.Info("存储布局迁移完成", "migrated", migrated, "skipped", skipped, "failed", failed)
	if failed > 0 {
		return fmt.Errorf("迁移存在 %d 个失败项，详见日志", failed)
	}
	return nil
}

// migrateOneArticleDir 将旧目录 {srcBucket}/{srcDir} 全部文件搬到 {dstBucket}/{dstDir}，搬完删源。
// 旧目录不存在视为已迁移（幂等）。
func migrateOneArticleDir(ctx context.Context, store storage.StorageClient, srcBucket, srcDir, dstBucket, dstDir string) error {
	files, err := store.DownloadDir(ctx, srcBucket, srcDir)
	if err != nil {
		// 旧目录不存在 → 已迁移或无文件，幂等跳过
		slog.Info("迁移跳过：旧目录不存在", "src_bucket", srcBucket, "src_dir", srcDir)
		return nil
	}
	defer func() {
		for _, r := range files {
			r.Close()
		}
	}()

	for filename, r := range files {
		data, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("读取文件失败 %s: %w", filename, err)
		}
		ct := imageContentType(filename)
		if strings.HasSuffix(filename, ".md") {
			ct = "text/markdown"
		}
		if err := store.UploadFile(ctx, dstBucket, dstDir, filename, bytes.NewReader(data), int64(len(data)), ct); err != nil {
			return fmt.Errorf("上传文件失败 %s: %w", filename, err)
		}
	}

	if err := store.DeleteDir(ctx, srcBucket, srcDir); err != nil {
		slog.Warn("迁移删除旧目录失败（已搬完，可手动清理）", "src_bucket", srcBucket, "src_dir", srcDir, "error", err)
	}
	return nil
}
