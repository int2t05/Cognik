// Package rag_test 验证 RAG 检索优化组件（Sandwich Reorder + Context Packing）。
package rag_test

import (
	"testing"

	"cognos/internal/rag"
)

// --- Sandwich Reorder ---

func TestSandwichReorder_ShortList(t *testing.T) {
	results := []rag.RetrievalResult{
		{ChunkID: 1, Score: 0.9},
		{ChunkID: 2, Score: 0.8},
	}
	reordered := rag.SandwichReorder(results)
	if len(reordered) != 2 {
		t.Errorf("2 条结果应保持 2 条，得到 %d", len(reordered))
	}
}

func TestSandwichReorder_HighAtEdges(t *testing.T) {
	results := []rag.RetrievalResult{
		{ChunkID: 1, Score: 0.9},
		{ChunkID: 2, Score: 0.8},
		{ChunkID: 3, Score: 0.7},
		{ChunkID: 4, Score: 0.6},
		{ChunkID: 5, Score: 0.5},
	}
	reordered := rag.SandwichReorder(results)
	// rank 0（最高分）应在首部
	if reordered[0].ChunkID != 1 {
		t.Errorf("首部应为最高分 chunk 1，得到 %d", reordered[0].ChunkID)
	}
	// rank 1（次高分）应在尾部
	if reordered[len(reordered)-1].ChunkID != 2 {
		t.Errorf("尾部应为次高分 chunk 2，得到 %d", reordered[len(reordered)-1].ChunkID)
	}
}

func TestSandwichReorder_Empty(t *testing.T) {
	reordered := rag.SandwichReorder(nil)
	if reordered != nil {
		t.Errorf("空输入应返回 nil，得到 %d 条", len(reordered))
	}
}

// --- Context Packing ---

func TestPackContext_NoBudget(t *testing.T) {
	results := []rag.RetrievalResult{
		{Content: "chunk1", Score: 0.9},
		{Content: "chunk2", Score: 0.8},
	}
	packed := rag.PackContext(results, 0)
	if len(packed) != 2 {
		t.Errorf("budget=0 不应截断，得到 %d 条", len(packed))
	}
}

func TestPackContext_Truncate(t *testing.T) {
	// 每条 chunk ~1 token（6 ASCII char → 6/4 ≈ 1 token）
	results := []rag.RetrievalResult{
		{Content: "chunk1", Score: 0.9},
		{Content: "chunk2", Score: 0.8},
		{Content: "chunk3", Score: 0.7},
	}
	packed := rag.PackContext(results, 1) // budget=1，约放 1 条
	if len(packed) > 2 {
		t.Errorf("budget=1 应截断到 ≤2 条，得到 %d", len(packed))
	}
	if len(packed) == 0 {
		t.Error("至少应返回 1 条")
	}
}

func TestPackContext_Empty(t *testing.T) {
	packed := rag.PackContext(nil, 1000)
	if packed != nil {
		t.Errorf("空输入应返回 nil，得到 %d 条", len(packed))
	}
}
