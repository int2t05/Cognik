// Package knowledge 知识库领域业务逻辑、数据访问与 HTTP 处理。
//
// image_cleaner.go：孤儿图片定时清理——扫描 image/ 目录下未被任何已发布文章引用的图片。
// 图片按内容寻址（SHA256[:8].ext），文章在正文中用 ![](image/{name}) 引用。
// 未被任何文章 Content 引用的图片即为孤儿，可安全删除。
package knowledge

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ImageCleaner 孤儿图片清理器。扫描本地文件系统 image/ 目录，删除未被引用的图片。
type ImageCleaner struct {
	svc         *KnowledgeService
	imageDir    string // 图片物理目录路径（local 模式）
	bucket      string
}

// NewImageCleaner 创建图片清理器。imageDir 为图片所在物理目录。
func NewImageCleaner(svc *KnowledgeService, imageDir, bucket string) *ImageCleaner {
	return &ImageCleaner{svc: svc, imageDir: imageDir, bucket: bucket}
}

// imageRefPattern 匹配 markdown 中的图片引用：![](image/{name}) 或 <img src="image/{name}">
var imageRefPattern = regexp.MustCompile(`image/([a-f0-9]+\.\w+)`)

// CleanupOrphaned 扫描 image/ 目录，删除未被任何已发布文章引用的图片。
// 返回删除的图片数。
func (c *ImageCleaner) CleanupOrphaned(ctx context.Context) (int, error) {
	if c == nil || c.imageDir == "" {
		return 0, nil
	}

	// 1. 收集所有已发布文章引用的图片名
	referenced := make(map[string]bool)
	articles, _, err := c.svc.repo.ListArticles(ctx, 0, 4, 0, "", "", 1, 10000)
	if err != nil {
		return 0, fmt.Errorf("查询文章列表失败: %w", err)
	}
	for _, a := range articles {
		for _, match := range imageRefPattern.FindAllStringSubmatch(a.Content, -1) {
			if len(match) > 1 {
				referenced[match[1]] = true
			}
		}
	}

	// 2. 扫描 image/ 目录下的所有图片文件
	entries, err := os.ReadDir(c.imageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // 目录不存在，无图片可清理
		}
		return 0, fmt.Errorf("扫描图片目录失败: %w", err)
	}

	// 3. 删除未被引用的孤儿图片
	deleted := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !referenced[entry.Name()] {
			path := filepath.Join(c.imageDir, entry.Name())
			if err := os.Remove(path); err != nil {
				slog.Warn("删除孤儿图片失败", "file", entry.Name(), "error", err)
			} else {
				deleted++
			}
		}
	}

	if deleted > 0 {
		slog.Info("孤儿图片清理完成", "deleted", deleted, "referenced", len(referenced))
	}
	return deleted, nil
}

// StartPeriodicCleanup 启动定时清理（默认每 24 小时一次）。
func (c *ImageCleaner) StartPeriodicCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.CleanupOrphaned(ctx); err != nil {
				slog.Warn("孤儿图片清理失败", "error", err)
			}
		}
	}
}
