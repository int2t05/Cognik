// Package rag 实现自建 RAG 检索引擎。
//
// retriever.go 实现向量检索器（Embedder + VectorStore → Retriever 接口）。
package rag

import (
	"context"
	"fmt"

	"opsmind/internal/adapter"
)

// VectorRetriever 向量检索器：将查询文本向量化后调用 pgvector cosine 检索。
//
// 实现 Retriever 接口，供 Pipeline 在 vector_retrieve 步骤中使用。
type VectorRetriever struct {
	embedder *Embedder
	store    adapter.VectorStore
}

// NewVectorRetriever 创建向量检索器实例。
//
// store 可以为 nil（pgvector 不可用时向量检索返回空结果）。
func NewVectorRetriever(embedder *Embedder, store adapter.VectorStore) *VectorRetriever {
	return &VectorRetriever{embedder: embedder, store: store}
}

// Retrieve 执行向量检索：embedding 查询 → pgvector cosine 搜索。
//
// r 为 nil（pgvector 初始化失败时 main.go 传入 nil 值）
// 或 store 为 nil 时静默降级返回空结果，不阻塞管道。
func (r *VectorRetriever) Retrieve(ctx context.Context, query string, kbID int64, topK int) ([]RetrievalResult, error) {
	if r == nil || r.store == nil {
		return nil, nil
	}
	if r.embedder == nil {
		return nil, fmt.Errorf("embedder 未初始化")
	}

	vectors, _, err := r.embedder.Embed(ctx, []string{query}, "")
	if err != nil {
		return nil, fmt.Errorf("查询向量化失败: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("查询向量化返回空结果")
	}

	results, err := r.store.CosineSearch(ctx, kbID, vectors[0], topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector 检索失败: %w", err)
	}

	retrievalResults := make([]RetrievalResult, len(results))
	for i, r := range results {
		retrievalResults[i] = RetrievalResult{
			ChunkID:        r.ChunkID,
			ArticleID:      r.ArticleID,
			Content:        r.Content,
			Score:          r.Score,
			RawCosineScore: r.Score, // 向量检索的 Score 即 1 - cosine_distance ∈ [0,1]
			Source:         "vector",
			ChunkIndex:     r.ChunkIndex,
		}
	}
	return retrievalResults, nil
}
