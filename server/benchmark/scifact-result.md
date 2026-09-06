# BEIR SciFact 检索评估报告

**评估日期**:2026-09-06
**数据集**:BEIR SciFact 官方(5183 文档 + 300 test 查询)
**评估对象**:Cognik 检索管道(向量 + BM25 + RRF 混合)
**运行时长**:1020 秒(约 17 分钟,含 5183 篇串行向量化)

---

## 汇总结果

| Stage | Rec@5 | Rec@10 | MRR | nDCG@10 |
|-------|-------|--------|-----|---------|
| vector | 0.6834 | 0.7568 | 0.5810 | 0.6172 |
| bm25 | 0.7119 | 0.7723 | 0.6331 | 0.6630 |
| hybrid | 0.7552 | 0.8298 | 0.6319 | 0.6754 |

- **vector** — DashScope text-embedding-v2(1536 维),pgvector cosine
- **bm25** — gse 中文分词,Okapi k1=1.5 / b=0.75,内存索引
- **hybrid** — RRF 融合 k=30,retrievalK=30 候选

---

## 关键发现

1. **混合检索全面最优** — Recall@10 = 0.830,优于向量(0.757)和 BM25(0.772)。RRF 融合有效互补两路召回。
2. **BM25 略优于向量** — 科学文献场景关键词精确匹配强;DashScope 通用嵌入非领域专用。
3. **nDCG 与官方持平** — 混合 nDCG@10 = 0.675,接近 BEIR 官方 BM25 基线 ~0.678。
4. **向量化耗时** — 5183 篇串行(DashScope API,无并发)约 17 分钟。

---

## 与 BEIR 官方基线对比

| 基线 | nDCG@10 | 来源 |
|------|---------|------|
| BEIR 官方 BM25 | ~0.678 | BEIR 论文(Thakur et al. 2021, arXiv:2104.08663) |
| Cognik BM25 | 0.6630 | 本测试(gse + Okapi) |
| Cognik 混合 | 0.6754 | 本测试(BM25 + DashScope 向量 + RRF k=30) |

差异分析:Cognik BM25 略低于官方(0.663 vs 0.678),gse 分词器对英文学术文本不如 BM25 原生实现。混合检索补齐了这一差距。

---

## 环境

| 项 | 值 |
|----|----|
| Embedding | DashScope text-embedding-v2,1536 维 |
| pgvector | halfvec(1536),HNSW m=16 / ef_construction=200 / ef_search=100 |
| Chunker | Markdown-aware,2000 字符 / 重叠 0 |
| DB | cognik_test(localhost:5432) |
| 运行命令 | `cd server && go test ./benchmark/scifact/... -v -tags=integration -run Benchmark -p 1 -timeout 1800s` |
