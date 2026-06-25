// Package rag 实现自建 RAG 检索引擎。
//
// pipeline.go 实现 RAG 管道编排器（Pipeline）。
//
// 管道执行流程（按序）：
//  1. QueryRewrite    — LLM 改写口语化查询
//  2. MultiRoute      — LLM 生成多路子查询
//  3. HybridRetrieve  — 向量检索 + BM25 → RRF 融合（可选）
//  4. Rerank          — cross-encoder 重排序候选
//
// 每步失败按降级矩阵处理：
//   - 查询改写/多路路由/重排序失败 → 降级继续（不阻塞）
//   - 向量检索失败（无 BM25 降级）→ 返回错误（核心路径不可降级）
//   - BM25 失败 → 仅用向量结果
//
// 设计决策（ADR-V2-001）：
// Pipeline 通过 Retriever 接口调用向量检索和 BM25，
// 不直接依赖 VectorStore 或 BM25Retriever 的具体实现。
// 这样 BM25 懒加载和 TTL 对 Pipeline 完全透明。
package rag

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"opsmind/internal/adapter"
)

// =============================================================================
// Pipeline — RAG 管道编排器
// =============================================================================

// Pipeline 组装 RAG 管道各步骤，按序执行检索流程。
type Pipeline struct {
	vectorRetriever Retriever           // 向量检索器（通过 Embedder+VectorStore 实现，不可为 nil）
	bm25Retriever   Retriever           // BM25 检索器（可为 nil，表示不启用 BM25）
	llmClient       adapter.LLMClient   // LLM 客户端（查询改写/多路）
	reranker        adapter.Reranker    // Cross-encoder 重排序器（可为 nil，nil 时降级跳过）
	embedder        *Embedder           // 向量嵌入器
	llmModel        string              // LLM 模型名称（查询改写/多路同步调用需要）
}

// NewPipeline 创建 Pipeline 实例。
//
// vectorRet 不可为 nil。bm25Ret 可以为 nil（不启用 BM25 混合检索）。
// reranker 可以为 nil（不启用重排序或降级为 LLM prompt 方案）。
// llmModel 用于查询改写和多路检索的同步 LLM 调用（与流式生成的模型相同）。
func NewPipeline(vectorRet, bm25Ret Retriever, llm adapter.LLMClient, emb *Embedder, reranker adapter.Reranker, llmModel string) *Pipeline {
	return &Pipeline{
		vectorRetriever: vectorRet,
		bm25Retriever:   bm25Ret,
		llmClient:       llm,
		reranker:        reranker,
		embedder:        emb,
		llmModel:        llmModel,
	}
}

// =============================================================================
// Execute — 管道主入口
// =============================================================================

// Execute 执行 RAG 管道，返回检索结果和管道指标。
//
// kbID 为知识库 ID，opts 控制各步骤开关和参数。
//
// 步骤降级策略：
//   - 查询改写 / 多路检索：llmClient == nil 时静默跳过
//   - 重排序：reranker == nil 时静默跳过
//   - 向量检索：核心路径，失败直接返回错误
//   - BM25 检索：失败降级为仅向量结果
//
// Rerank 使用原始 query（而非改写后的查询）评估相关性，
// 原因是多路检索生成的路由查询可能偏离用户原始意图。
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
	// ─── 缓存 question embedding，供 S_qa 计算复用 ───
	// 放在查询改写之后：开启改写时使用改写后 query 的 embedding，
	// 使 S_qa 问答匹配分与检索实际使用的查询对齐。
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
	// Retriever 接口内部处理 embedding 生成，Pipeline 不直接调用 Embedder。
	//
	// 多路检索置信度策略：
	// 同一 chunk 被多条 route 命中时，RawCosineScore 取所有命中的均值，
	// 避免频繁出现的 chunk 因某条 route 的噪声分而失真。
	var allChunks []RetrievalResult
	hybridRan := false // 记录 BM25 是否实际产出结果（用于置信度计算）
	rerankRan := false // 记录重排序是否实际执行（用于置信度计算）
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
			allChunks = HybridFuse(vectorResults, bm25Results, opts.RerankCount)
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
		// 重排序前按 RerankCount 截断候选池，避免多路检索结果过多时
		// 送入 cross-encoder 的候选过多影响延迟
		candidates := allChunks
		if len(candidates) > opts.RerankCount {
			candidates = candidates[:opts.RerankCount]
		}

		slog.Info("开始重排序", "候选数", len(candidates), "query", query)
		_ = track("rerank", "重排序", func() error {
			// 使用原始 query：cross-encoder 评估 query-document 对的相关性，
			// 原始 query 最能代表用户真实意图
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

	// 内容级去重：多路检索 + 混合融合后，不同 ChunkID 可能包含相同文本。
	// 按内容去重保留最高 Score 的条目，避免前端展示重复内容和 LLM 收到重复上下文。
	allChunks = dedupByContent(allChunks)

	// 截取 TopK
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

// computeConfidenceScores 按管道步骤逐层计算每个 chunk 的综合置信度 ConfRaw。
//
// 采用分层组合公式，每层基于前一层的输出叠加：
//
//	Layer 0: S_qa = RawCosineScore                     ← 基座（已含查询改写+多路均值）
//	Layer 1: if hybrid: S = (1-α)*S + α*Bm25NormScore   ← BM25 混合，α=0.4
//	Layer 2: if rerank: S = (1-β)*S + β*RerankScore     ← 重排序修正，β=0.6
//
// 任意步骤组合均可兼容——未运行的步骤对应字段为 0，该层退化为恒等。
//
// 权重设计依据：
//   - BM25 权重 0.4：稀疏检索辅助信号，不应主导稠密向量结果
//   - Rerank 权重 0.6：cross-encoder 对相关性判断更准确，给予更高权重
func computeConfidenceScores(chunks []RetrievalResult, hybridRan, rerankRan bool) {
	const (
		bm25Weight  = 0.4 // BM25 归一化分在综合置信度中的权重
		rerankWeight = 0.6 // Cross-encoder 分在综合置信度中的权重
	)

	for i := range chunks {
		c := &chunks[i]
		s := c.RawCosineScore // Layer 0: S_qa 基座

		// Layer 1: BM25 混合（仅当混合检索实际运行且该 chunk 有 BM25 分时生效）
		if hybridRan && c.Bm25NormScore > 0 {
			s = (1-bm25Weight)*s + bm25Weight*c.Bm25NormScore
		}

		// Layer 2: 重排序修正（仅当重排序实际运行且该 chunk 有 cross-encoder 分时生效）
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

// computeDisplayScores 从 ConfRaw 生成前端展示用的 chunk 分数。
//
// 每个 chunk 的 ShowScore 等于其 ConfRaw（综合置信度 [0,1]），
// 不做批次归一化——ConfRaw 本身就是跨批次可比的统一量纲。
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

// retrieveMultiRoute 执行多路向量检索并对同一 chunk 的 RawCosineScore 取均值。
//
// 多路检索时同一 chunk 可能被多条 route 命中，每次命中的余弦相似度不同。
// 取均值避免频繁出现的 chunk 因某条 route 的噪声分而失真，
// 同时保留"被多条 route 命中"这一正向信号。
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

	// 多路均值：同一 chunk 出现在多条 route 中时取均分
	for i := range allResults {
		acc := chunkScores[allResults[i].ChunkID]
		if acc != nil && acc.count > 1 {
			allResults[i].RawCosineScore = acc.sum / float64(acc.count)
		}
	}

	return allResults
}

// normalizeBm25Scores 将 BM25 原始分数归一化到 [0,1] 区间。
//
// BM25 分数无上界且量纲与余弦相似度不同，归一化后使 Bm25NormScore
// 可与 RawCosineScore 在同一公式中加权混合。
// 使用批次内 max-min 归一化：norm = (score - min) / (max - min)，
// 单结果时直接赋予 0.8（BM25 命中但无法归一化时的保守估计）。
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
//
// 多路检索和混合融合回退路径可能产生同一 chunk 的重复条目，
// 去重避免 LLM 收到重复上下文浪费 token 预算。
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
//
// 场景：文档处理时可能产生内容重叠的多个 chunk（不同 ChunkID 但相同文本），
// 多路检索会将这些 chunk 全部召回。HybridFuse 按 ChunkID 合并无法捕获此类重复。
// 去重避免前端展示重复条目和 LLM 上下文浪费 token。
// 时间复杂度 O(n)，内容作为 map key 不额外哈希。
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
