// Package rag 实现自建 RAG 检索引擎。
//
// packing.go：Context Packing——token 预算内贪心填充检索结果。
// 从高分到低分依次放入，剩余 token 用截断的下一个 chunk 填充。
// 参考 typegraph.ai Context Window Assembly Strategies。
package rag

// PackContext 在 token 预算内贪心填充检索结果。
// 从高分（已排序）到低分依次放入，剩余 token 用截断的下一个 chunk 填充。
// tokenBudget ≤ 0 时不截断（返回原列表）。
func PackContext(results []RetrievalResult, tokenBudget int) []RetrievalResult {
	if tokenBudget <= 0 || len(results) == 0 {
		return results
	}
	var packed []RetrievalResult
	used := 0
	for _, r := range results {
		tokens := estimateTokensForString(r.Content)
		if used+tokens > tokenBudget && len(packed) > 0 {
			// 尝试截断下一个 chunk 填充剩余空间
			remaining := tokenBudget - used
			if remaining > 100 {
				r.Content = truncateString(r.Content, remaining)
				packed = append(packed, r)
			}
			break
		}
		packed = append(packed, r)
		used += tokens
	}
	return packed
}

// estimateTokensForString 估算字符串的 token 数（rune 近似）。
func estimateTokensForString(s string) int {
	runes := 0
	for _, r := range s {
		runes++
		if r < 128 {
			runes--
		}
	}
	asciiChars := len([]byte(s)) - runes
	return runes + asciiChars/4
}

// truncateString 截断字符串到 maxLen 字符。
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
