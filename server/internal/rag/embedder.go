// Package rag 实现自建 RAG 检索引擎。
//
// embedder.go 实现批量文本向量化，自动分页调用 Embedding API。
// 查询侧（单文本）命中 LRU+TTL 缓存，索引侧（批量）完全旁路。
package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"cognos/internal/infra/adapter"
)

// cacheEntry 查询 embedding 缓存条目。
type cacheEntry struct {
	vec      []float32
	dim      int
	expireAt time.Time
}

// queryEmbedCache 查询侧 embedding 缓存（LRU + TTL）。
// 仅缓存单文本查询结果；索引批路径完全旁路。key 为 model:text 的哈希。
type queryEmbedCache struct {
	mu        sync.RWMutex
	entries   map[string]cacheEntry
	ttl       time.Duration
	max       int
	accessCnt uint64 // 近似 LRU：淘汰最旧条目（按插入序遍历）
}

func newQueryEmbedCache(ttl time.Duration, max int) *queryEmbedCache {
	if max <= 0 {
		max = 1000
	}
	return &queryEmbedCache{entries: make(map[string]cacheEntry), ttl: ttl, max: max}
}

// get 命中返回向量（深拷贝，避免外部修改污染缓存）；未命中或过期返回 nil。
func (c *queryEmbedCache) get(key string) ([]float32, int, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expireAt) {
		return nil, 0, false
	}
	vec := make([]float32, len(e.vec))
	copy(vec, e.vec)
	return vec, e.dim, true
}

// put 写入并按上限淘汰最旧条目。
func (c *queryEmbedCache) put(key string, vec []float32, dim int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		// 淘汰过期项；若无过期项，淘汰 expireAt 最早的
		var oldestKey string
		var oldestExp time.Time
		for k, e := range c.entries {
			if time.Now().After(e.expireAt) {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || e.expireAt.Before(oldestExp) {
				oldestKey = k
				oldestExp = e.expireAt
			}
		}
		if len(c.entries) >= c.max && oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}
	stored := make([]float32, len(vec))
	copy(stored, vec)
	c.entries[key] = cacheEntry{vec: stored, dim: dim, expireAt: time.Now().Add(c.ttl)}
}

// Embedder 批量文本向量化器，封装 EmbeddingClient 的自动分批与部分失败处理。
type Embedder struct {
	client    adapter.EmbeddingClient
	batchSize int
	queryCache *queryEmbedCache // 查询侧缓存（nil 时禁用）
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

// SetQueryCache 启用查询侧 embedding 缓存（仅单文本路径生效，批量旁路）。
func (e *Embedder) SetQueryCache(ttl time.Duration, max int) {
	e.queryCache = newQueryEmbedCache(ttl, max)
}

// SetClient 替换内部 Embedding 客户端（默认配置变更时由回调调用）。
func (e *Embedder) SetClient(client adapter.EmbeddingClient) {
	e.client = client
}

// embedSingle 向量化单文本（查询路径），命中缓存则直接返回。
func (e *Embedder) embedSingle(ctx context.Context, text, model string) ([][]float32, int, error) {
	if e.queryCache != nil {
		h := sha256.Sum256([]byte(model + "\x00" + text))
		key := hex.EncodeToString(h[:])
		if vec, dim, ok := e.queryCache.get(key); ok {
			return [][]float32{vec}, dim, nil
		}
		vectors, dim, err := e.embedBatch(ctx, []string{text}, model)
		if err == nil && len(vectors) > 0 {
			e.queryCache.put(key, vectors[0], dim)
		}
		return vectors, dim, err
	}
	return e.embedBatch(ctx, []string{text}, model)
}

// Embed 将文本列表批量转换为向量。model 为空时用客户端默认模型。
// 单文本命中查询缓存；批量（索引路径）完全旁路缓存。
func (e *Embedder) Embed(ctx context.Context, texts []string, model string) ([][]float32, int, error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}
	if e.client == nil {
		return nil, 0, fmt.Errorf("embedder 未初始化: EmbeddingClient 为 nil")
	}
	// 单文本走缓存路径
	if len(texts) == 1 {
		return e.embedSingle(ctx, texts[0], model)
	}
	return e.embedBatch(ctx, texts, model)
}

// embedBatch 批量向量化（自动分页 + 维度校验），不经缓存。
func (e *Embedder) embedBatch(ctx context.Context, texts []string, model string) ([][]float32, int, error) {
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
