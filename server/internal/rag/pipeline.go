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

	// ─── 缓存原始 query 的 embedding，供 S_qa 计算复用 ───
	var questionEmbedding []float32
	if p.embedder != nil {
		vecs, _, err := p.embedder.Embed(ctx, []string{query}, "")
		if err == nil && len(vecs) > 0 {
			questionEmbedding = vecs[0]
		}
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
	var allChunks []RetrievalResult
	if opts.Hybrid && p.bm25Retriever != nil {
		var vectorResults, bm25Results []RetrievalResult

		// 3a: 向量检索（核心路径—失败不可降级）
		err := track("vector_retrieve", "向量检索", func() error {
			for _, route := range routes {
				results, err := p.vectorRetriever.Retrieve(ctx, route, kbID, opts.TopK)
				if err != nil {
					return err
				}
				vectorResults = append(vectorResults, results...)
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

		// 3c: RRF 融合
		fuseErr := track("hybrid_fuse", "混合融合", func() error {
			allChunks = HybridFuse(vectorResults, bm25Results, opts.RerankCount)
			if len(allChunks) == 0 {
				return fmt.Errorf("混合融合后无结果")
			}
			return nil
		})
		if fuseErr != nil && len(vectorResults) == 0 && len(bm25Results) == 0 {
			// 两路都为空时才传播错误（否则融合失败不影响已有结果）
			return nil, fmt.Errorf("混合检索无结果: %w", fuseErr)
		}
		if len(allChunks) == 0 && len(vectorResults) > 0 {
			allChunks = dedupChunks(vectorResults)
		} else if len(allChunks) == 0 && len(bm25Results) > 0 {
			allChunks = dedupChunks(bm25Results)
		}
	} else {
		// 纯向量模式
		err := track("vector_retrieve", "向量检索", func() error {
			for _, route := range routes {
				results, err := p.vectorRetriever.Retrieve(ctx, route, kbID, opts.TopK)
				if err != nil {
					return err
				}
				allChunks = append(allChunks, results...)
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
			slog.Info("重排序完成", "结果数", len(reranked))
			return nil
		})
	}

	// 截取 TopK
	if len(allChunks) > opts.TopK {
		allChunks = allChunks[:opts.TopK]
	}

	// 计算 chunk 展示分（批次内 min-max 归一化，仅用于前端进度条）
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

// computeDisplayScores 对检索结果做批次内 min-max 归一化，生成前端展示用的分数。
//
// 归一化到 [0,1] 后每个 chunk 的展示分仅表示"在当前检索批次中的相对位置"，
// 不参与 Conf_raw 计算，不落库。
func computeDisplayScores(chunks []RetrievalResult) []ChunkDisplay {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return []ChunkDisplay{{ID: chunks[0].ChunkID, Score: 1.0, Source: fmt.Sprint(chunks[0].ArticleID)}}
	}

	// 找批次内 min/max
	minS, maxS := chunks[0].Score, chunks[0].Score
	for _, c := range chunks[1:] {
		if c.Score < minS {
			minS = c.Score
		}
		if c.Score > maxS {
			maxS = c.Score
		}
	}

	displays := make([]ChunkDisplay, len(chunks))
	span := maxS - minS
	for i, c := range chunks {
		score := 1.0
		if span > 0 {
			score = (c.Score - minS) / span
		}
		displays[i] = ChunkDisplay{
			ID:     c.ChunkID,
			Score:  score,
			Source: fmt.Sprint(c.ArticleID),
		}
	}
	return displays
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
