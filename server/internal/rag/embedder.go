// Package rag 实现自建 RAG 检索引擎。
//
// embedder.go 实现批量文本向量化。
//
// 为什么需要批量分页：
// Embedding API 通常有单次请求的文本数量限制（如 OpenAI 限制 2048 tokens），
// 大批量文本需要拆分为多个小批次调用。
// batchSize=20 是经验值——在减少 API 往返次数和单次请求大小之间取得平衡。
package rag

import (
	"context"
	"fmt"

	"opsmind/internal/adapter"
)

// Embedder 批量文本向量化器。
//
// 封装 EmbeddingClient，自动分批调用 + 部分失败处理。
type Embedder struct {
	client    adapter.EmbeddingClient
	batchSize int
}

// NewEmbedder 创建 Embedder 实例。
//
// client 为 OpenAI-compatible Embedding 客户端。
// batchSize 控制每批最大文本数，建议 20。
// client 为 nil 时不立即报错——Embed 调用时会返回明确错误，避免启动期装配顺序问题。
func NewEmbedder(client adapter.EmbeddingClient, batchSize int) *Embedder {
	if batchSize <= 0 {
		batchSize = 20
	}
	return &Embedder{
		client:    client,
		batchSize: batchSize,
	}
}

// SetClient 替换内部 Embedding 客户端（默认配置变更时由回调调用）。
func (e *Embedder) SetClient(client adapter.EmbeddingClient) {
	e.client = client
}

// Embed 将文本列表批量转换为向量。
//
// 返回的向量列表顺序与输入 texts 严格一致。
// 采用 fail-fast 策略：任一批次失败立即返回错误，
// 避免部分成功导致 vectors[i] 与 texts[i] 索引错位。
//
// 各批次返回的 embedding 维度必须一致——若中途模型变更导致维度不同则报错，
// 防止混维向量写入 pgvector 时报错不友好。
//
// client 为 nil 时返回错误而非 panic。
// 空输入返回空向量列表且 dimension=0。
func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}

	// client 为 nil 时返回明确错误，避免 panic
	if e.client == nil {
		return nil, 0, fmt.Errorf("embedder 未初始化: EmbeddingClient 为 nil")
	}

	var (
		allVectors [][]float32
		dimension  int
	)

	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		batchIdx := i / e.batchSize

		resp, err := e.client.CreateEmbeddings(ctx, adapter.EmbeddingRequest{
			Model: "", // 空字符串表示使用 EmbeddingClient 配置的默认模型
			Input: batch,
		})
		if err != nil {
			// fail-fast：批次失败立即返回，保留错误上下文便于调试
			return nil, 0, fmt.Errorf("第 %d 批 embedding 失败 (texts[%d:%d], 共 %d 条): %w",
				batchIdx, i, end, len(batch), err)
		}

		// 校验维度一致性：各批次必须返回相同维度
		if dimension == 0 && resp.Dimension > 0 {
			dimension = resp.Dimension
		} else if resp.Dimension > 0 && resp.Dimension != dimension {
			return nil, 0, fmt.Errorf("第 %d 批 embedding 维度不一致: 预期 %d, 实际 %d (可能中途模型变更)",
				batchIdx, dimension, resp.Dimension)
		}

		allVectors = append(allVectors, resp.Embeddings...)
	}

	return allVectors, dimension, nil
}
