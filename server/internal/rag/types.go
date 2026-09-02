// Package rag 实现自建 RAG 检索引擎：分块、BM25、向量、RRF 融合、重排序、文档异步处理。
// rag 包不依赖 HTTP 层，仅依赖 adapter 接口（LLMClient/EmbeddingClient/VectorStore）。
package rag

import (
	"context"
)

// =============================================================================
// Retriever 接口
// =============================================================================

// Retriever 定义检索引擎接口，Pipeline 通过此接口统一调度多路检索。
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

// FusionStrategy 混合融合策略接口（默认 RRF k=60）。
type FusionStrategy interface {
	Fuse(vectorResults, bm25Results []RetrievalResult, topK int) []RetrievalResult
}

// =============================================================================
// RAGOptions — 检索参数
// =============================================================================

// RAGOptions 控制 RAG 管道各步骤的开关和参数。
// bool 字段零值 false 无法区分"未传"与"显式关"，调用方必须以 DefaultRAGOptions() 为起点再覆盖。
type RAGOptions struct {
	TopK             int                 `json:"top_k"`             // 返回结果数，默认 5（零值由 Normalize 填充）
	QueryRewrite     bool                `json:"query_rewrite"`     // 查询改写开关（调用方须显式设置）
	MultiRoute       bool                `json:"multi_route"`       // 多路检索开关
	Hybrid           bool                `json:"hybrid"`            // BM25 混合检索开关
	Rerank           bool                `json:"rerank"`            // 重排序开关
	DisableRetrieval bool                `json:"disable_retrieval"` // 禁用检索（纯 LLM 对话模式）
	RouteCount       int                 `json:"route_count"`       // 子查询数，默认 3（零值由 Normalize 填充）
	RerankCount      int                 `json:"rerank_count"`      // 重排序候选数，默认 topK*3（零值由 Normalize 填充）
	History          []map[string]string `json:"-"`                 // 对话历史（不入 JSON），用于查询改写上下文消歧
}

// DefaultRAGOptions 返回默认的 RAG 检索选项。
func DefaultRAGOptions() RAGOptions {
	return RAGOptions{
		TopK:         5,
		QueryRewrite: true,
		MultiRoute:   true,
		Hybrid:       true,
		Rerank:       true,
		RouteCount:   3,
		RerankCount:  15,
	}
}

// Normalize 将零值字段填充为默认值，确保管道行为一致：
//   - TopK <= 0 → 5
//   - RouteCount <= 0 → 3
//   - RerankCount <= 0 或 < TopK → TopK * 3（重排序候选池不小于目标返回数）
func (opts *RAGOptions) Normalize() {
	if opts.TopK <= 0 {
		opts.TopK = 5
	}
	if opts.RouteCount <= 0 {
		opts.RouteCount = 3
	}
	if opts.RerankCount <= 0 {
		opts.RerankCount = opts.TopK * 3
	}
	// 确保重排序候选池不小于目标返回数
	if opts.RerankCount < opts.TopK {
		opts.RerankCount = opts.TopK * 3
	}
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
	ConfRaw        float64 `json:"conf_raw,omitempty"` // 综合置信度 [0,1]（由 computeConfidenceScores 计算）
	Source         string  `json:"source"`             // 检索来源："vector" | "bm25" | "hybrid"
	ChunkIndex     int     `json:"chunk_index"`        // 分块序号
}

// ChunkDisplay chunks SSE 事件中单条 chunk 的展示信息（不含内容文本，仅标识与分数）。
type ChunkDisplay struct {
	ID     int64   `json:"id"`
	Score  float64 `json:"score"`  // 综合置信度 ConfRaw [0,1]（S_qa + BM25 + Rerank 分层组合）
	Source string  `json:"source"` // 来源文档名称
}

// =============================================================================
// 管道指标类型
// =============================================================================

// RAGResult RAG 管道执行最终结果。
type RAGResult struct {
	Chunks            []RetrievalResult `json:"chunks"`  // 检索到的分块列表（按分数降序）
	ChunkDisplays     []ChunkDisplay    `json:"-"`       // 前端展示用的 chunk 分数（批次归一化）
	QuestionEmbedding []float32         `json:"-"`       // 用户问题的 embedding 向量，供 S_qa 复用
	Metrics           PipelineMetrics   `json:"metrics"` // 管道各步骤耗时与状态
}

// PipelineMetrics 管道各步骤的执行指标。
type PipelineMetrics struct {
	Steps           []StepMetric `json:"steps"`             // 各步骤指标（按执行顺序）
	TotalDurationMS int64        `json:"total_duration_ms"` // 管道总耗时（毫秒）
}

// StepMetric 单个管道步骤的执行指标。
type StepMetric struct {
	StepID     string `json:"step_id"`     // 步骤标识：query_rewrite / multi_route / vector_retrieve / bm25_retrieve / hybrid_fuse / rerank / llm_generate
	Label      string `json:"label"`       // 步骤显示名称（中文）
	DurationMS int64  `json:"duration_ms"` // 步骤耗时（毫秒）
	Success    bool   `json:"success"`     // 是否成功
	Error      string `json:"error"`       // 失败时的错误信息
}

// =============================================================================
// SSE 步骤事件
// =============================================================================

// StepEvent SSE 流式响应中的管道步骤进度事件。
type StepEvent struct {
	Type  string `json:"type"`  // "step"
	ID    string `json:"id"`    // 步骤标识
	Label string `json:"label"` // 步骤显示名称
}

// =============================================================================
// 回调类型
// =============================================================================

// StepCallback 管道步骤回调函数。
// Pipeline 在进入每个步骤时调用此回调，用于向前端发送 SSE 步骤事件。
type StepCallback func(event StepEvent)
