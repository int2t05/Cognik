// generate_article.go：知识库文章生成工具（SyncTool）。
//
// 将搜索/抓取的结果整理为结构化 Markdown（含 frontmatter sources），写入知识库 Draft。
// 复用 main.go 的 ArticleWriter（桥接 KnowledgeService.CreateArticle）。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opsmind/internal/agent"

	"github.com/cloudwego/eino/schema"
)

// ArticleWriter 知识库文章写入接口（供 generate_article 工具）。
// main.go 的 deepResearchWriter 实现它；content 含 frontmatter（含 sources）。
type ArticleWriter interface {
	CreateArticle(ctx context.Context, title, content string, kbID int64) (articleID int64, err error)
}

// GenerateArticleTool 知识库文章生成工具（实现 agent.SyncTool）。
type GenerateArticleTool struct {
	writer ArticleWriter
}

// NewGenerateArticleTool 创建文章生成工具。
func NewGenerateArticleTool(writer ArticleWriter) *GenerateArticleTool {
	return &GenerateArticleTool{writer: writer}
}

// Info 返回工具元信息。
func (t *GenerateArticleTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "generate_article",
		Desc: "Write a structured Markdown article to the knowledge base. The article is saved as Draft (pending human review before entering RAG). Use after web_search + web_fetch to persist research findings.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title": {
				Type:     schema.String,
				Desc:     "Article title",
				Required: true,
			},
			"content": {
				Type:     schema.String,
				Desc:     "Article body in Markdown (with inline citations [1][2] referencing sources)",
				Required: true,
			},
			"sources": {
				Type: schema.Array,
				Desc: `Sources list, JSON array of {"url":"...","title":"..."}. Maintains citation number → URL mapping in frontmatter.`,
			},
			"kb_id": {
				Type:     schema.Integer,
				Desc:     "Target knowledge base ID to write the article to",
				Required: true,
			},
		}),
	}
}

// generateArticleParams generate_article 工具参数。
type generateArticleParams struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Sources []struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"sources"`
	KBID int64 `json:"kb_id"`
}

// Call 生成 frontmatter + 拼接正文，写入知识库 Draft。
func (t *GenerateArticleTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params generateArticleParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Title) == "" {
		return "", fmt.Errorf("title is required")
	}
	if strings.TrimSpace(params.Content) == "" {
		return "", fmt.Errorf("content is required")
	}
	if params.KBID <= 0 {
		return "", fmt.Errorf("kb_id is required")
	}

	// 拼接 frontmatter（sources 维护编号→URL 映射）+ 正文
	fullContent := formatArticle(params.Title, params.Content, params.Sources)

	articleID, err := t.writer.CreateArticle(ctx, params.Title, fullContent, params.KBID)
	if err != nil {
		return "", fmt.Errorf("写入知识库失败: %w", err)
	}
	return fmt.Sprintf("文章已写入知识库（ID: %d，Draft 状态，待人工审核后 Published 进 RAG）", articleID), nil
}

// formatArticle 生成 frontmatter + 正文（标题已在 frontmatter 声明，正文原样拼接）。
func formatArticle(title, content string, sources []struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}) string {
	var sb strings.Builder
	// frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %s\n", title))
	sb.WriteString("source_type: deep_research\n")
	if len(sources) > 0 {
		sb.WriteString("sources:\n")
		for i, s := range sources {
			sb.WriteString(fmt.Sprintf("  - id: %d\n", i+1))
			sb.WriteString(fmt.Sprintf("    url: %s\n", s.URL))
			if s.Title != "" {
				sb.WriteString(fmt.Sprintf("    title: %s\n", s.Title))
			}
			sb.WriteString(fmt.Sprintf("    accessed: %s\n", time.Now().Format("2006-01-02")))
		}
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("---\n\n")
	// 正文原样拼接（标题已在 frontmatter 声明）
	sb.WriteString(content)
	return sb.String()
}
