// Package rag 实现自建 RAG 检索引擎。
//
// hybrid.go 实现 RRF（Reciprocal Rank Fusion）混合融合算法。
package rag

import (
	"sort"
)

const (
	// rrfK RRF 平滑常数，标准值 60。
	rrfK = 60
)

// HybridFuse 使用 RRF 融合向量检索和 BM25 检索结果。
// 融合时保留 RawCosineScore 与 Bm25NormScore，由 KBStore.Search 后续加权混合为置信度。
func HybridFuse(vectorResults, bm25Results []RetrievalResult, topK int) []RetrievalResult {
	if len(vectorResults) == 0 && len(bm25Results) == 0 {
		return nil
	}

	// 单路结果直接截断到 topK
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

	// 向量路径贡献：保留 RawCosineScore
	type fusedEntry struct {
		score  float64
		result RetrievalResult
	}
	fused := make(map[int64]*fusedEntry)

	for rank, r := range vectorResults {
		rrfScore := 1.0 / (float64(rrfK) + float64(rank+1))
		if entry, exists := fused[r.ChunkID]; exists {
			entry.score += rrfScore
		} else {
			fused[r.ChunkID] = &fusedEntry{
				score:  rrfScore,
				result: r, // 保留向量结果的 RawCosineScore
			}
		}
	}

	// BM25 路径贡献：保留 Bm25NormScore，与已存在的向量结果合并
	for rank, r := range bm25Results {
		rrfScore := 1.0 / (float64(rrfK) + float64(rank+1))
		if entry, exists := fused[r.ChunkID]; exists {
			entry.score += rrfScore
			// chunk 在两路都命中：合并 BM25 归一化分到向量结果上
			entry.result.Bm25NormScore = r.Bm25NormScore
		} else {
			fused[r.ChunkID] = &fusedEntry{
				score:  rrfScore,
				result: r, // BM25-only chunk：RawCosineScore=0, Bm25NormScore 已有值
			}
		}
	}

	sorted := make([]RetrievalResult, 0, len(fused))
	for _, entry := range fused {
		r := entry.result
		r.Score = entry.score
		r.Source = "hybrid"
		sorted = append(sorted, r)
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	if len(sorted) > topK {
		sorted = sorted[:topK]
	}
	return sorted
}
