// Package rag 实现自建 RAG 检索引擎。
//
// sandwich.go：Sandwich Reorder——高分放首尾，低分放中间。
// 缓解 Lost in the Middle：LLM 对上下文窗口首尾信息处理能力优于中间。
// 参考 Dify reorder.py 奇偶拆分+反转。
package rag

// SandwichReorder 将检索结果重排：高分放首尾，低分放中间。
// 输入须已按 Score 降序排列。输出顺序：[0, 2, 4, ..., 5, 3, 1]。
func SandwichReorder(results []RetrievalResult) []RetrievalResult {
	n := len(results)
	if n <= 2 {
		return results
	}
	reordered := make([]RetrievalResult, n)
	left, right := 0, n-1
	for i, r := range results {
		if i%2 == 0 {
			reordered[left] = r
			left++
		} else {
			reordered[right] = r
			right--
		}
	}
	return reordered
}
