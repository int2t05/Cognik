// Package rag 实现自建 RAG 检索引擎。
//
// hybrid.go 实现 RRF（Reciprocal Rank Fusion）混合融合算法。
//
// RRF 是一种无需训练的无监督融合算法，将多路检索结果按排名位置融合：
//
//	RRF_score(d) = Σ 1/(k + rank_i(d))
//
// 其中 k=60 是经验常数（Cormack et al., 2009），
// rank_i(d) 是文档 d 在第 i 路检索中的排名（从 1 开始）。
//
// 为什么选择 RRF 而非线性加权：
// RRF 不需要对两路分数做归一化（向量 cosine 和 BM25 的量纲不同），
// 天然处理分数不可比问题，且对异常值不敏感。
package rag

import (
	"sort"
)

// =============================================================================
// RRF 混合融合
// =============================================================================

const (
	// rrfK RRF 算法的平滑常数。
	// k=60 是学术界标准值，降低 k 会增强高位排名的影响力。
	rrfK = 60
)

// HybridFuse 使用 RRF 融合向量检索和 BM25 检索结果。
//
// 两路均有结果时计算 RRF 分数，降序排列后截取 topK。
// 仅单路有结果时直接截断到 topK 后返回。
func HybridFuse(vectorResults, bm25Results []RetrievalResult, topK int) []RetrievalResult {
	if len(vectorResults) == 0 && len(bm25Results) == 0 {
		return nil
	}

	// 仅单路有结果时直接截断到 topK
	if len(vectorResults) == 0 {
		n := min(len(bm25Results), topK)
		result := make([]RetrievalResult, n)
		for i := range n {
			result[i] = bm25Results[i]
			result[i].Source = "hybrid"
		}
		return result
	}
	if len(bm25Results) == 0 {
		n := min(len(vectorResults), topK)
		result := make([]RetrievalResult, n)
		for i := range n {
			result[i] = vectorResults[i]
			result[i].Source = "hybrid"
		}
		return result
	}

	// 计算 RRF 分数
	// map[chunkID] → {rrfScore, index, *result}
	type fusedEntry struct {
		score  float64
		result RetrievalResult
	}
	fused := make(map[int64]*fusedEntry)

	// 向量路径贡献
	for rank, r := range vectorResults {
		rrfScore := 1.0 / (float64(rrfK) + float64(rank+1))
		if entry, exists := fused[r.ChunkID]; exists {
			entry.score += rrfScore
		} else {
			fused[r.ChunkID] = &fusedEntry{
				score:  rrfScore,
				result: r,
			}
		}
	}

	// BM25 路径贡献
	for rank, r := range bm25Results {
		rrfScore := 1.0 / (float64(rrfK) + float64(rank+1))
		if entry, exists := fused[r.ChunkID]; exists {
			entry.score += rrfScore
			// 合并内容（优先保留向量结果的内容，通常更完整）
		} else {
			fused[r.ChunkID] = &fusedEntry{
				score:  rrfScore,
				result: r,
			}
		}
	}

	// 转换为排序切片
	sorted := make([]RetrievalResult, 0, len(fused))
	for _, entry := range fused {
		r := entry.result
		r.Score = entry.score
		r.Source = "hybrid"
		sorted = append(sorted, r)
	}

	// 降序排列
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// 截取 topK
	if len(sorted) > topK {
		sorted = sorted[:topK]
	}
	return sorted
}
