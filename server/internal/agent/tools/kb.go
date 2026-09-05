// Package tools 提供 Agent 内置工具集。
//
// kb.go：知识库文章工具（SyncTool）。action 参数区分 6 操作：
// search/get/list/create/update/delete。检索封装纯检索原语（无 query_rewrite/multi_route）。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cognos/internal/agent"
	"cognos/internal/rag"

	"cognos/internal/agent/llm"
)

// KBFilter 知识库检索/列表过滤条件（frontmatter 元数据预过滤）。
type KBFilter struct {
	Type string   // guide/reference/procedure/analysis/note/faq/snippet
	Tags []string // 多维度标签
}

// KBEntry 知识库检索结果（chunk 粒度）。
type KBEntry struct {
	Content  string            `json:"content"`  // chunk 正文
	Score    float64            `json:"score"`    // 相关度
	Source   string            `json:"source"`   // 文件路径（引用追踪）
	Metadata map[string]any    `json:"metadata"` // frontmatter 元数据
}

// SearchOutcome 检索产出——entries + CRAG 充分性 verdict。
type SearchOutcome struct {
	Entries []KBEntry     // 检索结果（已 rerank + dedup + packing）
	Verdict rag.Verdict    // CRAG 评估：strong/ambiguous/weak
}

// KBArticle 完整文章（get 返回）。
type KBArticle struct {
	Frontmatter map[string]any `json:"frontmatter"` // frontmatter 元数据
	Content     string         `json:"content"`     // Markdown 正文
	FilePath    string         `json:"file_path"`   // 文件路径
}

// KBListItem 文章列表条目（含 article_id 供 kb(get) 调用）。
type KBListItem struct {
	ArticleID int64    `json:"article_id"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Tags      []string `json:"tags"`
}

// KBCreateParams 新建文章参数。
type KBCreateParams struct {
	KBID     int64
	Title    string
	Content  string   // Markdown 正文
	Type     string   // guide/reference/procedure/analysis/note/faq/snippet
	Tags     []string
	Sources  []KBSource
	System   string
	Severity string
}

// KBSource 引用来源（Agent 生成时必填）。
type KBSource struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// KBUpdateFields 更新文章字段（零值字段不更新）。
type KBUpdateFields struct {
	Title   *string
	Content *string
	Type    *string
	Tags    []string
}

// KBStore 知识库文章存储抽象（CRUD + 检索）。
// 实现包装 KnowledgeService（CRUD）与 RAG 引擎检索步骤（search）。
type KBStore interface {
	// Search 检索文章（BM25+pgvector→RRF→rerank→CRAG，返回 chunks + 充分性 verdict）。
	Search(ctx context.Context, query string, kbID int64, limit int, filter KBFilter) (SearchOutcome, error)
	// Get 按 article_id 读完整文章 + frontmatter。
	Get(ctx context.Context, kbID, articleID int64) (*KBArticle, error)
	// GetBatch 批量读文章摘要（每篇截断到 maxChars）。
	GetBatch(ctx context.Context, kbID int64, articleIDs []int64, maxChars int) ([]KBArticle, error)
	// List 分页列出文章标题（返回列表 + 总数）。
	List(ctx context.Context, kbID int64, filter KBFilter, limit, offset int) (items []KBListItem, total int, err error)
	// Create 新建 Draft 文章（质量门 + frontmatter 生成）。
	Create(ctx context.Context, params KBCreateParams) (slug string, err error)
	// Update 更新文章内容/frontmatter（增量 re-index）。
	Update(ctx context.Context, kbID int64, slug string, articleID int64, fields KBUpdateFields) error
	// Delete 删文章 + 清理 pgvector/BM25 索引。
	Delete(ctx context.Context, kbID int64, slug string, articleID int64) error
}

// KBTool 知识库文章工具（实现 agent.SyncTool）。
type KBTool struct {
	store KBStore
}

// NewKBTool 创建知识库工具。
func NewKBTool(store KBStore) *KBTool {
	return &KBTool{store: store}
}

// Info 返回工具元信息。
func (t *KBTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "kb",
		Desc: `Knowledge base operations (search/get/list/create/update/delete).
- search: RAG retrieve chunks by query; result begins with [检索充分性: strong|ambiguous|weak]. On weak, prefer web_search before answering.
- get: read one full article by article_id, or batch summaries (first 500 chars each) by article_ids.
- list: paginated titles (default 20/page) — use to browse, not to answer.
- create/update: write findings back to KB (Draft status, human review → Published).
- Do NOT use for: ticket status queries, user lookups, or non-KB data.`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"action":      {Type: llm.String, Desc: "search/get/list/create/update/delete", Required: true},
			"kb_id":       {Type: llm.Integer, Desc: "Target knowledge base ID", Required: true},
			"query":       {Type: llm.String, Desc: "Search query (action=search)"},
			"article_id":  {Type: llm.Integer, Desc: "Article ID (action=get/update/delete)"},
			"article_ids": {Type: llm.Array, Desc: "Article IDs for batch read (action=get)"},
			"title":       {Type: llm.String, Desc: "Article title (action=create/update)"},
			"content":     {Type: llm.String, Desc: "Markdown body (action=create/update)"},
			"type":        {Type: llm.String, Desc: "guide/reference/procedure/analysis/note/faq/snippet"},
			"tags":        {Type: llm.Array, Desc: "Tag list"},
			"limit":       {Type: llm.Integer, Desc: "Page size (default 20 for list, 5 for search)"},
			"offset":      {Type: llm.Integer, Desc: "Page offset (action=list, default 0)"},
		}),
	}
}

// kbParams kb 工具参数。
type kbParams struct {
	Action     string   `json:"action"`
	KBID       int64    `json:"kb_id"`
	Query      string   `json:"query,omitempty"`
	ArticleID  int64    `json:"article_id,omitempty"`
	ArticleIDs []int64  `json:"article_ids,omitempty"`
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	Type       string   `json:"type,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// Call 按 action 路由到 KBStore 方法。
func (t *KBTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var p kbParams
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if p.KBID <= 0 {
		return "", fmt.Errorf("kb_id is required")
	}

	switch p.Action {
	case "search":
		return t.doSearch(ctx, p)
	case "get":
		return t.doGet(ctx, p)
	case "list":
		return t.doList(ctx, p)
	case "create":
		return t.doCreate(ctx, p)
	case "update":
		return t.doUpdate(ctx, p)
	case "delete":
		return t.doDelete(ctx, p)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *KBTool) doSearch(ctx context.Context, p kbParams) (string, error) {
	if strings.TrimSpace(p.Query) == "" {
		return "", fmt.Errorf("query is required for action=search")
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 5
	}
	outcome, err := t.store.Search(ctx, p.Query, p.KBID, limit, KBFilter{Type: p.Type, Tags: p.Tags})
	if err != nil {
		return "", fmt.Errorf("检索失败: %w", err)
	}
	entries := outcome.Entries
	if len(entries) == 0 {
		return "无检索结果", nil
	}
	var sb strings.Builder
	// CRAG 充分性 preamble（机器可读，Agent 据此决定是否 web_search 补充）
	v := outcome.Verdict
	if v.Level != "" {
		if v.Sufficient {
			fmt.Fprintf(&sb, "[检索充分性: %s | confidence=%.2f] 检索充分，可直接基于以下内容回答。\n\n", v.Level, v.Confidence)
		} else {
			fmt.Fprintf(&sb, "[检索充分性: %s | confidence=%.2f] %s 结果可能不足，建议改写查询或调用 web_search 补充后再回答。\n\n", v.Level, v.Confidence, v.Reason)
		}
	}
	for i, e := range entries {
		fmt.Fprintf(&sb, "[%d] score=%.3f\n    %s\n    来源: %s\n", i+1, e.Score, e.Content, e.Source)
	}
	return strings.TrimSpace(sb.String()), nil
}

func (t *KBTool) doGet(ctx context.Context, p kbParams) (string, error) {
	// 批量读取：article_ids 数组
	if len(p.ArticleIDs) > 0 {
		articles, err := t.store.GetBatch(ctx, p.KBID, p.ArticleIDs, 500)
		if err != nil {
			return "", fmt.Errorf("批量读取文章失败: %w", err)
		}
		if len(articles) == 0 {
			return "无文章", nil
		}
		var sb strings.Builder
		for _, a := range articles {
			fmt.Fprintf(&sb, "--- Article id=%d: %s ---\n", a.Frontmatter["article_id"], a.Frontmatter["title"])
			sb.WriteString(a.Content)
			sb.WriteString("\n\n")
		}
		return strings.TrimSpace(sb.String()), nil
	}
	// 单篇读取：article_id
	if p.ArticleID <= 0 {
		return "", fmt.Errorf("article_id or article_ids is required for action=get")
	}
	article, err := t.store.Get(ctx, p.KBID, p.ArticleID)
	if err != nil {
		return "", fmt.Errorf("读取文章失败: %w", err)
	}
	var sb strings.Builder
	if len(article.Frontmatter) > 0 {
		sb.WriteString("---\n")
		for k, v := range article.Frontmatter {
			fmt.Fprintf(&sb, "%s: %v\n", k, v)
		}
		sb.WriteString("---\n\n")
	}
	sb.WriteString(article.Content)
	return sb.String(), nil
}

func (t *KBTool) doList(ctx context.Context, p kbParams) (string, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	items, total, err := t.store.List(ctx, p.KBID, KBFilter{Type: p.Type, Tags: p.Tags}, limit, offset)
	if err != nil {
		return "", fmt.Errorf("列出文章失败: %w", err)
	}
	if len(items) == 0 {
		return "无文章", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "共 %d 篇，显示 %d-%d：\n", total, offset+1, offset+len(items))
	for i, item := range items {
		fmt.Fprintf(&sb, "[%d] id=%d %s (%s) slug=%s\n", offset+i+1, item.ArticleID, item.Title, item.Type, item.Slug)
	}
	return strings.TrimSpace(sb.String()), nil
}

func (t *KBTool) doCreate(ctx context.Context, p kbParams) (string, error) {
	if strings.TrimSpace(p.Title) == "" {
		return "", fmt.Errorf("title is required for action=create")
	}
	if strings.TrimSpace(p.Content) == "" {
		return "", fmt.Errorf("content is required for action=create")
	}
	if p.Type == "" {
		return "", fmt.Errorf("type is required for action=create")
	}
	slug, err := t.store.Create(ctx, KBCreateParams{
		KBID:    p.KBID,
		Title:   p.Title,
		Content: p.Content,
		Type:    p.Type,
		Tags:    p.Tags,
	})
	if err != nil {
		return "", fmt.Errorf("创建文章失败: %w", err)
	}
	return fmt.Sprintf("文章已写入知识库（slug: %s，Draft 状态，待人工审核后 Published 进 RAG）", slug), nil
}

func (t *KBTool) doUpdate(ctx context.Context, p kbParams) (string, error) {
	if p.ArticleID <= 0 {
		return "", fmt.Errorf("article_id is required for action=update")
	}
	fields := KBUpdateFields{}
	if p.Title != "" {
		fields.Title = &p.Title
	}
	if p.Content != "" {
		fields.Content = &p.Content
	}
	if p.Type != "" {
		fields.Type = &p.Type
	}
	fields.Tags = p.Tags
	if err := t.store.Update(ctx, p.KBID, "", p.ArticleID, fields); err != nil {
		return "", fmt.Errorf("更新文章失败: %w", err)
	}
	return "文章已更新（增量 re-index）", nil
}

func (t *KBTool) doDelete(ctx context.Context, p kbParams) (string, error) {
	if p.ArticleID <= 0 {
		return "", fmt.Errorf("article_id is required for action=delete")
	}
	if err := t.store.Delete(ctx, p.KBID, "", p.ArticleID); err != nil {
		return "", fmt.Errorf("删除文章失败: %w", err)
	}
	return "文章已删除（索引已清理）", nil
}
