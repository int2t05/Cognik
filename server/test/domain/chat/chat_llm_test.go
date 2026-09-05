//go:build integration

// Package service_test 验证与真实 LLM/Embedding 服务的端到端集成。
//
// 需要运行中的服务：llama.cpp (LLM :8081, Embedding :8082)。
package chat_test

import (
	"context"
	"testing"
	"time"

	"cognos/internal/agent/llm"
)

// TestLLMClient_ChatCompletion 验证 Eino ChatModel 可正常调用 llama.cpp。
// 用 Eino ChatModel.Generate 调用 LLM。
//
// 需要运行中的 LLM 服务，不可用时跳过。
func TestLLMClient_ChatCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(bgCtx, 5*time.Second)
	defer cancel()

	// 临时构造 ChatModel 探活
	chatModel := llm.NewChatModel(llm.ChatModelConfig{
		APIKey:  "",
		Model:   "qwen3-4b",
		BaseURL: "http://localhost:8081/v1",
	})

	resp, err := chatModel.Generate(ctx, []*llm.Message{
		{Role: "user", Content: "你好"},
	})
	if err != nil {
		t.Skipf("LLM 调用失败（%v），跳过集成测试", err)
		return
	}
	if resp.Content == "" {
		t.Error("期望非空响应内容")
	}
	t.Logf("LLM 响应: %s", resp.Content)
}
