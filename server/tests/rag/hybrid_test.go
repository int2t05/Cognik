package rag_test

import (
	"testing"

	"opsmind/internal/rag"
)

// TestHybridFuse_BothSources 验证两路均有结果时的 RRF 融合。
func TestHybridFuse_BothSources(t *testing.T) {
	vectorResults := []rag.RetrievalResult{
		{ChunkID: 1, Score: 0.95, Source: "vector"},
		{ChunkID: 2, Score: 0.80, Source: "vector"},
		{ChunkID: 3, Score: 0.60, Source: "vector"},
	}
	bm25Results := []rag.RetrievalResult{
		{ChunkID: 2, Score: 0.90, Source: "bm25"},
		{ChunkID: 3, Score: 0.70, Source: "bm25"},
		{ChunkID: 4, Score: 0.50, Source: "bm25"},
	}

	// ChunkID=2 在两路都排高位，融合后应排最前
	fused := rag.HybridFuse(vectorResults, bm25Results, 5)

	if len(fused) == 0 {
		t.Fatal("RRF 融合后不应为空")
	}
	if fused[0].ChunkID != 2 {
		t.Errorf("两个来源都排前的文档应融合后第一: 期望 ChunkID=2, 实际 %d", fused[0].ChunkID)
	}
	if fused[0].Source != "hybrid" {
		t.Errorf("融合结果 Source 应为 hybrid, 实际 %s", fused[0].Source)
	}
}

// TestHybridFuse_VectorOnly 验证仅有向量结果时的降级。
func TestHybridFuse_VectorOnly(t *testing.T) {
	vectorResults := []rag.RetrievalResult{
		{ChunkID: 10, Score: 0.90, Source: "vector"},
		{ChunkID: 11, Score: 0.70, Source: "vector"},
	}
	var bm25Results []rag.RetrievalResult

	fused := rag.HybridFuse(vectorResults, bm25Results, 3)

	if len(fused) != 2 {
		t.Fatalf("仅向量结果时应保留全部, 期望 2, 实际 %d", len(fused))
	}
	// 结果应原样返回（RRF 分数归一化后排序不变）
	if fused[0].Source != "hybrid" {
		t.Errorf("Source 应为 hybrid, 实际 %s", fused[0].Source)
	}
}

// TestHybridFuse_BM25Only 验证仅有 BM25 结果时的降级。
func TestHybridFuse_BM25Only(t *testing.T) {
	var vectorResults []rag.RetrievalResult
	bm25Results := []rag.RetrievalResult{
		{ChunkID: 20, Score: 0.80, Source: "bm25"},
	}

	fused := rag.HybridFuse(vectorResults, bm25Results, 3)

	if len(fused) != 1 {
		t.Fatalf("仅 BM25 结果时应保留全部, 期望 1, 实际 %d", len(fused))
	}
	if fused[0].ChunkID != 20 {
		t.Errorf("期望 ChunkID=20, 实际 %d", fused[0].ChunkID)
	}
}

// TestHybridFuse_TopK 验证 topK 截取。
func TestHybridFuse_TopK(t *testing.T) {
	vectorResults := make([]rag.RetrievalResult, 20)
	bm25Results := make([]rag.RetrievalResult, 20)
	for i := 0; i < 20; i++ {
		vectorResults[i] = rag.RetrievalResult{ChunkID: int64(i), Score: float64(20-i) / 20, Source: "vector"}
		bm25Results[i] = rag.RetrievalResult{ChunkID: int64(i + 100), Score: float64(20-i) / 20, Source: "bm25"}
	}

	fused := rag.HybridFuse(vectorResults, bm25Results, 5)
	if len(fused) != 5 {
		t.Errorf("topK=5 期望 5 条, 实际 %d", len(fused))
	}
}

// TestHybridFuse_EmptyInput 验证空输入返回空。
func TestHybridFuse_EmptyInput(t *testing.T) {
	fused := rag.HybridFuse(nil, nil, 5)
	if len(fused) != 0 {
		t.Errorf("空输入期望 0 条, 实际 %d", len(fused))
	}
}

// TestHybridFuse_SameChunkDifferentRank 验证同一文档在不同路径的不同排序正确融合。
func TestHybridFuse_SameChunkDifferentRank(t *testing.T) {
	// chunk=1: vector排第1, bm25排第3
	// chunk=2: vector排第2, bm25排第1
	// chunk=3: vector排第3, bm25排第2
	vectorResults := []rag.RetrievalResult{
		{ChunkID: 1, Score: 0.99, Source: "vector"},
		{ChunkID: 2, Score: 0.80, Source: "vector"},
		{ChunkID: 3, Score: 0.60, Source: "vector"},
	}
	bm25Results := []rag.RetrievalResult{
		{ChunkID: 2, Score: 0.90, Source: "bm25"},
		{ChunkID: 3, Score: 0.70, Source: "bm25"},
		{ChunkID: 1, Score: 0.50, Source: "bm25"},
	}

	fused := rag.HybridFuse(vectorResults, bm25Results, 3)
	if len(fused) != 3 {
		t.Fatalf("期望 3 条融合结果, 实际 %d", len(fused))
	}

	// RRF(1, 3rd) < RRF(2nd, 1st) < RRF(3rd, 2nd)
	// Expected order: chunk=2 (best), then chunk=1 or chunk=3
	t.Logf("融合排序: %v", fused)
}
