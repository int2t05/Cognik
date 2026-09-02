// Package rag 实现自建 RAG 检索引擎。
//
// embedder.go 实现批量文本向量化，自动分页调用 Embedding API。
package rag

import (
	"context"
	"fmt"

	"opsmind/internal/infra/adapter"
)

// Embedder 批量文本向量化器，封装 EmbeddingClient 的自动分批与部分失败处理。
type Embedder struct {
	client    adapter.EmbeddingClient
	batchSize int
}

// NewEmbedder 创建 Embedder 实例。client 为 nil 时延迟到 Embed 调用报错。
// batchSize 控制每批最大文本数。
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

// Embed 将文本列表批量转换为向量。model 为空时用客户端默认模型。
func (e *Embedder) Embed(ctx context.Context, texts []string, model string) ([][]float32, int, error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}
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
			Model: model,
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
