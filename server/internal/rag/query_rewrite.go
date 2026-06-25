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
	"log/slog"
	"strings"

	"opsmind/internal/adapter"
)

// stripThinkingPrefix 移除模型思考/推理前缀，提取实际改写结果。
//
// Qwen3 等模型的思考模式会在输出中夹带「好的」「首先」「我需要」等推理前缀。
// 这些内容对检索无意义且会污染后续管道步骤。
//
// 策略：
//  1. 移除常见的思考前缀模式（"好的，用户..."、"首先，我需要..."等）
//  2. 截取首个有效改写句（通常是被改写后的检索查询）
func stripThinkingPrefix(s string) string {
	// 策略 1：如果输出是单行且不含思考标记，直接返回
	if len(s) <= 100 && !strings.Contains(s, "首先") && !strings.Contains(s, "好的") {
		return s
	}

	// 策略 2：按句号/换行分割，取最后一个看起来像"查询"的片段
	// 思考链通常以「好的，用户问的是...」开头，以改写后的查询结束
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '。'
	})
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		// 跳过明显是思考的片段
		lower := strings.ToLower(part)
		if strings.Contains(lower, "首先") || strings.Contains(lower, "接下来") ||
			strings.Contains(lower, "需要考虑") || strings.Contains(lower, "好的") ||
			strings.Contains(lower, "原始查询") || strings.Contains(lower, "用户问的是") {
			continue
		}
		// 找到第一个非思考片段，作为改写结果
		if len(part) > 2 {
			return part
		}
	}

	// 策略 3：无法识别 → 返回原始字符串（后续步骤可降级处理）
	return s
}

// QueryRewrite 使用 LLM 改写查询为更适合检索的形式。
//
// history 为最近 N 轮对话（每轮含 role/content），用于上下文消歧。
// LLM 调用失败或 llm 为 nil 时降级返回原始 query。
func QueryRewrite(ctx context.Context, llm adapter.LLMClient, model, query string, history []map[string]string) (string, error) {
	if llm == nil {
		return query, nil
	}

	// 构造 prompt
	//
	// 为什么用 few-shot 而非单行指令：
	// Qwen3 等模型内建思考模式，简单的「不要解释」指令不足够阻止其输出
	// 推理链。通过给出输入→输出示例，让模型直接模仿输出格式。
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

	// 后处理：移除模型的思考/推理内容
	// Qwen3 等模型的思考模式会在输出中夹带「好的」「首先」「我需要」等前缀，
	// 这些前缀之后的检索关键词才能用于 pipeline 后续步骤。
	// 策略：如果输出明显是思考链（含多个句号/换行），尝试提取最后一句作为改写结果。
	result = stripThinkingPrefix(result)

	slog.Info("查询改写完成", "原始", query, "改写", result, "model", model)
	return result, nil
}
