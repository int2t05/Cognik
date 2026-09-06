# Cognik 端到端 QA 测试报告

**日期**: 2026-09-06
**KB**: QA-OpsKB (6 文档, 18 查询)
**评估对象**: Cognik RAG 检索管道 (向量 + BM25 + RRF + CRAG)

## 问答结果

| # | 难度 | Question | Verdict | 工具调用数 | 回答长度 |
|---|------|----------|---------|-----------|---------|
| 1 | simple | How to configure shared_buffers and work |  | 8 | 3834 |
| 2 | simple | What to check when a Docker container is |  | 12 | 3609 |
| 3 | simple | How to debug a Kubernetes pod in CrashLo |  | 11 | 4248 |
| 4 | simple | Which tools to use for disk I/O performa |  | 7 | 3947 |
| 5 | simple | How to diagnose DNS resolution problems  |  | 11 | 4843 |
| 6 | simple | How to disable SSH password authenticati |  | 9 | 3937 |
| 7 | multihop | A PostgreSQL container in Kubernetes kee |  | 3 | 6928 |
| 8 | multihop | How to trace network connections of a sp |  | 9 | 4789 |
| 9 | multihop | A Kubernetes pod has high disk I/O causi |  | 12 | 4037 |
| 10 | multihop | How to harden a Docker container for pro |  | 8 | 4861 |
| 11 | multihop | PostgreSQL slow queries are suspected, h |  | 10 | 7786 |
| 12 | multihop | Kubernetes node is running out of memory |  | 12 | 4741 |
| 13 | multihop | DNS resolution is slow for Kubernetes po |  | 12 | 5619 |
| 14 | multihop | How to configure TLS certificates for a  |  | 8 | 4248 |
| 15 | paraphrased | Database performance degraded after migr |  | 10 | 6100 |
| 16 | paraphrased | Container cannot reach external API endp |  | 9 | 5741 |
| 17 | paraphrased | Pods are being evicted from the cluster  |  | 10 | 3571 |
| 18 | paraphrased | Application latency increased after the  |  | 3 | 1862 |

## Agent Trace 摘要

| # | Question | reasoning | tool_calls | tool_results | answer 长度 |
|---|----------|-----------|------------|-------------|------------|
| 1 | How to configure shared_buffer | 407 | 8 | 8 | 3834 |
| 2 | What to check when a Docker co | 174 | 12 | 12 | 3609 |
| 3 | How to debug a Kubernetes pod  | 630 | 11 | 11 | 4248 |
| 4 | Which tools to use for disk I/ | 311 | 7 | 7 | 3947 |
| 5 | How to diagnose DNS resolution | 343 | 11 | 11 | 4843 |
| 6 | How to disable SSH password au | 311 | 9 | 9 | 3937 |
| 7 | A PostgreSQL container in Kube | 444 | 3 | 3 | 6928 |
| 8 | How to trace network connectio | 402 | 9 | 9 | 4789 |
| 9 | A Kubernetes pod has high disk | 679 | 12 | 12 | 4037 |
| 10 | How to harden a Docker contain | 447 | 8 | 8 | 4861 |
| 11 | PostgreSQL slow queries are su | 298 | 10 | 10 | 7786 |
| 12 | Kubernetes node is running out | 253 | 12 | 12 | 4741 |
| 13 | DNS resolution is slow for Kub | 302 | 12 | 12 | 5619 |
| 14 | How to configure TLS certifica | 533 | 8 | 8 | 4248 |
| 15 | Database performance degraded  | 434 | 10 | 10 | 6100 |
| 16 | Container cannot reach externa | 243 | 9 | 9 | 5741 |
| 17 | Pods are being evicted from th | 303 | 10 | 10 | 3571 |
| 18 | Application latency increased  | 94 | 3 | 3 | 1862 |

## 失败分析

- Q1 (simple): How to configure shared_buffers and work_mem in Po
  - Verdict: (无)
  - 相关文档: ['postgresql-tuning.md']
  - Top source: (无)
- Q2 (simple): What to check when a Docker container is stuck in 
  - Verdict: (无)
  - 相关文档: ['docker-troubleshooting.md']
  - Top source: (无)
- Q3 (simple): How to debug a Kubernetes pod in CrashLoopBackOff 
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md']
  - Top source: (无)
- Q4 (simple): Which tools to use for disk I/O performance analys
  - Verdict: (无)
  - 相关文档: ['linux-performance.md']
  - Top source: (无)
- Q5 (simple): How to diagnose DNS resolution problems in product
  - Verdict: (无)
  - 相关文档: ['network-diagnosis.md']
  - Top source: (无)
- Q6 (simple): How to disable SSH password authentication and enf
  - Verdict: (无)
  - 相关文档: ['security-hardening.md']
  - Top source: (无)
- Q7 (multihop): A PostgreSQL container in Kubernetes keeps getting
  - Verdict: (无)
  - 相关文档: ['postgresql-tuning.md', 'docker-troubleshooting.md', 'kubernetes-debugging.md']
  - Top source: (无)
- Q8 (multihop): How to trace network connections of a specific Doc
  - Verdict: (无)
  - 相关文档: ['docker-troubleshooting.md', 'network-diagnosis.md']
  - Top source: (无)
- Q9 (multihop): A Kubernetes pod has high disk I/O causing node pr
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md', 'linux-performance.md']
  - Top source: (无)
- Q10 (multihop): How to harden a Docker container for production de
  - Verdict: (无)
  - 相关文档: ['docker-troubleshooting.md', 'security-hardening.md']
  - Top source: (无)
- Q11 (multihop): PostgreSQL slow queries are suspected, how to use 
  - Verdict: (无)
  - 相关文档: ['postgresql-tuning.md']
  - Top source: (无)
- Q12 (multihop): Kubernetes node is running out of memory, how to i
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md', 'linux-performance.md']
  - Top source: (无)
- Q13 (multihop): DNS resolution is slow for Kubernetes pods, how to
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md', 'network-diagnosis.md']
  - Top source: (无)
- Q14 (multihop): How to configure TLS certificates for a service ru
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md', 'network-diagnosis.md', 'security-hardening.md']
  - Top source: (无)
- Q15 (paraphrased): Database performance degraded after migrating to n
  - Verdict: (无)
  - 相关文档: ['postgresql-tuning.md', 'linux-performance.md']
  - Top source: (无)
- Q16 (paraphrased): Container cannot reach external API endpoint
  - Verdict: (无)
  - 相关文档: ['docker-troubleshooting.md', 'network-diagnosis.md']
  - Top source: (无)
- Q17 (paraphrased): Pods are being evicted from the cluster repeatedly
  - Verdict: (无)
  - 相关文档: ['kubernetes-debugging.md', 'linux-performance.md']
  - Top source: (无)
- Q18 (paraphrased): Application latency increased after the last deplo
  - Verdict: (无)
  - 相关文档: ['linux-performance.md', 'network-diagnosis.md']
  - Top source: (无)

## 环境

- Embedding: DashScope text-embedding-v2, 1536 维
- pgvector: halfvec(1536), HNSW m=16/ef_construction=200/ef_search=100
- Chunker: Markdown-aware, 500 字符 / 重叠 100
- CRAG: ThresholdEvaluator (0.40/0.70)