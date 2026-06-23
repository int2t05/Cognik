package rag_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"opsmind/internal/adapter"
	"opsmind/internal/rag"
)

// =============================================================================
// mockReranker 模拟 cross-encoder 重排序器
// =============================================================================

type mockReranker struct {
	order  []int
	scores []float64
	err    error
}

func (m *mockReranker) Rerank(ctx context.Context, query string, passages []adapter.RerankPassage) (*adapter.RerankResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &adapter.RerankResult{Order: m.order, Scores: m.scores}, nil
}

// =============================================================================
// 查询改写测试
// =============================================================================

// TestQueryRewrite_Success 验证查询改写基本功能。
func TestQueryRewrite_Success(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{
			Content: "如何配置 VPN 客户端连接",
			FinishReason: "stop",
		},
	}

	rewritten, err := rag.QueryRewrite(context.Background(), llm, "VPN怎么连", nil)
	if err != nil {
		t.Fatalf("QueryRewrite 失败: %v", err)
	}
	if rewritten == "" {
		t.Error("改写结果不应为空")
	}
	if !strings.Contains(rewritten, "VPN") {
		t.Errorf("改写结果应包含关键词 'VPN': %s", rewritten)
	}
}

// TestQueryRewrite_LLMFail 验证 LLM 调用失败时降级返回原始查询。
func TestQueryRewrite_LLMFail(t *testing.T) {
	llm := &mockLLMClient{
		chatError: fmt.Errorf("LLM 服务不可用"),
	}

	original := "账号冻结如何处理"
	rewritten, err := rag.QueryRewrite(context.Background(), llm, original, nil)

	// LLM 失败时：返回原始查询 + error。Pipeline 层通过 track() 记录失败但不阻塞。
	if rewritten != original {
		t.Fatalf("LLM 失败应返回原始查询, 期望 %q, 实际 %q", original, rewritten)
	}
	if err == nil {
		t.Error("LLM 失败应返回 error 以标记管道步骤失败状态")
	}
}

// TestQueryRewrite_WithHistory 验证带历史对话的查询改写。
func TestQueryRewrite_WithHistory(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{
			Content: "如何重置已过期的 VPN 密码",
			FinishReason: "stop",
		},
	}

	history := []map[string]string{
		{"role": "user", "content": "VPN密码忘了"},
		{"role": "assistant", "content": "您可以联系管理员重置密码。"},
	}

	rewritten, err := rag.QueryRewrite(context.Background(), llm, "过期了怎么办", history)
	if err != nil {
		t.Fatalf("带历史对话的 QueryRewrite 失败: %v", err)
	}
	if rewritten == "" {
		t.Error("改写结果不应为空")
	}
}

// =============================================================================
// 多路检索测试
// =============================================================================

// TestMultiRoute_Success 验证多路检索生成子查询。
func TestMultiRoute_Success(t *testing.T) {
	llm := &mockLLMClient{
		chatResponse: &adapter.ChatResponse{
			Content: `["VPN连接故障排查", "VPN客户端配置", "VPN证书问题"]`,
			FinishReason: "stop",
		},
	}

	routes, err := rag.MultiRoute(context.Background(), llm, "VPN连接问题", 3)
	if err != nil {
		t.Fatalf("MultiRoute 失败: %v", err)
	}
	if len(routes) < 2 {
		t.Errorf("多路检索期望 ≥ 2 个子查询, 实际 %d", len(routes))
	}
}

// TestMultiRoute_LLMFail 验证 LLM 失败时降级返回原查询。
func TestMultiRoute_LLMFail(t *testing.T) {
	llm := &mockLLMClient{
		chatError: fmt.Errorf("LLM 服务不可用"),
	}

	original := "邮箱无法登录"
	routes, err := rag.MultiRoute(context.Background(), llm, original, 3)

	if err != nil {
		t.Fatalf("LLM 失败不应报错（应降级）: %v", err)
	}
	if len(routes) != 1 || routes[0] != original {
		t.Errorf("LLM 失败应返回 [原始查询], 实际 %v", routes)
	}
}

// =============================================================================
// 重排序测试
// =============================================================================

// TestRerank_Success 验证 cross-encoder 重排序基本功能。
func TestRerank_Success(t *testing.T) {
	reranker := &mockReranker{
		order:  []int{2, 0, 1}, // 按相关度：chunk 3 → 1 → 2
		scores: []float64{0.95, 0.72, 0.31},
	}

	candidates := []rag.RetrievalResult{
		{ChunkID: 1, Content: "VPN客户端下载安装教程", Score: 0.80},
		{ChunkID: 2, Content: "打印机驱动安装指南", Score: 0.60},
		{ChunkID: 3, Content: "VPN连接失败常见原因及解决方案", Score: 0.75},
	}

	query := "VPN无法连接"
	reranked, err := rag.Rerank(context.Background(), reranker, query, candidates)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}
	if len(reranked) != 3 {
		t.Errorf("重排序后应保持 3 条结果, 实际 %d", len(reranked))
	}
	// cross-encoder 重排后 chunk 3（最相关）应排第一
	if reranked[0].ChunkID != 3 {
		t.Errorf("cross-encoder 重排后应把最相关 chunk 排第一, 实际 chunk_id=%d", reranked[0].ChunkID)
	}
}

// TestRerank_NilReranker 验证 reranker 为 nil 时降级返回原排序。
func TestRerank_NilReranker(t *testing.T) {
	candidates := []rag.RetrievalResult{
		{ChunkID: 10, Content: "最重要内容", Score: 0.90},
		{ChunkID: 20, Content: "次重要内容", Score: 0.70},
	}

	// reranker=nil 模拟 cross-encoder 未启用
	reranked, err := rag.Rerank(context.Background(), nil, "查询", candidates)
	if err != nil {
		t.Fatalf("reranker 为 nil 不应报错（应降级）: %v", err)
	}
	if len(reranked) != 2 {
		t.Errorf("降级应保留原始结果数, 期望 2, 实际 %d", len(reranked))
	}
	// 原始排序保持
	if reranked[0].ChunkID != 10 {
		t.Errorf("降级应保持原始排序")
	}
}
