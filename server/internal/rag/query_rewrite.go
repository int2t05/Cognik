// Package rag 实现自建 RAG 检索引擎。
//
// query_rewrite.go 实现查询改写（Query Rewrite）。
//
// 为什么需要查询改写：
// 用户的原始查询通常口语化、不完整（如"VPN怎么连"），
// 直接检索可能命中率低。通过 LLM 将口语化查询改写为正式、
// 信息完整的检索查询，显著提升召回率。
//
// 降级策略：
// LLM 调用失败时，不阻塞管道，直接返回原始查询继续后续步骤。
package rag

import (
	"context"
	"fmt"
	"strings"

	"opsmind/internal/adapter"
)

// QueryRewrite 使用 LLM 改写查询为更适合检索的形式。
//
// history 为最近 N 轮对话（每轮含 role/content），用于上下文消歧。
// LLM 调用失败或 llm 为 nil 时降级返回原始 query。
func QueryRewrite(ctx context.Context, llm adapter.LLMClient, model, query string, history []map[string]string) (string, error) {
	if llm == nil {
		return query, nil
	}

	// 构造 prompt
	systemMsg := "你是运维场景的查询改写助手。将用户口语化问题改写为正式、精确的检索查询。\n\n规则：\n1. 将口语转为书面用语（如「怎么搞」→「如何配置」）\n2. 补充运维术语（如「连不上」→「网络连接失败」）\n3. 若对话历史中有指代（「那个」「它」），替换为具体名词\n4. 只输出改写后的一句话，不要解释"
	userMsg := fmt.Sprintf("原始查询：%s", query)

	messages := []adapter.ChatMessage{
		{Role: "system", Content: systemMsg},
	}

	// 添加历史对话（最近 3 轮）
	for _, h := range history {
		role := h["role"]
		content := h["content"]
		if role == "user" || role == "assistant" {
			messages = append(messages, adapter.ChatMessage{Role: role, Content: content})
		}
	}

	messages = append(messages, adapter.ChatMessage{Role: "user", Content: userMsg})

	resp, err := llm.ChatCompletion(ctx, adapter.ChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   256,
		Temperature: 0.1, // 低温度保证输出稳定
	})
	if err != nil {
		// 降级：返回原始查询，但上报错误让管道步骤显示失败状态
		return query, fmt.Errorf("查询改写 LLM 调用失败: %w", err)
	}
	result := strings.TrimSpace(resp.Content)
	if result == "" {
		return query, nil
	}
	return result, nil
}
