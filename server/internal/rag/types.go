// Package rag 实现自建 RAG 检索引擎：分块、BM25、向量、RRF 融合、重排序、文档异步处理。
// rag 包不依赖 HTTP 层，仅依赖 adapter 接口（EmbeddingClient/VectorStore）。
package rag

import (
	"context"
)

// =============================================================================
// Retriever 接口
// =============================================================================

// Retriever 定义检索引擎接口，向量检索器与 BM25 检索器均实现此接口。
type Retriever interface {
	// Retrieve 执行检索，返回 topK 个最相关的结果。
	Retrieve(ctx context.Context, query string, kbID int64, topK int) ([]RetrievalResult, error)
}

// =============================================================================
// 内部模块接口
// =============================================================================

// TextChunker 文本分块接口。
type TextChunker interface {
	Split(text string) []string
}

// TextEmbedder 文本向量化接口。
type TextEmbedder interface {
	Embed(ctx context.Context, texts []string, model string) ([][]float32, int, error)
}

// =============================================================================
// 检索结果类型
// =============================================================================

// RetrievalResult 单条检索命中结果。
type RetrievalResult struct {
	ChunkID        int64   `json:"chunk_id"`           // knowledge_chunks.id
	ArticleID      int64   `json:"article_id"`         // knowledge_articles.id
	Content        string  `json:"content"`            // 分块文本内容
	Score          float64 `json:"score"`              // 相关度分数（RRF 融合后可 >1，BM25 无上界）
	RawCosineScore float64 `json:"-"`                  // 向量检索原始余弦相似度 [0,1]，S_qa 置信度基座（BM25-only chunk 为 0）
	Bm25NormScore  float64 `json:"-"`                  // 归一化 BM25 分数 [0,1]（仅混合检索时有值，否则 0）
	RerankScore    float64 `json:"-"`                  // Cross-encoder 相关性分数 [0,1]（仅重排序时有值，否则 0）
	ConfRaw        float64 `json:"conf_raw,omitempty"` // 综合置信度 [0,1]（由 KBStore.Search 计算）
	Source         string  `json:"source"`             // 检索来源："vector" | "bm25" | "hybrid"
	ChunkIndex     int     `json:"chunk_index"`        // 分块序号
}
