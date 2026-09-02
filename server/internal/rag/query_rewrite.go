// Package rag 实现自建 RAG 检索引擎。
//
// query_rewrite.go 实现查询改写，LLM 将口语化查询转为正式检索查询。LLM 失败时降级返回原查询。
package rag

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"opsmind/internal/infra/adapter"
)

// stripThinkingPrefix 移除模型思考/推理前缀，提取实际改写结果。
// 适配层已禁用思考模式，此函数为 API 不支持 chat_template_kwargs 时的兜底清理。
func stripThinkingPrefix(s string) string {
	// 短输出且不含思考标记：直接当作改写结果
	if len(s) <= 80 && !strings.Contains(s, "首先") && !strings.Contains(s, "好的") {
		return s
	}

	// 按句号/换行分割，从末尾反向找第一个非思考片段
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '。'
	})
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if strings.Contains(lower, "首先") || strings.Contains(lower, "接下来") ||
			strings.Contains(lower, "需要考虑") || strings.Contains(lower, "好的") ||
			strings.Contains(lower, "原始查询") || strings.Contains(lower, "用户问的是") {
			continue
		}
		if len(part) > 2 {
			return part
		}
	}

	// 兜底：返回原始字符串
	return s
}

// QueryRewrite 使用 LLM 改写查询为更适合检索的形式。history 为最近 N 轮对话用于消歧。
// LLM 失败或 llm 为 nil 时降级返回原始 query。
func QueryRewrite(ctx context.Context, llm adapter.LLMClient, model, query string, history []map[string]string) (string, error) {
	if llm == nil {
		return query, nil
	}

	// 思考模式已在适配层禁用，模型直接输出改写结果。
	systemMsg := "你是运维场景的查询改写助手。将用户口语化问题改写为正式、精确的检索查询。\n\n规则：\n1. 将口语转为书面用语（如「怎么搞」→「如何配置」）\n2. 补充运维术语（如「连不上」→「网络连接失败」）\n3. 若对话历史中有指代（「那个」「它」），替换为具体名词\n4. 只输出改写后的一句话，不要解释"
	userMsg := fmt.Sprintf("原始查询：%s\n\n请直接输出改写后的查询语句，不要输出任何解释、分析或思考过程。", query)

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
		MaxTokens:   128, // 改写结果通常 20-50 字，128 token 足够且限制思考输出
		Temperature: 0.1, // 低温度保证输出稳定
	})
	if err != nil {
		// 降级：返回原始查询，但上报错误让管道步骤显示失败状态
		slog.Warn("查询改写 LLM 调用失败，降级为原始查询", "model", model, "query", query, "error", err)
		return query, fmt.Errorf("查询改写 LLM 调用失败: %w", err)
	}
	result := strings.TrimSpace(resp.Content)
	if result == "" {
		slog.Info("查询改写返回空结果，使用原始查询", "query", query)
		return query, nil
	}

	// 后处理：移除模型思考内容残留
	result = stripThinkingPrefix(result)

	// 安全网：改写结果异常长（超原始查询 3 倍且 >60 字符）则回退原始查询
	if len(result) > 60 && len(result) > len(query)*3 {
		slog.Warn("查询改写结果异常长，疑似思考内容残留，回退原始查询",
			"改写长度", len(result), "原始长度", len(query))
		result = query
	}

	slog.Info("查询改写完成", "原始", query, "改写", result, "model", model)
	return result, nil
}
