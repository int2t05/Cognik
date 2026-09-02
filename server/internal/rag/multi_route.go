// Package rag 实现自建 RAG 检索引擎。
//
// multi_route.go 实现多路检索，LLM 生成互补子查询提升查全率。失败时降级为单路检索。
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"opsmind/internal/infra/adapter"
)

// MultiRoute 使用 LLM 从不同角度生成多个子查询，count 自动钳位到 [2,4]。
// LLM 失败或 llm 为 nil 时降级返回 [query]。
func MultiRoute(ctx context.Context, llm adapter.LLMClient, model, query string, count int) ([]string, error) {
	if llm == nil {
		return []string{query}, nil
	}

	// 钳位到 [2, 4]：<2 没必要多路，>4 检索放大过多
	if count < 2 {
		count = 2
	} else if count > 4 {
		count = 4
	}

	systemMsg := fmt.Sprintf(
		"你是运维场景的查询扩展助手。将用户问题从不同角度拆解为 %d 个互补子查询。\n\n规则：\n1. 每个子查询覆盖不同运维维度（操作步骤、错误排查、配置方法、权限问题）\n2. 子查询之间互补而非重复\n3. 只输出 JSON 字符串数组，例：[\"查询1\",\"查询2\"]",
		count,
	)
	userMsg := fmt.Sprintf("原始查询：%s", query)

	resp, err := llm.ChatCompletion(ctx, adapter.ChatRequest{
		Model: model,
		Messages: []adapter.ChatMessage{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   512,
		Temperature: 0.3,
	})
	if err != nil {
		slog.Warn("多路检索 LLM 调用失败，降级为单路", "model", model, "query", query, "error", err)
		return []string{query}, fmt.Errorf("多路检索 LLM 调用失败，降级为单路: %w", err)
	}

	// 从 LLM 响应中提取 JSON 数组
	routes := parseMultiRouteJSON(resp.Content, query, count)
	if len(routes) == 0 {
		slog.Info("多路检索解析为空，降级为单路", "query", query, "raw_response", resp.Content)
		return []string{query}, nil
	}
	slog.Info("多路检索完成", "原始", query, "子查询数", len(routes), "子查询", routes, "model", model)
	return routes, nil
}

// parseMultiRouteJSON 从 LLM 响应中解析 JSON 字符串数组，先整段解析失败后截取首个 [...] 再解析。
func parseMultiRouteJSON(raw string, originalQuery string, maxCount int) []string {
	raw = strings.TrimSpace(raw)

	// 尝试 1：整段解析
	if routes := tryParseJSONArray(raw, originalQuery, maxCount); routes != nil {
		return routes
	}

	// 尝试 2：截取首个 [...] 再解析
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		if routes := tryParseJSONArray(raw[start:end+1], originalQuery, maxCount); routes != nil {
			return routes
		}
	}

	return nil
}

// tryParseJSONArray 尝试将字符串解析为 JSON 字符串数组，去重并截断。
func tryParseJSONArray(s string, originalQuery string, maxCount int) []string {
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}

	var routes []string
	seen := make(map[string]bool)
	for _, r := range arr {
		r = strings.TrimSpace(r)
		if r == "" || r == originalQuery || seen[r] {
			continue
		}
		seen[r] = true
		routes = append(routes, r)
		if len(routes) >= maxCount {
			break
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}
