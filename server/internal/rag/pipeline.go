// Package rag 实现自建 RAG 检索引擎。
//
// pipeline.go 实现 RAG 管道编排器：查询改写 → 多路检索 → 混合检索 → 重排序。
// 向量检索为核心路径不可降级；查询改写/多路/重排序失败降级继续，BM25 失败仅用向量结果。
package rag

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"opsmind/internal/infra/adapter"
)

// =============================================================================
// Pipeline — RAG 管道编排器
// =============================================================================

// Pipeline 组装 RAG 管道各步骤，按序执行检索流程。
type Pipeline struct {
	vectorRetriever Retriever         // 向量检索器（不可为 nil）
	bm25Retriever   Retriever         // BM25 检索器（nil 表示不启用）
	llmClient       LLMClient // LLM 客户端（查询改写/多路）
	reranker        adapter.Reranker  // 重排序器（nil 时降级跳过）
	embedder        TextEmbedder      // 向量嵌入器（问题 embedding 缓存）
	fusion          FusionStrategy    // 混合融合策略（默认 RRF k=60）
	llmModel        string            // LLM 模型名称
}

// NewPipeline 创建 Pipeline 实例。vectorRet 不可为 nil，bm25Ret/reranker 可为 nil（降级跳过）。
func NewPipeline(vectorRet, bm25Ret Retriever, llm LLMClient, emb TextEmbedder, reranker adapter.Reranker, llmModel string) *Pipeline {
	return &Pipeline{
		vectorRetriever: vectorRet,
		bm25Retriever:   bm25Ret,
		llmClient:       llm,
		reranker:        reranker,
		embedder:        emb,
		fusion:          &rrfFusion{},
		llmModel:        llmModel,
	}
}

// =============================================================================
// Execute — 管道主入口
// =============================================================================

// Execute 执行 RAG 管道，返回检索结果和管道指标。
// 向量检索失败直接返回错误；查询改写/多路/重排序在依赖为 nil 时静默跳过。
// Rerank 使用原始 query 评估相关性（路由查询可能偏离用户意图）。
func (p *Pipeline) Execute(ctx context.Context, query string, kbID int64, opts RAGOptions, onStep StepCallback) (*RAGResult, error) {
	// 入口规范化：零值字段使用默认值，保证后续步骤无需单独处理零值
	opts.Normalize()

	// DisableRetrieval：跳过全部检索管道，仅返回空结果。
	// 用于纯 LLM 对话模式（管理员配置 ai.rag_enabled=false 时生效）。
	if opts.DisableRetrieval {
		return &RAGResult{Chunks: nil, Metrics: PipelineMetrics{TotalDurationMS: 0}}, nil
	}

	start := time.Now()
	metrics := PipelineMetrics{}
	var steps []StepMetric

	// 追踪步骤用时
	track := func(stepID, label string, fn func() error) error {
		stepStart := time.Now()
		if onStep != nil {
			onStep(StepEvent{Type: "step", ID: stepID, Label: label})
		}
		err := fn()
		dur := time.Since(stepStart).Milliseconds()
		sm := StepMetric{
			StepID:     stepID,
			Label:      label,
			DurationMS: dur,
			Success:    err == nil,
		}
		if err != nil {
			sm.Error = err.Error()
		}
		steps = append(steps, sm)
		return err
	}

	// ─── Step 1: 查询改写 ───
	rewrittenQuery := query
	if opts.QueryRewrite && p.llmClient != nil {
		_ = track("query_rewrite", "查询改写", func() error {
			rw, err := QueryRewrite(ctx, p.llmClient, p.llmModel, query, opts.History)
			if err != nil {
				return err
			}
			rewrittenQuery = rw
			return nil
		})
	}
	// ─── 缓存 question embedding，供 S_qa 复用（改写后 query 的 embedding） ───
	var questionEmbedding []float32
	if p.embedder != nil {
		vecs, _, err := p.embedder.Embed(ctx, []string{rewrittenQuery}, "")
		if err == nil && len(vecs) > 0 {
			questionEmbedding = vecs[0]
		}
	}

	// ─── Step 2: 多路检索 ───
	routes := []string{rewrittenQuery}
	if opts.MultiRoute && opts.RouteCount > 1 && p.llmClient != nil {
		_ = track("multi_route", "多路检索", func() error {
			rts, err := MultiRoute(ctx, p.llmClient, p.llmModel, rewrittenQuery, opts.RouteCount)
			if err != nil {
				return err
			}
			routes = rts
			return nil
		})
	}

	// ─── Step 3: 检索（向量 + 可选 BM25） ───
	var allChunks []RetrievalResult
	hybridRan := false // BM25 是否实际产出结果（用于置信度计算）
	rerankRan := false // 重排序是否实际执行（用于置信度计算）
	if opts.Hybrid && p.bm25Retriever != nil {
		var vectorResults, bm25Results []RetrievalResult

		// 3a: 向量检索（核心路径—失败不可降级，含多路均值）
		err := track("vector_retrieve", "向量检索", func() error {
			multiRoute := opts.MultiRoute && opts.RouteCount > 1 && len(routes) > 1
			if multiRoute {
				vectorResults = retrieveMultiRoute(ctx, p.vectorRetriever, routes, kbID, opts.TopK)
			} else {
				for _, route := range routes {
					results, err := p.vectorRetriever.Retrieve(ctx, route, kbID, opts.TopK)
					if err != nil {
						return err
					}
					vectorResults = append(vectorResults, results...)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("向量检索失败: %w", err)
		}

		// 3b: BM25 检索（降级—失败不阻塞）
		_ = track("bm25_retrieve", "BM25 检索", func() error {
			for _, route := range routes {
				results, err := p.bm25Retriever.Retrieve(ctx, route, kbID, opts.TopK)
				if err != nil {
					return err
				}
				bm25Results = append(bm25Results, results...)
			}
			return nil
		})

		// BM25 分数归一化到 [0,1]，使 Bm25NormScore 可与 RawCosineScore 混合
		if len(bm25Results) > 0 {
			normalizeBm25Scores(bm25Results)
			hybridRan = true
		}

		// 3c: RRF 融合（内部携带 Bm25NormScore 到融合结果）
		fuseErr := track("hybrid_fuse", "混合融合", func() error {
			allChunks = p.fusion.Fuse(vectorResults, bm25Results, opts.RerankCount)
			if len(allChunks) == 0 {
				return fmt.Errorf("混合融合后无结果")
			}
			return nil
		})
		if fuseErr != nil && len(vectorResults) == 0 && len(bm25Results) == 0 {
			return nil, fmt.Errorf("混合检索无结果: %w", fuseErr)
		}
		if len(allChunks) == 0 && len(vectorResults) > 0 {
			allChunks = dedupChunks(vectorResults)
		} else if len(allChunks) == 0 && len(bm25Results) > 0 {
			allChunks = dedupChunks(bm25Results)
		}
	} else {
		// 纯向量模式（含多路均值）
		err := track("vector_retrieve", "向量检索", func() error {
			multiRoute := opts.MultiRoute && opts.RouteCount > 1 && len(routes) > 1
			if multiRoute {
				allChunks = retrieveMultiRoute(ctx, p.vectorRetriever, routes, kbID, opts.TopK)
			} else {
				for _, route := range routes {
					results, err := p.vectorRetriever.Retrieve(ctx, route, kbID, opts.TopK)
					if err != nil {
						return err
					}
					allChunks = append(allChunks, results...)
				}
			}
			if len(allChunks) == 0 {
				return fmt.Errorf("向量检索无结果")
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("向量检索失败: %w", err)
		}
	}

	// ─── Step 4: 重排序 ───
	if opts.Rerank && len(allChunks) > 1 && p.reranker != nil {
		// 按 RerankCount 截断候选池，避免 cross-encoder 候选过多影响延迟
		candidates := allChunks
		if len(candidates) > opts.RerankCount {
			candidates = candidates[:opts.RerankCount]
		}

		slog.Info("开始重排序", "候选数", len(candidates), "query", query)
		_ = track("rerank", "重排序", func() error {
			// 使用原始 query 评估相关性（最能代表用户意图）
			reranked, err := Rerank(ctx, p.reranker, query, candidates)
			if err != nil {
				slog.Warn("重排序失败，降级为原始排序", "query", query, "候选数", len(candidates), "error", err)
				return err
			}
			allChunks = reranked
			rerankRan = true
			slog.Info("重排序完成", "结果数", len(reranked))
			return nil
		})
	}

	// 内容级去重：不同 ChunkID 可能含相同文本，按内容去重保留最高 Score
	allChunks = dedupByContent(allChunks)

	if len(allChunks) > opts.TopK {
		allChunks = allChunks[:opts.TopK]
	}

	// 计算综合置信度：按管道步骤逐层组合 S_qa / BM25 / Rerank
	computeConfidenceScores(allChunks, hybridRan, rerankRan)

	// 生成前端展示用的 chunk 分（基于 ConfRaw）
	chunkDisplays := computeDisplayScores(allChunks)

	metrics.Steps = steps
	metrics.TotalDurationMS = time.Since(start).Milliseconds()

	// 记录管道执行总结：异常步骤数 + 检索结果数 + 总耗时
	failCount := 0
	for _, s := range steps {
		if !s.Success {
			failCount++
		}
	}
	slog.Info("RAG 管道执行完成", "kb_id", kbID, "chunks", len(allChunks),
		"steps", len(steps), "failures", failCount, "latency_ms", metrics.TotalDurationMS)

	return &RAGResult{
		Chunks:            allChunks,
		ChunkDisplays:     chunkDisplays,
		QuestionEmbedding: questionEmbedding,
		Metrics:           metrics,
	}, nil
}

// computeConfidenceScores 按管道步骤逐层计算每个 chunk 的综合置信度 ConfRaw：
//
//	Layer 0: S = RawCosineScore
//	Layer 1: if hybrid: S = (1-α)*S + α*Bm25NormScore   (α=0.4)
//	Layer 2: if rerank: S = (1-β)*S + β*RerankScore     (β=0.6)
//
// 未运行的步骤对应字段为 0，该层退化为恒等。
func computeConfidenceScores(chunks []RetrievalResult, hybridRan, rerankRan bool) {
	const (
		bm25Weight   = 0.4 // BM25 归一化分在综合置信度中的权重
		rerankWeight = 0.6 // Cross-encoder 分在综合置信度中的权重
	)

	for i := range chunks {
		c := &chunks[i]
		s := c.RawCosineScore // Layer 0: S_qa 基座

		// Layer 1: BM25 混合（混合检索运行且该 chunk 有分时生效）
		if hybridRan && c.Bm25NormScore > 0 {
			s = (1-bm25Weight)*s + bm25Weight*c.Bm25NormScore
		}

		// Layer 2: 重排序修正（重排序运行且该 chunk 有分时生效）
		if rerankRan && c.RerankScore > 0 {
			s = (1-rerankWeight)*s + rerankWeight*c.RerankScore
		}

		// 精度钳位
		if s < 0 {
			s = 0
		}
		if s > 1 {
			s = 1
		}
		c.ConfRaw = s
	}
}

// computeDisplayScores 从 ConfRaw 生成前端展示用的 chunk 分数，不做批次归一化（ConfRaw 跨批次可比）。
func computeDisplayScores(chunks []RetrievalResult) []ChunkDisplay {
	displays := make([]ChunkDisplay, len(chunks))
	for i, c := range chunks {
		displays[i] = ChunkDisplay{
			ID:     c.ChunkID,
			Score:  c.ConfRaw,
			Source: fmt.Sprintf("来源 %d", i+1),
		}
	}
	return displays
}

// retrieveMultiRoute 执行多路向量检索并对同一 chunk 的 RawCosineScore 取均值，避免单 route 噪声分失真。
func retrieveMultiRoute(ctx context.Context, retriever Retriever, routes []string, kbID int64, topK int) []RetrievalResult {
	type scoreAccum struct {
		sum   float64
		count int
	}
	chunkScores := make(map[int64]*scoreAccum)
	var allResults []RetrievalResult

	for _, route := range routes {
		results, err := retriever.Retrieve(ctx, route, kbID, topK)
		if err != nil {
			continue // 单路失败降级跳过
		}
		for _, r := range results {
			acc := chunkScores[r.ChunkID]
			if acc == nil {
				acc = &scoreAccum{}
				chunkScores[r.ChunkID] = acc
			}
			acc.sum += r.RawCosineScore
			acc.count++
		}
		allResults = append(allResults, results...)
	}

	// 多路均值：同一 chunk 被多条 route 命中时取均分
	for i := range allResults {
		acc := chunkScores[allResults[i].ChunkID]
		if acc != nil && acc.count > 1 {
			allResults[i].RawCosineScore = acc.sum / float64(acc.count)
		}
	}

	return allResults
}

// normalizeBm25Scores 将 BM25 原始分数归一化到 [0,1]，使 Bm25NormScore 可与 RawCosineScore 加权混合。
// 批次内 max-min 归一化；单结果时赋予 0.8（保守估计）。
func normalizeBm25Scores(results []RetrievalResult) {
	if len(results) == 0 {
		return
	}
	if len(results) == 1 {
		results[0].Bm25NormScore = 0.8
		return
	}

	// 找到批次内的最大最小值
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
			results[i].Bm25NormScore = 0.8 // 所有分数相同，保守估计
		}
	}
}

// dedupChunks 按 ChunkID 去重，保留首次出现的结果。
func dedupChunks(chunks []RetrievalResult) []RetrievalResult {
	seen := make(map[int64]bool, len(chunks))
	result := make([]RetrievalResult, 0, len(chunks))
	for _, c := range chunks {
		if !seen[c.ChunkID] {
			seen[c.ChunkID] = true
			result = append(result, c)
		}
	}
	return result
}

// dedupByContent 按 chunk 内容文本去重，保留 Score 最高的条目。
// 捕获 HybridFuse 按 ChunkID 合并无法去除的内容相同但 ChunkID 不同的重复。时间复杂度 O(n)。
func dedupByContent(chunks []RetrievalResult) []RetrievalResult {
	seen := make(map[string]int, len(chunks)) // content → 在 result 中的索引
	result := make([]RetrievalResult, 0, len(chunks))
	for _, c := range chunks {
		if idx, exists := seen[c.Content]; exists {
			// 保留 Score 更高的（RRF 融合分或重排序后的分）
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
