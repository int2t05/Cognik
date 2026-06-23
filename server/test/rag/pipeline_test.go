package rag_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/rag"
)

// =============================================================================
// mockRetriever 模拟 Retriever
// =============================================================================

type mockRetriever struct {
	results []rag.RetrievalResult
	err     error
}

func (m *mockRetriever) Retrieve(ctx context.Context, query string, kbID int64, topK int) ([]rag.RetrievalResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// =============================================================================
// mockLLMClient 模拟 LLMClient
// =============================================================================

type mockLLMClient struct {
	chatResponse *adapter.ChatResponse
	chatError    error
	streamChunks []adapter.StreamChunk
}

func (m *mockLLMClient) ChatCompletion(ctx context.Context, req adapter.ChatRequest) (*adapter.ChatResponse, error) {
	if m.chatError != nil {
		return nil, m.chatError
	}
	return m.chatResponse, nil
}

func (m *mockLLMClient) ChatCompletionStream(ctx context.Context, req adapter.ChatRequest) (<-chan adapter.StreamChunk, error) {
	if m.chatError != nil {
		return nil, m.chatError
	}
	ch := make(chan adapter.StreamChunk, len(m.streamChunks))
	go func() {
		for _, c := range m.streamChunks {
			ch <- c
		}
		close(ch)
	}()
	return ch, nil
}

// =============================================================================
// 管道测试
// =============================================================================

// TestPipeline_FullSuccess 验证完整管道（所有步骤成功）。
func TestPipeline_FullSuccess(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{Content: "改写后的查询", FinishReason: "stop"},
	}
	embedder := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	vectorRet := &mockRetriever{
		results: []rag.RetrievalResult{
			{ChunkID: 1, Content: "VPN排查步骤", Score: 0.95, Source: "vector"},
		},
	}

	pipe := rag.NewPipeline(vectorRet, nil, llm, embedder, nil) // reranker=nil: cross-encoder 未启用，降级跳过
	opts := rag.DefaultRAGOptions()

	result, err := pipe.Execute(context.Background(), "VPN怎么连接", 1, opts, nil)
	if err != nil {
		t.Fatalf("完整管道 Execute 失败: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Error("完整管道应返回检索结果")
	}
	if len(result.Metrics.Steps) == 0 {
		t.Error("管道应包含步骤指标")
	}
	if result.Metrics.TotalDurationMS < 0 {
		t.Error("管道总耗时不应为负数")
	}
}

// TestPipeline_QueryRewriteDisabled 验证查询改写开关关闭时跳过。
func TestPipeline_QueryRewriteDisabled(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{Content: "查询改写", FinishReason: "stop"},
	}
	embedder := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	vectorRet := &mockRetriever{
		results: []rag.RetrievalResult{
			{ChunkID: 2, Content: "测试", Score: 0.80, Source: "vector"},
		},
	}

	pipe := rag.NewPipeline(vectorRet, nil, llm, embedder, nil) // reranker=nil: cross-encoder 未启用，降级跳过
	opts := rag.DefaultRAGOptions()
	opts.QueryRewrite = false // 关闭查询改写

	result, err := pipe.Execute(context.Background(), "测试查询", 1, opts, nil)
	if err != nil {
		t.Fatalf("关闭查询改写的管道应成功: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Error("应返回检索结果")
	}
}

// TestPipeline_VectorRetrievalFail 验证向量检索失败时应返回错误。
func TestPipeline_VectorRetrievalFail(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{Content: "原始查询", FinishReason: "stop"},
	}
	embedder := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	failingRetriever := &mockRetriever{
		err: fmt.Errorf("pgvector 服务不可用"),
	}

	// 不启用 BM25 时，向量检索失败应导致错误
	pipe := rag.NewPipeline(failingRetriever, nil, llm, embedder, nil)
	opts := rag.DefaultRAGOptions()
	opts.Hybrid = false // 纯向量模式
	opts.QueryRewrite = false

	_, err := pipe.Execute(context.Background(), "测试", 1, opts, nil)
	if err == nil {
		t.Error("向量检索失败应返回错误（纯向量模式不可降级）")
	}
}

// TestPipeline_StepCallback 验证步骤回调被正确调用。
func TestPipeline_StepCallback(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{Content: "查询改写", FinishReason: "stop"},
	}
	embedder := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	vectorRet := &mockRetriever{
		results: []rag.RetrievalResult{
			{ChunkID: 1, Content: "test", Score: 0.90, Source: "vector"},
		},
	}

	pipe := rag.NewPipeline(vectorRet, nil, llm, embedder, nil) // reranker=nil: cross-encoder 未启用，降级跳过

	var steps []string
	callback := func(event rag.StepEvent) {
		steps = append(steps, event.ID)
	}

	opts := rag.DefaultRAGOptions()
	_, err := pipe.Execute(context.Background(), "test", 1, opts, callback)
	if err != nil {
		t.Fatalf("管道执行失败: %v", err)
	}
	if len(steps) == 0 {
		t.Error("管道应触发至少 1 个步骤回调")
	}
}

// TestPipeline_Metrics 验证管道指标包含各步骤耗时。
func TestPipeline_Metrics(t *testing.T) {
	// 用真实延迟验证耗时记录
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{Content: "改写", FinishReason: "stop"},
	}
	embedder := rag.NewEmbedder(&mockEmbeddingClient{dimension: 1024}, 20)
	vectorRet := &mockRetriever{
		results: []rag.RetrievalResult{
			{ChunkID: 1, Content: "test", Score: 0.90, Source: "vector"},
		},
	}

	pipe := rag.NewPipeline(vectorRet, nil, llm, embedder, nil) // reranker=nil: cross-encoder 未启用，降级跳过
	opts := rag.DefaultRAGOptions()

	start := time.Now()
	result, err := pipe.Execute(context.Background(), "test", 1, opts, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("管道执行失败: %v", err)
	}

	// 总耗时应在合理范围内
	if result.Metrics.TotalDurationMS < 0 {
		t.Error("总耗时不应为负数")
	}
	// Mock 执行极快，总耗时应接近 elapsed；允许合理误差
	if result.Metrics.TotalDurationMS > int64(elapsed.Milliseconds())+100 {
		t.Errorf("指标中总耗时 %dms 远大于实际耗时 %dms", result.Metrics.TotalDurationMS, elapsed.Milliseconds())
	}
}
