# 🧠 RAG 引擎

OpsMind 自建了一套完整的 RAG（Retrieval-Augmented Generation）检索增强生成引擎，不依赖 LangChain 或 LlamaIndex 等第三方框架。

## 为什么自建？

| 考量 | 自建 | 第三方框架 |
|------|------|-----------|
| 可观测性 | 每步可跟踪、可审计 | 黑盒抽象，调试困难 |
| 可控性 | 管道参数完全可控 | 受框架设计约束 |
| 依赖 | 纯 Go + Python（仅重排序） | 引入 Python 依赖链 |
| 部署 | 单二进制 + 1 个子进程 | 额外服务/运行时 |
| 性能 | BM25 零网络开销 | 通常需外部检索服务 |

## 管道全景

```
用户问题（带会话历史上下文）
  │
  ├─ Step 1: 查询改写 (可选)
  │   LLM 利用最近 N 条历史消息消除指代歧义
  │   例："怎么解决？" → "VPN 连接超时怎么解决？"
  │
  ├─ Step 2: 多路检索 (可选)
  │   LLM 将改写后的问题分解为 2-4 个不同角度的子查询
  │   例："VPN 连接超时" → ["VPN 超时排查步骤", "VPN 网络配置", "VPN 客户端设置"]
  │
  ├─ Step 3: 向量检索 (核心，不可降级)
  │   使用原始 question 生成 embedding 向量
  │   pgvector cosine 相似度检索 Top K 个分块
  │
  ├─ Step 4: BM25 检索 (可选)
  │   Go 原生 Okapi BM25 算法 + gse 中文分词
  │   对同一候选池进行稀疏检索
  │
  ├─ Step 5: RRF 融合 (可选，依赖 3+4)
  │   Reciprocal Rank Fusion (k=60)
  │   融合向量和 BM25 的双路排序结果
  │
  ├─ Step 6: 重排序 (可选)
  │   Cross-encoder 模型对候选分块重新精排
  │   使用原始 query（非改写后），截断 top rerank_count
  │
  └─ Step 7: LLM 生成 (核心，不可降级)
      将检索到的上下文组装为 prompt
      SSE 流式输出 token 级结果
```

## 管道配置

所有步骤可以独立开关，通过 `rag_options` 参数传入：

```go
type RAGOptions struct {
    TopK         int                     // 最终返回的分块数（默认 5）
    QueryRewrite bool                    // 是否启用查询改写
    MultiRoute   bool                    // 是否启用多路检索
    Hybrid       bool                    // 是否启用 BM25 + RRF 融合
    Rerank       bool                    // 是否启用重排序
    RouteCount   int                     // 多路检索子查询数（默认 3）
    RerankCount  int                     // 重排序候选池大小（默认 TopK × 3）
    History      []map[string]string     // 会话历史（供查询改写使用）
}
```

`Normalize()` 方法在管道入口自动填充零值默认值，避免异常行为。

## 降级矩阵

单步骤失败不应阻塞整个管道。降级策略如下：

| 步骤 | 失败行为 | 触发条件 |
|------|----------|----------|
| 查询改写 | ✅ 降级—使用原始 question | LLM 调用失败或 llmClient == nil |
| 多路检索 | ✅ 降级—使用单路原始查询 | LLM 调用失败或 llmClient == nil |
| 向量检索 | ❌ **阻塞**—返回错误 | pgvector 不可达 |
| BM25 检索 | ✅ 降级—仅用向量结果 | 分词或检索异常 |
| RRF 融合 | ✅ 降级—使用单路排序 | 仅有一路结果时自然降级 |
| 重排序 | ✅ 降级—使用融合排序 | reranker == nil 或子进程异常 |
| LLM 生成 | ❌ **阻塞**—返回错误 | LLM 不可达 |

> 核心路径（向量检索、LLM 生成）不可降级——无检索结果或无法生成答案时，必须向用户明确反馈而非静默失败。

## 组件详解

### 查询改写 (query_rewrite.go)

利用 LLM 和会话历史上下文消除用户问题中的指代歧义。

- **输入：** 当前 question + 最近 N 条历史消息（由 `History` 参数控制）
- **输出：** 改写后的明确问题文本
- **Prompt 策略：** 指示 LLM 结合历史上下文补全指代信息，不改变原始意图
- **失败降级：** 返回原始问题文本

### 多路检索 (multi_route.go)

将改写后的问题分解为多个不同角度的子查询，提升检索召回率。

- **输入：** 改写后的问题文本
- **输出：** JSON 数组格式的 2-4 个子查询字符串
- **钳位机制：** 无论 LLM 返回多少条，强制截断到 [2, 4] 范围
- **失败降级：** 降级为单路检索（使用原问题）

### 向量检索 (retriever.go)

- **Embedding 生成：** 通过 `EmbeddingClient` 调用 `/v1/embeddings` 端点
- **向量存储：** pgvector 的 `halfvec` 类型（float16 半精度，节省 50% 存储）
- **索引：** HNSW 索引，`halfvec_cosine_ops` 算子
- **距离函数：** `<=>`（cosine distance）
- **性能：** 10,000 个分块下查询 < 50ms

### BM25 检索 (bm25.go)

- **算法：** Okapi BM25（k1=1.5, b=0.75）
- **分词：** gse 中文分词器（纯 Go，支持自定义词典）
- **实现：** 内存中的倒排索引，每次检索时动态计算 BM25 分数
- **特点：** 零网络开销、零外部依赖，擅长关键词精确匹配

### RRF 融合 (hybrid.go)

Reciprocal Rank Fusion 将向量检索和 BM25 检索的排序结果融合：

```
RRF_score(d) = Σ 1 / (k + rank_i(d))

其中 k = 60，rank_i(d) 是文档 d 在第 i 个排序列表中的排名
```

RRF 的优势在于无需分数归一化，直接基于排名位置融合。

### 重排序 (rerank.go)

使用 cross-encoder 模型对候选分块精排，弥补双塔模型（embedding）的语义损失。

- **实现方式：** Python 子进程，通过 stdin/stdout JSON Lines 协议通信
- **模型加载：** 子进程启动时加载一次，常驻内存，单次推理约 50ms
- **候选截断：** 重排前按 `RerankCount` 截断候选池（默认 TopK × 3 = 15）
- **降级：** 子进程不可用时静默跳过

## 文档处理管道

上传文档（PDF / DOCX / MD / TXT）的后台异步处理流程：

```
文件上传 → MinIO
  │
  └─ 异步处理 (goroutine pool)
       │
       ├─ parsing    — document_parser.go（PDF→pdfcpu、DOCX→docx2md、MD/TXT 直接读取）
       ├─ chunking   — RecursiveCharacterTextSplitter（chunk_size=1000, overlap=200）
       ├─ embedding  — 批量调用 Embedding API
       └─ indexing   — 批量写入 pgvector (halfvec)
            │
            └─ completed → 可参与 RAG 检索
```

处理状态机：`pending → parsing → chunking → embedding → indexing → completed`，失败时标记 `failed` 并记录错误信息，支持重试。

## 错误处理

所有 AI/RAG 相关错误通过统一错误码暴露：

| 错误码 | HTTP | 含义 |
|--------|------|------|
| 20001 | 503 | AI 服务不可用（LLM / Embedding 调用失败） |
| 20002 | 503 | RAG 服务不可用（pgvector 检索失败） |

服务不可用时，前端展示明确提示而非空白页面，引导用户稍后重试或提交申告。
