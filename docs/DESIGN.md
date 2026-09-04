# OpsMind 统一记忆系统设计

> 基于 `docs/research/unified-memory/` 调研报告（6 文件）产出。简洁纯净，只描述设计决策。

---

## 1. 核心类比

| OS 概念 | OpsMind 对应 | 物理实现 |
|---------|-------------|---------|
| L1 Cache | Agent 上下文窗口 | Eino ReactAgent 内存 |
| RAM | 会话记忆 | `memory/sessions/{id}/` |
| Disk | 全局记忆 + 知识库 | `memory/global/` + `kb/{kb_slug}/` |
| 页表 | 索引文件 | `MEMORY.md` / `INDEX.md` |
| 页错误 | memory_recall 工具 / RAG 检索 | 按需加载到上下文 |
| 分区 | 知识库分库 | `kb/{kb_slug}/` + `memory/global/` |
| 交换 | 上下文压缩 | Eino summarization + reduction |

上下文窗口是 L1 cache，不是 RAM——O(n²) 注意力导致窗口越大性能越差。

---

## 2. 文档组织架构

```
storage/
├── kb/                                    # 知识库（对外资产，审核后可引用）
│   └── {kb_slug}/                         # 知识库分区
│       ├── INDEX.md                        # 页目录（脚本自动重建）
│       ├── log.jsonl                       # 审计日志
│       ├── {doc_slug}/                     # 每篇文档一个文件夹（md + 图片）
│       │   ├── index.md
│       │   └── images/
│       └── {doc_slug}/
│           ├── index.md
│           └── images/
│
├── memory/                                # 记忆（Agent 自用，参考 Claude Code）
│   ├── global/                             # 全局记忆（跨会话）
│   │   ├── MEMORY.md                        # 索引（启动加载，≤200 行）
│   │   ├── pg-cluster-topology.md
│   │   └── k8s-crashloop-symptoms.md
│   └── sessions/                           # 会话记忆（单会话）
│       └── {session_id}/
│           ├── MEMORY.md
│           └── diagnosed-pg-cpu-issue.md
│
└── _index/                                # 派生索引（可重建，gitignored）
    ├── pgvector/
    ├── bm25/
    └── ingest_queue.jsonl
```

### 设计决策

| 维度 | 决策 | 理由 |
|------|------|------|
| kb 文档结构 | `{doc_slug}/index.md + images/` 文件夹式 | 文档有本地图片，需同目录存放 |
| kb 无分类子目录 | 分类靠 frontmatter `type` 字段 | 扁平存放，INDEX.md 按 type 分组渲染 |
| memory 扁平文件 | 参考 Claude Code `~/.claude/memories/` | 可检查、可编辑、无数据库 |
| memory 两个粒度 | `global/`（跨会话）+ `sessions/{id}/`（单会话） | 与系统相关与用户无关 |
| 文件即真相 | MD 文件 = 真相源；pgvector/BM25 = 派生索引 | 删除索引可从文件重建 |
| MEMORY.md ≤200 行 | 超出则合并旧条目 | 限制上下文膨胀 |

---

## 3. 文件格式

### 3.1 kb 文档

`kb/{kb_slug}/postgres-high-cpu/index.md`:

```yaml
---
title: "PostgreSQL 高 CPU 排障"
type: runbook
status: published
tags: ["postgres", "cpu"]
system: "pg-cluster"
severity: "SEV-2"
source_type: deep_research
sources:
  - url: "https://postgresql.org/docs/..."
    title: "PostgreSQL 官方文档"
created: "2026-09-04T10:00:00+08:00"
updated: "2026-09-04T10:00:00+08:00"
---

# PostgreSQL 高 CPU 排障

![CPU 监控图](images/pg-cpu-graph.png)
```

### 3.2 global 记忆

`memory/global/pg-cluster-topology.md`:

```yaml
---
name: pg-cluster-topology
description: PG 集群拓扑结构
created: "2026-09-04T10:00:00+08:00"
modified: "2026-09-04T10:00:00+08:00"
---

## 拓扑
- 3 节点流复制
- 主节点：192.168.1.10:5432
```

### 3.3 索引文件

`memory/global/MEMORY.md`:

```markdown
# Memory Index

- [pg-cluster-topology](pg-cluster-topology.md) — PG 集群拓扑：3 节点流复制
- [pg-cluster-vacuum-pattern](pg-cluster-vacuum-pattern.md) — vacuum 需定期检查
```

`kb/{kb_slug}/INDEX.md`:

```markdown
<!-- 自动生成，禁止手编 -->
# 知识库目录

## 运维手册
- [PostgreSQL 高 CPU 排障](postgres-high-cpu/) `runbook` `postgres` `SEV-2`

## 系统文档
- [PostgreSQL 集群架构](pg-cluster-architecture/) `architecture` `postgres`
```

---

## 4. 记忆层级

| 层级 | 物理存储 | 索引 | 检索方式 | 生命周期 |
|------|---------|------|---------|---------|
| L1 上下文窗口 | Agent 内存 | — | — | 当前会话 |
| L3 会话记忆 | `memory/sessions/{id}/*.md` | `MEMORY.md` | BM25 / 精确 | 单会话 |
| L4 全局记忆 | `memory/global/*.md` | `MEMORY.md` | BM25 | 跨会话 |
| Storage 知识库 | `kb/{kb_slug}/{doc_slug}/index.md` | `INDEX.md` | RAG 引擎 | 持久 |

---

## 5. Agent 记忆工具

| 工具 | 说明 | scope |
|------|------|-------|
| `memory_remember(text, scope, importance)` | 写入记忆 | session / global |
| `memory_recall(query, scope, limit)` | 检索记忆 | session / global / knowledge |
| `memory_forget(scope, key)` | 标记失效（非物理删除） | session / global |

`scope=knowledge` 时路由到 RAG 引擎（BM25 + pgvector + RRF + rerank）。

---

## 6. 上下文压缩（三级管线）

| 级别 | 触发 | 操作 | 有损 | 来源 |
|------|------|------|:---:|------|
| 1. HeadAndTail | 每轮 | 保留首尾，中间省略 | 否 | autogen |
| 2. 去重清理 | token > 70% | 丢弃重复 tool result | 否 | Eino reduction 中间件 |
| 3. Autocompact | token > 85% | LLM 摘要 | 是 | Eino summarization 中间件 |

Eino SDK 内置 `summarization` + `reduction` 中间件 + `CheckPointStore`，直接复用。

---

## 7. 异步处理管道

```
deep_research 产出 → kb/{kb_slug}/{doc_slug}/ (status: draft)
                          ↓ enqueue <5ms
                    _index/ingest_queue.jsonl
                          ↓ 定时消费者
                    去重 → 质量门 → 分解 → 重组 → 索引
                          ↓ 审核通过
                    status: published
                          ↓
                    _index/pgvector + bm25（索引构建）
                          ↓
                    INDEX.md（页目录刷新）
```

### 消费者设计（参考 membox）

- enqueue 只做一次 INSERT，<5ms 返回
- worker 用 lease 机制防并发（TTL 60s，过期自动接管）
- 崩溃恢复：processing → pending 自动重置
- 优雅降级：嵌入服务不可用时仍写入文件，标记 `pending_index`

---

## 8. 检索方案

### 8.1 分层检索

| scope | 检索方式 | 理由 |
|-------|---------|------|
| knowledge（大语料） | BM25 + 向量 + RRF + rerank | OpsMind 已有 RAG 引擎 |
| global（Agent 自记忆） | 纯文本 + BM25 | 规模小（几百条），Claude Code 实证 |
| session（会话记忆） | BM25 / 精确 | 不需要语义检索 |

### 8.2 BM25 为主

BM25 在零样本/域外场景下极强（BEIR 基准）。Claude Code 从向量库退回到纯文本 + grep + BM25。如果只能选一个，选 BM25。

### 8.3 GraphRAG

当前不需要。运维查询多为单跳事实查找。如果未来出现多跳关系查询，优先评估 LazyGraphRAG 或 HippoRAG 2。

---

## 9. 检索质量优化

### P0（立即提升，零 LLM 成本）

| 项 | 改动 | 效果 | 来源 |
|----|------|------|------|
| Sandwich Reorder | ~15 行 | Lost in the Middle 缓解 | Dify `reorder.py` |
| BM25 Enriched Texts | ~30 行 | 关键词召回率提升 | Open WebUI |
| RRF k 值可配置（默认 30） | 1 行 | rerank 候选质量提升 | RustyRAG k=20 |

### P1（显著提升）

| 项 | 改动 | 效果 | 来源 |
|----|------|------|------|
| **Contextual Retrieval** | ~100 行 | **失败率降低 49-67%** | Anthropic 2024.09 |
| Token-based Chunking | ~20 行 | chunk 大小一致性 | RAGFlow |
| Metadata 预过滤 | ~50 行 | 搜索空间缩小 | Dify |
| Context Packing | ~50 行 | context window 利用率 | typegraph.ai |

### Contextual Retrieval（最大优化机会）

对每个 chunk，LLM 生成 1-2 句上下文摘要 prepend 到 chunk 前，然后同时做 embedding 和 BM25 索引。

```
原始: "检查 pg_stat_activity 中的慢查询"
contextualized: "<context>PostgreSQL 高 CPU 排障手册，第二章</context>
                  检查 pg_stat_activity 中的慢查询"
```

Anthropic 实证：基线 5.7% → +Contextual 2.9% → +Rerank **1.9%**（-67%）。

---

## 10. 会话生命周期

### 启动
```
加载 global/MEMORY.md → 注入 L1 上下文（系统拓扑 + 近期经验）
```

### 会话中
```
每轮对话 → tool 调用 → 三级压缩管线 → checkpoint 持久化
memory_recall(scope=session/global/knowledge) 按需检索
```

### 会话结束
```
扫描 sessions/{id}/MEMORY.md
→ LLM 提取有长期价值的内容
→ memory_remember(scope=global) 写入 global/
→ 更新 global/MEMORY.md
→ 清理 sessions/{id}/（可选保留审计）
```

### 暂停/恢复
```
pause → Thread 序列化到 sessions/{id}/
resume → 加载 Thread → 继续执行
```

---

## 11. 与现有架构的映射

| 现有组件 | 统一架构中的角色 | 变更 |
|---------|----------------|------|
| RAG 引擎 | knowledge scope 检索后端 | 保留；+Sandwich Reorder +Contextual Retrieval |
| KnowledgeArticle + KnowledgeChunk | kb 文件索引 | 新增 `file_path` 指向 `{doc_slug}/index.md` |
| Eino ReactAgent | L1 上下文管理者 | +summarization +reduction 中间件 |
| SQLite（V1.3） | L3 CheckPointStore | 迁移到 `sessions/{id}/` 文件式 |
| 9 工具 + SubAgent | Agent 能力 | +memory_remember/recall/forget 工具 |
| chunker → embedder | 异步消费者步骤 | 复用，作为定时 worker 处理步骤 |

---

## 12. 实现优先级

| 优先级 | 项 | 依赖 | 改动量 |
|:------:|----|------|:-----:|
| P0 | memory_remember/recall/forget 工具 | — | ~100 行 |
| P0 | Eino summarization + reduction 配置 | — | ~20 行 |
| P0 | Sandwich Reorder + BM25 Enriched + RRF 调参 | RAG 引擎 | ~50 行 |
| P1 | `memory/global/` + `MEMORY.md` | memory 工具 | ~50 行 |
| P1 | `memory/sessions/{id}/` + 会话索引 | memory 工具 | ~50 行 |
| P1 | kb 文件式存储迁移 | — | ~100 行 |
| P1 | `_index/ingest_queue.jsonl` + 定时消费者 | kb 文件式 | ~150 行 |
| P1 | INDEX.md 自动重建 | kb 文件式 | ~30 行 |
| P1 | Contextual Retrieval | RAG 引擎 | ~100 行 |
| P2 | Token-based Chunking | chunker | ~20 行 |
| P2 | Metadata 预过滤 | Retriever 接口 | ~50 行 |
| P2 | Context Packing | pipeline | ~50 行 |
| P2 | 会话结束提取流 | global + session | ~80 行 |
| P3 | Consolidation Flow | global memory | ~100 行 |

---

## 13. 关联文档

| 文档 | 用途 |
|------|------|
| [`research/unified-memory/01-os-analogy.md`](../unified-memory/01-os-analogy.md) | OS 类比理论（MemGPT/Pichay/ClawVM 论文） |
| [`research/unified-memory/02-page-table-sharding.md`](../unified-memory/02-page-table-sharding.md) | 页表与分库设计 |
| [`research/unified-memory/03-async-pipeline.md`](../unified-memory/03-async-pipeline.md) | 异步处理管道 + Eino SDK 内置能力 |
| [`research/unified-memory/04-architecture.md`](../unified-memory/04-architecture.md) | 完整架构（含数据流图） |
| [`research/unified-memory/05-competitor-analysis.md`](../unified-memory/05-competitor-analysis.md) | 竞品知识库架构对比 |
| [`research/unified-memory/06-retrieval-optimization.md`](../unified-memory/06-retrieval-optimization.md) | 检索质量优化深挖 |
