// Package tools 提供 Agent 内置工具集。
//
// kb_store_impl.go：KBStore 实现——检索复用 RAG 引擎，CRUD 包装 KnowledgeService。
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

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
	retrievalK       int // 两阶段候选池：retrieve retrievalK → rerank → 截到 limit
	evaluator        rag.SufficiencyEvaluator // CRAG 充分性评估器（nil 时跳过 verdict）
	storageRoot      string
	bucket           string
	articleSvc       *knowledge.KnowledgeService
	ingestQueue      *rag.IngestQueue // 异步索引队列（update 时入队触发增量 re-index）
}

// NewKBStoreImpl 创建 KBStore 实现。
// vectorRetriever/bm25Retriever/reranker 用于 search；articleSvc 用于 CRUD；ingestQueue 用于 update 入队。
// retrievalK 为两阶段候选池大小（< limit 时回退为 limit），0 时用 limit。
// evaluator 为 CRAG 充分性评估器（nil 时跳过 verdict，仅返回 entries）。
func NewKBStoreImpl(vr *rag.VectorRetriever, br *rag.BM25Retriever, rr adapter.Reranker, svc *knowledge.KnowledgeService, iq *rag.IngestQueue, storageRoot, bucket string, retrievalK int, evaluator rag.SufficiencyEvaluator) KBStore {
	return &kbStoreImpl{
		vectorRetriever: vr,
		bm25Retriever:   br,
		reranker:        rr,
		retrievalK:      retrievalK,
		evaluator:       evaluator,
		articleSvc:      svc,
		ingestQueue:     iq,
		storageRoot:     storageRoot,
		bucket:          bucket,
	}
}

// Search 封装纯检索原语：BM25 + pgvector → RRF 融合 → cross-encoder rerank → CRAG 评估。
// 不含 query_rewrite/multi_route——Agent ReAct 自行处理改写与多路。
// 两阶段：向量+BM25 各取 retrievalK 候选 → RRF 融合 → rerank 全池 → 截到 limit。
func (s *kbStoreImpl) Search(ctx context.Context, query string, kbID int64, limit int, filter KBFilter) (SearchOutcome, error) {
	if limit <= 0 {
		limit = 5
	}
	// 两阶段候选池：retrievalK ≥ limit，给 rerank 足够候选以提升精度
	pool := s.retrievalK
	if pool < limit {
		pool = limit
	}
	// metadata 过滤条件（Type + Tags 硬过滤，下推两路检索器）
	metaFilter := rag.MetaFilter{ArticleType: filter.Type, Tags: filter.Tags}

	// 并行检索：向量（核心，含 embedding HTTP）+ BM25（内存，<1ms）。
	// 两路皆尽力而为：单路失败仅 Warn 降级，不阻塞另一路（向量+LLM 为核心路径，双路全失败返回空）。
	var (
		vecResults   []rag.RetrievalResult
		bm25Results   []rag.RetrievalResult
		vecErr, bmErr error
	)
	var wg sync.WaitGroup
	if s.vectorRetriever != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vecResults, vecErr = s.vectorRetriever.RetrieveFiltered(ctx, query, kbID, pool, metaFilter)
		}()
	}
	if s.bm25Retriever != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm25Results, bmErr = s.bm25Retriever.RetrieveFiltered(ctx, query, kbID, pool, metaFilter)
			// BM25 分数归一化到 [0,1]
			normalizeBm25(bm25Results)
		}()
	}
	wg.Wait()
	if vecErr != nil {
		slog.Warn("向量检索失败，降级为 BM25-only", "kb_id", kbID, "error", vecErr)
	}
	if bmErr != nil {
		slog.Warn("BM25 检索失败，降级为向量-only", "kb_id", kbID, "error", bmErr)
	}

	// RRF 融合（融合到 pool，不早截——给 rerank 全部候选）
	fused := rag.HybridFuse(vecResults, bm25Results, pool)
	if len(fused) == 0 {
		return SearchOutcome{}, nil
	}

	// cross-encoder rerank（全池；reranker 为 nil 时 Rerank 透传）
	reranked, err := rag.Rerank(ctx, s.reranker, query, fused)
	if err != nil {
		slog.Warn("rerank 失败，降级为 RRF 排序结果", "error", err)
		reranked = fused
	}

	// 内容级去重，保留 Score 最高
	deduped := dedupByContent(reranked)

	// 置信度计算：精度阶段信号优先（参考 CRAG + 生产 RAG 实践）
	// - 有 rerank：ConfRaw = RerankScore（cross-encoder sigmoid ∈[0,1]，直接相关性概率）
	// - 无 rerank：ConfRaw = RRF 融合分 min-max 归一化到 [0,1]（rank-based，召回阶段信号）
	computeConfidence(deduped, s.reranker != nil && len(reranked) > 1)

	// 按 ConfRaw 降序重排（ConfRaw 为最终排序依据；sandwich 假设输入已排序）
	sortByConfRawDesc(deduped)

	// CRAG 充分性评估（在 sandwich/packing/截断前，基于完整排序结果）
	var verdict rag.Verdict
	if s.evaluator != nil {
		verdict, err = s.evaluator.Evaluate(ctx, query, deduped)
		if err != nil {
			slog.Warn("CRAG 评估失败，降级为无 verdict", "error", err)
			verdict = rag.Verdict{}
		}
	}

	// Sandwich Reorder：高分放首尾，低分放中间（Lost in the Middle 缓解）
	deduped = rag.SandwichReorder(deduped)

	// Context Packing：token 预算内贪心填充（最大化有效信息量）
	deduped = rag.PackContext(deduped, 2000)

	// 截到 limit（两阶段：rerank 全池后取 top N）
	if len(deduped) > limit {
		deduped = deduped[:limit]
	}

	// 映射为 KBEntry
	entries := make([]KBEntry, 0, len(deduped))
	for _, r := range deduped {
		entries = append(entries, KBEntry{
			Content: r.Content,
			Score:   r.ConfRaw,
			Source:  fmt.Sprintf("kb/%d/published/article-%d.md", kbID, r.ArticleID),
		})
	}
	return SearchOutcome{Entries: entries, Verdict: verdict}, nil
}

// Get 按 article_id 读完整文章 + frontmatter。
func (s *kbStoreImpl) Get(ctx context.Context, kbID int64, articleID int64) (*KBArticle, error) {
	if articleID <= 0 {
		return nil, fmt.Errorf("article_id is required")
	}
	detail, err := s.articleSvc.GetArticleDetail(ctx, articleID)
	if err != nil {
		return nil, err
	}
	return &KBArticle{
		Frontmatter: map[string]any{
			"title":      detail.Title,
			"status":     detail.StatusText,
			"article_id": detail.ID,
			"tags":       detail.Tags,
		},
		Content:  detail.Content,
		FilePath: detail.MinioPath,
	}, nil
}

// GetBatch 批量读文章摘要（每篇截断到 maxChars）。
func (s *kbStoreImpl) GetBatch(ctx context.Context, kbID int64, articleIDs []int64, maxChars int) ([]KBArticle, error) {
	if len(articleIDs) == 0 {
		return nil, fmt.Errorf("article_ids is required")
	}
	if maxChars <= 0 {
		maxChars = 500
	}
	articles := make([]KBArticle, 0, len(articleIDs))
	for _, aid := range articleIDs {
		a, err := s.Get(ctx, kbID, aid)
		if err != nil {
			continue // 单篇失败跳过
		}
		content := []rune(a.Content)
		if len(content) > maxChars {
			a.Content = string(content[:maxChars]) + "\n..."
		}
		articles = append(articles, *a)
	}
	return articles, nil
}

// List 分页列出文章标题（按 type/tags 过滤，返回列表 + 总数）。
func (s *kbStoreImpl) List(ctx context.Context, kbID int64, filter KBFilter, limit, offset int) (items []KBListItem, total int, err error) {
	if limit <= 0 {
		limit = 20
	}
	// 先查总数（status=4 Published）
	resp, err := s.articleSvc.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 10000)
	if err != nil {
		return nil, 0, err
	}
	// 过滤后取分页
	filtered := make([]KBListItem, 0)
	for _, a := range resp.Articles {
		articleType := a.ArticleType
		if articleType == "" {
			articleType = "guide"
		}
		if filter.Type != "" && filter.Type != articleType {
			continue
		}
		if len(filter.Tags) > 0 && !hasAnyTag(a.Tags, filter.Tags) {
			continue
		}
		filtered = append(filtered, KBListItem{
			ArticleID: a.ID,
			Slug:      slugify(a.Title),
			Title:     a.Title,
			Type:      articleType,
			Tags:      a.Tags,
		})
	}
	total = len(filtered)
	// 分页截取
	if offset >= total {
		return []KBListItem{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	items = filtered[offset:end]
	return items, total, nil
}

// Create 新建 Draft 文章（委托 KnowledgeService.CreateArticle）。
// Content = 正文 body（无 frontmatter）；Type → req.ArticleType（发布时若缺失由 LLM 补全）；
// Sources 作为正文末尾 ## Sources 段（自然 Markdown，非 frontmatter）。
func (s *kbStoreImpl) CreateAndPublish(ctx context.Context, params KBCreateParams) (string, error) {
	body := params.Content
	if len(params.Sources) > 0 {
		var sb strings.Builder
		sb.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n## Sources\n\n")
		for i, src := range params.Sources {
			if src.Title != "" {
				sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, src.Title, src.URL))
			} else {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, src.URL))
			}
		}
		body = sb.String()
	}
	_, err := s.articleSvc.CreateAndPublish(ctx, request.CreateArticleRequest{
		KBID:        params.KBID,
		Title:       params.Title,
		Content:     body,
		SourceType:  model.SourceTypeDeepResearch,
		ArticleType: params.Type,
		Tags:        params.Tags,
	}, 0)
	if err != nil {
		return "", err
	}
	return slugify(params.Title), nil
}

// UpdateAndRepublish 更新已发布文章并重新进入发布管道（委托 KnowledgeService.UpdateAndRepublish）。
// service 层内部触发 chunk→embed→ReplaceVectors 增量 reindex，无需 IngestQueue。
func (s *kbStoreImpl) UpdateAndRepublish(ctx context.Context, kbID int64, slug string, articleID int64, fields KBUpdateFields) error {
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
	return s.articleSvc.UpdateAndRepublish(ctx, articleID, req, 0)
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
// 检索辅助函数
// =============================================================================

// normalizeBm25 将 BM25 原始分数归一化到 [0,1]（Bm25NormScore，保留供调试/可观测）。
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

// computeConfidence 计算综合置信度 ConfRaw。
// 精度阶段信号优先（参考 CRAG：retrieval evaluator 基于相关性）：
// - rerankRan：ConfRaw = RerankScore（cross-encoder sigmoid ∈[0,1]，直接相关性概率）
// - 无 rerank：ConfRaw = RRF 融合分（Score）min-max 归一化到 [0,1]
// 召回阶段信号（cosine/BM25）与精度阶段信号（rerank）量纲不同，不混合。
func computeConfidence(chunks []rag.RetrievalResult, rerankRan bool) {
	if len(chunks) == 0 {
		return
	}
	if rerankRan {
		for i := range chunks {
			chunks[i].ConfRaw = clamp01(chunks[i].RerankScore)
		}
		return
	}
	// 无 rerank：RRF 融合分 min-max 归一化
	minS, maxS := chunks[0].Score, chunks[0].Score
	for _, c := range chunks[1:] {
		if c.Score < minS {
			minS = c.Score
		}
		if c.Score > maxS {
			maxS = c.Score
		}
	}
	span := maxS - minS
	for i := range chunks {
		if span > 0 {
			chunks[i].ConfRaw = (chunks[i].Score - minS) / span
		} else {
			chunks[i].ConfRaw = 0.8
		}
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// sortByConfRawDesc 按 ConfRaw 降序排序（ConfRaw 为最终相关性信号；sandwich 假设输入已排序）。
func sortByConfRawDesc(chunks []rag.RetrievalResult) {
	for i := 1; i < len(chunks); i++ {
		for j := i; j > 0 && chunks[j].ConfRaw > chunks[j-1].ConfRaw; j-- {
			chunks[j], chunks[j-1] = chunks[j-1], chunks[j]
		}
	}
}
