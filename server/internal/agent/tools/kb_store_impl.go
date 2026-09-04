// Package tools 提供 Agent 内置工具集。
//
// kb_store_impl.go：KBStore 实现——检索复用 RAG 引擎，CRUD 包装 KnowledgeService。
package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cognos/internal/domain/knowledge"
	"cognos/internal/infra/adapter"
	"cognos/internal/rag"
	"cognos/internal/shared/dto/request"
	"cognos/internal/shared/model"
)

// kbStoreImpl KBStore 实现：检索复用 RAG 引擎，CRUD 委托 KnowledgeService。
type kbStoreImpl struct {
	vectorRetriever *rag.VectorRetriever
	bm25Retriever    *rag.BM25Retriever
	reranker         adapter.Reranker
	articleSvc       *knowledge.KnowledgeService
}

// NewKBStoreImpl 创建 KBStore 实现。
// vectorRetriever/bm25Retriever/reranker 用于 search；articleSvc 用于 CRUD。
func NewKBStoreImpl(vr *rag.VectorRetriever, br *rag.BM25Retriever, rr adapter.Reranker, svc *knowledge.KnowledgeService) KBStore {
	return &kbStoreImpl{
		vectorRetriever: vr,
		bm25Retriever:   br,
		reranker:        rr,
		articleSvc:      svc,
	}
}

// Search 封装纯检索原语：BM25 + pgvector → RRF 融合 → cross-encoder rerank。
// 不含 query_rewrite/multi_route——Agent ReAct 自行处理改写与多路。
func (s *kbStoreImpl) Search(ctx context.Context, query string, kbID int64, limit int, filter KBFilter) ([]KBEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	// 并行检索：向量（核心）+ BM25（降级）
	vecResults, _ := s.vectorRetriever.Retrieve(ctx, query, kbID, limit)
	var bm25Results []rag.RetrievalResult
	if s.bm25Retriever != nil {
		bm25Results, _ = s.bm25Retriever.Retrieve(ctx, query, kbID, limit)
		// BM25 分数归一化到 [0,1]
		normalizeBm25(bm25Results)
	}

	// RRF 融合（空 BM25 时 HybridFuse 退化为向量截断）
	fused := rag.HybridFuse(vecResults, bm25Results, limit)
	if len(fused) == 0 {
		return nil, nil
	}

	// cross-encoder rerank（reranker 为 nil 时 Rerank 透传）
	reranked, _ := rag.Rerank(ctx, s.reranker, query, fused)

	// 内容级去重，保留 Score 最高
	deduped := dedupByContent(reranked)

	// 置信度计算（分层：cosine → +BM25 → +rerank）
	computeConfidence(deduped, len(bm25Results) > 0, s.reranker != nil && len(reranked) > 1)

	// 映射为 KBEntry
	entries := make([]KBEntry, 0, len(deduped))
	for _, r := range deduped {
		entries = append(entries, KBEntry{
			Content: r.Content,
			Score:   r.ConfRaw,
			Source:  fmt.Sprintf("kb/%d/published/article-%d.md", kbID, r.ArticleID),
		})
	}
	return entries, nil
}

// Get 按 article_id 读完整文章（slug 暂映射为 title 查找）。
func (s *kbStoreImpl) Get(ctx context.Context, kbID int64, slug string, articleID int64) (*KBArticle, error) {
	if articleID <= 0 {
		return nil, fmt.Errorf("article_id is required (slug 查找暂未实现，需 article_id)")
	}
	detail, err := s.articleSvc.GetArticleDetail(ctx, articleID)
	if err != nil {
		return nil, err
	}
	return &KBArticle{
		Frontmatter: map[string]any{
			"title":       detail.Title,
			"status":      detail.StatusText,
			"article_id":  detail.ID,
			"tags":        detail.Tags,
		},
		Content:  detail.Content,
		FilePath: detail.MinioPath,
	}, nil
}

// List 列出文章标题列表（按 type/tags 过滤）。
func (s *kbStoreImpl) List(ctx context.Context, kbID int64, filter KBFilter) ([]KBListItem, error) {
	// status=4 (Published)，sourceType=0 (全部)，processStatus="" (全部)
	resp, err := s.articleSvc.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 1000)
	if err != nil {
		return nil, err
	}
	items := make([]KBListItem, 0, len(resp.Articles))
	for _, a := range resp.Articles {
		// type 过滤（frontmatter type 暂未持久化到 DB，过滤待 type 字段持久化后启用）
		if filter.Type != "" {
			_ = filter.Type // 占位：type 过滤待 frontmatter type 持久化后启用
		}
		// tags 过滤
		if len(filter.Tags) > 0 && !hasAnyTag(a.Tags, filter.Tags) {
			continue
		}
		items = append(items, KBListItem{
			Slug:  slugify(a.Title),
			Title: a.Title,
			Type:  filter.Type,
			Tags:  a.Tags,
		})
	}
	return items, nil
}

// Create 新建 Draft 文章（委托 KnowledgeService.CreateArticle，生成完整 frontmatter）。
func (s *kbStoreImpl) Create(ctx context.Context, params KBCreateParams) (string, error) {
	// 生成完整 frontmatter（8 字段：title/type/status/created/updated/tags/source_type/sources）
	fullContent := formatArticleFrontmatter(params)
	_, err := s.articleSvc.CreateArticle(ctx, request.CreateArticleRequest{
		KBID:       params.KBID,
		Title:      params.Title,
		Content:    fullContent,
		SourceType: model.SourceTypeDeepResearch,
		Tags:       params.Tags,
	}, 0)
	if err != nil {
		return "", err
	}
	return slugify(params.Title), nil
}

// formatArticleFrontmatter 生成完整 frontmatter + 正文。
// 必填 5 字段：title/type/status/created/updated；可选：tags/sources/source_type。
func formatArticleFrontmatter(p KBCreateParams) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %s\n", p.Title))
	sb.WriteString(fmt.Sprintf("type: %s\n", p.Type))
	sb.WriteString("status: draft\n")
	now := time.Now().Format(time.RFC3339)
	sb.WriteString(fmt.Sprintf("created: %s\n", now))
	sb.WriteString(fmt.Sprintf("updated: %s\n", now))
	if len(p.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(p.Tags, ", ")))
	}
	sb.WriteString("source_type: deep_research\n")
	if len(p.Sources) > 0 {
		sb.WriteString("sources:\n")
		for i, src := range p.Sources {
			sb.WriteString(fmt.Sprintf("  - id: %d\n", i+1))
			sb.WriteString(fmt.Sprintf("    url: %s\n", src.URL))
			if src.Title != "" {
				sb.WriteString(fmt.Sprintf("    title: %s\n", src.Title))
			}
		}
	}
	if p.System != "" {
		sb.WriteString(fmt.Sprintf("system: %s\n", p.System))
	}
	if p.Severity != "" {
		sb.WriteString(fmt.Sprintf("severity: %s\n", p.Severity))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(p.Content)
	return sb.String()
}

// Update 更新文章（委托 KnowledgeService.UpdateArticle）。
func (s *kbStoreImpl) Update(ctx context.Context, kbID int64, slug string, articleID int64, fields KBUpdateFields) error {
	if articleID <= 0 {
		return fmt.Errorf("article_id is required for update")
	}
	req := request.UpdateArticleRequest{}
	if fields.Title != nil {
		req.Title = *fields.Title
	}
	if fields.Content != nil {
		req.Content = *fields.Content
	}
	if len(fields.Tags) > 0 {
		req.Tags = fields.Tags
	}
	return s.articleSvc.UpdateArticle(ctx, articleID, req)
}

// Delete 删文章（委托 KnowledgeService.DeleteArticle）。
func (s *kbStoreImpl) Delete(ctx context.Context, kbID int64, slug string, articleID int64) error {
	if articleID <= 0 {
		return fmt.Errorf("article_id is required for delete")
	}
	return s.articleSvc.DeleteArticle(ctx, articleID)
}

// hasAnyTag 判断 tags 中是否含有 target 中任一标签。
func hasAnyTag(tags, target []string) bool {
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	for _, t := range target {
		if set[t] {
			return true
		}
	}
	return false
}

// slugify 将标题转为 kebab-case slug（用于文件名与索引）。
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

var (
	nonAlnum  = regexp.MustCompile(`[^a-z0-9]+`)
	multiDash = regexp.MustCompile(`-+`)
)

// =============================================================================
// 检索辅助（内联实现，不依赖 pipeline.go 未导出函数）
// =============================================================================

const (
	bm25Weight   = 0.4 // BM25 归一化分在综合置信度中的权重
	rerankWeight = 0.6 // Cross-encoder 分在综合置信度中的权重
)

// normalizeBm25 将 BM25 原始分数归一化到 [0,1]，使 Bm25NormScore 可与 RawCosineScore 加权混合。
func normalizeBm25(results []rag.RetrievalResult) {
	if len(results) == 0 {
		return
	}
	if len(results) == 1 {
		results[0].Bm25NormScore = 0.8
		return
	}
	minS, maxS := results[0].Score, results[0].Score
	for _, r := range results[1:] {
		if r.Score < minS {
			minS = r.Score
		}
		if r.Score > maxS {
			maxS = r.Score
		}
	}
	span := maxS - minS
	for i := range results {
		if span > 0 {
			results[i].Bm25NormScore = (results[i].Score - minS) / span
		} else {
			results[i].Bm25NormScore = 0.8
		}
	}
}

// dedupByContent 按 chunk 内容文本去重，保留 Score 最高的条目。
func dedupByContent(chunks []rag.RetrievalResult) []rag.RetrievalResult {
	seen := make(map[string]int, len(chunks))
	result := make([]rag.RetrievalResult, 0, len(chunks))
	for _, c := range chunks {
		if idx, exists := seen[c.Content]; exists {
			if c.Score > result[idx].Score {
				result[idx] = c
			}
		} else {
			seen[c.Content] = len(result)
			result = append(result, c)
		}
	}
	return result
}

// computeConfidence 按管道步骤逐层计算综合置信度 ConfRaw。
func computeConfidence(chunks []rag.RetrievalResult, hybridRan, rerankRan bool) {
	for i := range chunks {
		c := &chunks[i]
		s := c.RawCosineScore
		if hybridRan && c.Bm25NormScore > 0 {
			s = (1-bm25Weight)*s + bm25Weight*c.Bm25NormScore
		}
		if rerankRan && c.RerankScore > 0 {
			s = (1-rerankWeight)*s + rerankWeight*c.RerankScore
		}
		if s < 0 {
			s = 0
		}
		if s > 1 {
			s = 1
		}
		c.ConfRaw = s
	}
}
