// Package rag 实现自建 RAG 检索引擎。
//
// rerank.go 实现 cross-encoder 驱动的候选文档重排序。
// 对应适配层为 adapter/rerank_client.go。
//
// 为什么从 LLM prompt 方案切换到 cross-encoder：
// LLM prompt 方案通过编号排序（"仅输出 3,1,2"）做重排，
// 存在三个根本问题：
//   1. 依赖 LLM 输出格式——编号顺序可能错误或缺失
//   2. 延迟高（500ms-2s），每次消耗 token
//   3. 候选内容全部塞进 prompt，无长度裁剪
// Cross-encoder 对每个 (query, passage) 对独立打分，
// 速度快（~50ms）、分数稳定、不需要 token 消耗。
//
// 降级策略：
// cross-encoder 服务不可用时降级为原始排序，
// Pipeline 在 Execute 中统一处理降级逻辑。
package rag

import (
	"context"
	"fmt"

	"opsmind/internal/adapter"
)

// Rerank 使用 cross-encoder 对候选文档按与 query 的相关性重新排序。
//
// 内部逻辑：
//  1. 单候选直接返回（无需重排）
//  2. 构建 []RerankPassage 并调用 Reranker.Rerank
//  3. 按返回 order 重新排列 candidates
//
// 重排序后将 cross-encoder 相关性分数写入每个结果的 RerankScore 字段，
// 供后续 computeConfidenceScores 加权混合使用。
//
// reranker 为 nil 时降级返回原始排序（不报错，RerankScore 保持 0）。
func Rerank(ctx context.Context, reranker adapter.Reranker, query string, candidates []RetrievalResult) ([]RetrievalResult, error) {
	if len(candidates) <= 1 {
		return candidates, nil
	}

	if reranker == nil {
		// cross-encoder 不可用时降级——保留原始排序
		return candidates, nil
	}

	// 构建 passage 列表
	passages := make([]adapter.RerankPassage, len(candidates))
	for i, c := range candidates {
		passages[i] = adapter.RerankPassage{ID: i, Text: c.Content}
	}

	result, err := reranker.Rerank(ctx, query, passages)
	if err != nil {
		return candidates, fmt.Errorf("重排序失败: %w", err)
	}

	if result == nil || len(result.Order) == 0 {
		// 子进程不可用时返回 nil result
		return candidates, nil
	}

	// 构建 order → score 的索引，用于写入 RerankScore
	orderToScore := make(map[int]float64, len(result.Order))
	for i, idx := range result.Order {
		if i < len(result.Scores) {
			orderToScore[idx] = result.Scores[i]
		}
	}

	// 按 cross-encoder 分数重排，同时写入 RerankScore
	reranked := make([]RetrievalResult, 0, len(candidates))
	for _, idx := range result.Order {
		if idx >= 0 && idx < len(candidates) {
			c := candidates[idx]
			if score, ok := orderToScore[idx]; ok {
				c.RerankScore = score
			}
			reranked = append(reranked, c)
		}
	}

	// 补充未被返回的候选（兜底，RerankScore 保持 0）
	seen := make(map[int]bool)
	for _, idx := range result.Order {
		seen[idx] = true
	}
	for i, c := range candidates {
		if !seen[i] {
			reranked = append(reranked, c)
		}
	}

	if len(reranked) == 0 {
		return candidates, nil
	}
	return reranked, nil
}
