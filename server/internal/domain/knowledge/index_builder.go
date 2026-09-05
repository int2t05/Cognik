// Package knowledge 知识库领域业务逻辑、数据访问与 HTTP 处理。
//
// index_builder.go：INDEX.md 页目录自动重建——从已发布文章的 frontmatter 元数据
// 按 type 分组、按 title 排序渲染。文件即真相：INDEX.md 是派生索引，可从文章重建。
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cognos/internal/shared/model"
)

// IndexBuilder 知识库页目录重建器。从已发布文章生成 INDEX.md。
type IndexBuilder struct {
	svc *KnowledgeService
}

// NewIndexBuilder 创建页目录重建器。
func NewIndexBuilder(svc *KnowledgeService) *IndexBuilder {
	return &IndexBuilder{svc: svc}
}

// RebuildKBIndex 包级函数——直接用 repo 查询已发布文章，重建 INDEX.md。
// 用于 onKBChanged 回调（避免循环引用 KnowledgeService）。
func RebuildKBIndex(repo knowledgeRepo, kbID int64, outputDir string) {
	ctx := context.Background()
	articles, _, err := repo.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 10000)
	if err != nil {
		slog.Warn("INDEX.md 重建：查询文章失败", "kb_id", kbID, "error", err)
		return
	}

	groups := make(map[string][]indexEntry)
	for _, a := range articles {
		var tagList []string
		if len(a.Tags) > 0 {
			_ = json.Unmarshal(a.Tags, &tagList)
		}
		entryType := a.ArticleType
		if entryType == "" {
			entryType = "guide"
		}
		groups[entryType] = append(groups[entryType], indexEntry{
			title: a.Title,
			tags:  tagList,
			slug:  slugify(a.Title),
		})
	}

	content := renderIndexMarkdown(groups)
	outputPath := filepath.Join(outputDir, "INDEX.md")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		slog.Warn("INDEX.md 重建：创建目录失败", "kb_id", kbID, "error", err)
		return
	}
	tmpPath := outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		slog.Warn("INDEX.md 重建：写入失败", "kb_id", kbID, "error", err)
		return
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		slog.Warn("INDEX.md 重建：重命名失败", "kb_id", kbID, "error", err)
		return
	}
	slog.Info("INDEX.md 重建完成", "kb_id", kbID, "articles", len(articles))
}

// RebuildIndex 重建指定 KB 的 INDEX.md 页目录。
// 扫描已发布文章，按 frontmatter type 分组，按 title 排序，原子写入。
func (b *IndexBuilder) RebuildIndex(ctx context.Context, kbID int64, outputPath string) error {
	// 从 DB 读取已发布文章（status=4），不依赖文件系统列举
	resp, err := b.svc.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 10000)
	if err != nil {
		return fmt.Errorf("读取文章列表失败: %w", err)
	}

	// 按 article_type 分组
		groups := make(map[string][]indexEntry)
		for _, a := range resp.Articles {
			entryType := a.ArticleType
			if entryType == "" {
				entryType = "guide"
			}
			groups[entryType] = append(groups[entryType], indexEntry{
				title: a.Title,
				tags:  a.Tags, // ListArticles 返回的 ArticleResponse.Tags 是 []string
				slug:  slugify(a.Title),
			})
		}

	// 渲染 INDEX.md
	content := renderIndexMarkdown(groups)

	// 原子写：write-to-temp + rename
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	tmpPath := outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, outputPath)
}

// indexEntry INDEX.md 条目。
type indexEntry struct {
	title string
	tags  []string
	slug  string
}

// renderIndexMarkdown 渲染 INDEX.md 内容：按 type 分组，组内按 title 排序。
func renderIndexMarkdown(groups map[string][]indexEntry) string {
	var sb strings.Builder
	sb.WriteString("<!-- 自动生成，禁止手编 -->\n")
	sb.WriteString("# 知识库目录\n\n")

	// 按 type 名称排序（字母序）
	typeNames := make([]string, 0, len(groups))
	for name := range groups {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		entries, ok := groups[typeName]
		if !ok || len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].title < entries[j].title })
		sb.WriteString(fmt.Sprintf("## %s\n", typeName))
		for _, e := range entries {
			tagsStr := ""
			if len(e.tags) > 0 {
				tagsStr = " `" + strings.Join(e.tags, "` `") + "`"
			}
			sb.WriteString(fmt.Sprintf("- [%s](published/%s.md)%s\n", e.title, e.slug, tagsStr))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()) + "\n"
}

// slugify 将标题转为 kebab-case slug。
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnumRegexp.ReplaceAllString(s, "-")
	s = multiDashRegexp.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

var (
	nonAlnumRegexp  = regexp.MustCompile(`[^a-z0-9]+`)
	multiDashRegexp = regexp.MustCompile(`-+`)
)
