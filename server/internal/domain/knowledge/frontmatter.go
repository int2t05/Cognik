// Package knowledge 知识库领域业务逻辑、数据访问与 HTTP 处理。
//
// frontmatter.go：文章元数据 schema——定义 frontmatter 字段集合、校验、解析与渲染。
// .md 文件 = frontmatter（派生元数据）+ # 标题 + 正文。文件即真相，DB 列为派生索引。
// 发布时由 MetadataCompleter 补全缺失字段（type/tags），正文不变。
package knowledge

import (
	"fmt"
	"strings"
	"time"

	"cognik/internal/rag"
)

// ArticleMeta 文章 frontmatter 元数据（.md 文件头部 --- 块）。
type ArticleMeta struct {
	Type       string   // guide/reference/procedure/analysis/note/faq/snippet
	Tags       []string // 标签
	Status     string   // draft/published
	SourceType string   // manual/upload/deep_research
	Created    string   // RFC3339
	Updated    string   // RFC3339
}

// AllowedArticleTypes 合法文章类型集合。发布时校验 + LLM 补全均从此集合选取。
var AllowedArticleTypes = []string{"guide", "reference", "procedure", "analysis", "note", "faq", "snippet"}

// IsAllowedType 判断 type 是否在合法集合内。
func IsAllowedType(t string) bool {
	for _, a := range AllowedArticleTypes {
		if a == t {
			return true
		}
	}
	return false
}

// ParseArticleMeta 从 content 解析 frontmatter → ArticleMeta + 正文 body。
// 无 frontmatter 时返回零值 meta 与原 content。
func ParseArticleMeta(content string) (ArticleMeta, string) {
	fm, body := rag.ParseFrontmatter(content)
	meta := ArticleMeta{
		Type:       fm["type"],
		Tags:       parseTagsField(fm["tags"]),
		Status:     fm["status"],
		SourceType: fm["source_type"],
		Created:    fm["created"],
		Updated:    fm["updated"],
	}
	return meta, body
}

// RenderArticleFile 渲染完整 .md 文件：frontmatter（固定字段顺序）+ # 标题 + 正文。
// processor 下载后剥离 frontmatter，仅对 # 标题 + 正文分块（H1 保留供 BM25 enrich）。
func RenderArticleFile(meta ArticleMeta, title, body string) string {
	now := time.Now().Format(time.RFC3339)
	if meta.Created == "" {
		meta.Created = now
	}
	meta.Updated = now
	if meta.Status == "" {
		meta.Status = "published"
	}
	if meta.SourceType == "" {
		meta.SourceType = "manual"
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("type: %s\n", meta.Type))
	sb.WriteString(fmt.Sprintf("status: %s\n", meta.Status))
	sb.WriteString(fmt.Sprintf("source_type: %s\n", meta.SourceType))
	sb.WriteString(fmt.Sprintf("created: %s\n", meta.Created))
	sb.WriteString(fmt.Sprintf("updated: %s\n", meta.Updated))
	if len(meta.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(meta.Tags, ", ")))
	}
	sb.WriteString("---\n\n")
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString(body)
	return sb.String()
}

// parseTagsField 解析 frontmatter tags 字段（[a, b] → []string{"a","b"}）。
func parseTagsField(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var tags []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
