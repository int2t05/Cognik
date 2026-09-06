// Package scifact 提供检索质量评估指标计算。
package scifact

import "math"

// RecallAtK 计算 top-k 中相关文档的覆盖率。
// retrieved 为按检索排名排序的文档 ID 列表;relevant 为相关文档 ID 集合。
func RecallAtK(retrieved []int64, relevant map[int64]bool, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	if k > len(retrieved) {
		k = len(retrieved)
	}
	hits := 0
	for i := 0; i < k; i++ {
		if relevant[retrieved[i]] {
			hits++
		}
	}
	return float64(hits) / float64(len(relevant))
}

// ReciprocalRank 返回首个相关文档的倒数排名(1/rank),无相关则 0。
func ReciprocalRank(retrieved []int64, relevant map[int64]bool) float64 {
	for i, id := range retrieved {
		if relevant[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// NDCGAtK 计算二值相关性的归一化折损累积增益(nDCG@k)。
// DCG = Σ rel_i / log2(i+2),IDCG 为理想排序下的 DCG,nDCG = DCG/IDCG。
func NDCGAtK(retrieved []int64, relevant map[int64]bool, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	if k > len(retrieved) {
		k = len(retrieved)
	}
	dcg := 0.0
	for i := 0; i < k; i++ {
		if relevant[retrieved[i]] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	nRel := len(relevant)
	if nRel > k {
		nRel = k
	}
	idcg := 0.0
	for i := 0; i < nRel; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// ResultArticleIDs 从 ArticleID 列表提取去重后的 ID(保留首次出现的排名)。
// 调用方先从 RetrievalResult 列表提取 ArticleID 再传入。
func ResultArticleIDs(articleIDs []int64) []int64 {
	ids := make([]int64, 0, len(articleIDs))
	seen := make(map[int64]bool)
	for _, id := range articleIDs {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}
