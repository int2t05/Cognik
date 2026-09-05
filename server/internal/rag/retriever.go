// Package rag 实现自建 RAG 检索引擎。
//
// retriever.go 实现向量检索器（Embedder + VectorStore → Retriever 接口）。
package rag

import (
	"context"
	"fmt"

	"cognik/internal/infra/adapter"
)

// VectorRetriever 向量检索器，将查询向量化后调用 pgvector cosine 检索。
type VectorRetriever struct {
	embedder *Embedder
	store    adapter.VectorStore
}

// NewVectorRetriever 创建向量检索器实例。store 为 nil 时降级返回空结果。
func NewVectorRetriever(embedder *Embedder, store adapter.VectorStore) *VectorRetriever {
	return &VectorRetriever{embedder: embedder, store: store}
}

// Retrieve 执行向量检索。r 或 store 为 nil 时降级返回空结果，不阻塞管道。
func (r *VectorRetriever) Retrieve(ctx context.Context, query string, kbID int64, topK int) ([]RetrievalResult, error) {
	return r.RetrieveFiltered(ctx, query, kbID, topK, MetaFilter{})
}

// RetrieveFiltered 带 metadata 硬过滤的向量检索（articleType + tags 下推到 pgvector WHERE）。
func (r *VectorRetriever) RetrieveFiltered(ctx context.Context, query string, kbID int64, topK int, filter MetaFilter) ([]RetrievalResult, error) {
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

	results, err := r.store.CosineSearchFiltered(ctx, kbID, vectors[0], topK, filter.ArticleType, filter.Tags)
	if err != nil {
		return nil, fmt.Errorf("pgvector 检索失败: %w", err)
	}

	retrievalResults := make([]RetrievalResult, len(results))
	for i, rr := range results {
		retrievalResults[i] = RetrievalResult{
			ChunkID:        rr.ChunkID,
			ArticleID:      rr.ArticleID,
			Content:        rr.Content,
			Score:          rr.Score,
			RawCosineScore: rr.Score, // 向量检索的 Score 即 1 - cosine_distance ∈ [0,1]
			Source:         "vector",
			ChunkIndex:     rr.ChunkIndex,
		}
	}
	return retrievalResults, nil
}
