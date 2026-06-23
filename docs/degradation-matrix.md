# OpsMind 系统降级能力全景审计

> **版本：** v1.0 | **日期：** 2026-06-23 | **范围：** 全系统 5 层架构

---

## 目录

1. [总览](#1-总览)
2. [RAG 管道降级矩阵](#2-rag-管道降级矩阵)
3. [LLM / Embedding 服务降级](#3-llm--embedding-服务降级)
4. [知识库降级矩阵](#4-知识库降级矩阵)
5. [申告工单降级矩阵](#5-申告工单降级矩阵)
6. [存储适配层降级](#6-存储适配层降级)
7. [会话与流式降级](#7-会话与流式降级)
8. [配置热替换](#8-配置热替换)
9. [跨层缺陷清单](#9-跨层缺陷清单)
10. [修复优先级建议](#10-修复优先级建议)

---

## 1. 总览

```
用户请求
  │
  ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Handler 层   │───▶│  Service 层   │───▶│ Repository 层 │
│  参数校验     │    │  业务逻辑     │    │  数据访问     │
└──────────────┘    └──────┬───────┘    └──────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌──────────┐
        │RAG 引擎 │  │Adapter  │  │Middleware│
        │7步管道  │  │LLM/Embed│  │JWT/RBAC  │
        └─────────┘  │/pgvector│  └──────────┘
                     │/MinIO   │
                     └─────────┘
```

**降级原则：** 单步骤失败不阻塞后续步骤，核心路径（向量检索 + LLM 生成）阻塞。外部依赖不可用时返回明确错误码而非静默失败。

---

## 2. RAG 管道降级矩阵

管道执行顺序：查询改写 → 多路检索 → 向量检索 → BM25 检索 → RRF 融合 → 重排序 → LLM 生成

| 步骤 | 失败行为 | 降级结果 | 错误码 | 代码位置 |
|------|----------|----------|--------|----------|
| 查询改写 | **降级** | 使用原始 question | — | `pipeline.go:107` `query_rewrite.go:55-58` |
| 多路检索 | **降级** | 单路检索 | — | `pipeline.go:120` `multi_route.go:57-59` |
| 向量检索 | **阻塞** | 管道终止 | `20002` | `pipeline.go:147-149` |
| BM25 检索 | **降级** | BM25 结果为空 | — | `pipeline.go:152` `bm25.go:307-317` |
| RRF 融合 | 条件降级 | 单路结果 | — | `pipeline.go:171-179` |
| 重排序 | **降级** | 使用原始排序 | — | `pipeline.go:209` `rerank.go:36-82` |
| LLM 生成 | **阻塞** | 兜底文本 | `20001` | `llm_service.go:131-137` |

### 2.1 向量检索详细分析

**代码位置：** `rag/retriever.go:30-47`

```go
func (r *VectorRetriever) Retrieve(ctx, query, kbID, topK) {
    if r.store == nil { return nil, nil }          // ← 静默返回空（无错误）
    if r.embedder == nil { return nil, error }     // ← 阻塞
    vectors, err := r.embedder.Embed(query)         // ← API 调用失败 → 阻塞
    results, err := r.store.CosineSearch(vectors)   // ← pgvector 失败 → 阻塞
}
```

| 失败原因 | 行为 | 诊断 |
|----------|------|------|
| `store == nil` | 返回 `nil, nil`（空结果） | **无法区分**"未配置"与"无匹配" |
| `embedder == nil` | 返回 error，管道终止 | 明确阻塞 |
| Embedding API 不可达 | 返回 error，管道终止 | 明确阻塞 |
| pgvector 查询失败 | 返回 error，管道终止 | 明确阻塞 |

### 2.2 BM25 检索详细分析

**代码位置：** `rag/bm25.go:293-317`

| 失败原因 | 行为 |
|----------|------|
| 索引不存在 | 返回空结果（无错误） |
| 索引过期 + 重建失败 | 返回空结果（无错误） |
| Context 取消 | 返回 `ctx.Err()`（管道丢弃错误） |

### 2.3 重排序详细分析

**代码位置：** `adapter/rerank_client.go:103-111, 182-194`

| 场景 | 行为 |
|------|------|
| Python 子进程未安装 | `NewSubprocessReranker` 返回 `nil`，管道跳过 |
| 子进程启动失败 | 同上 |
| 子进程运行中崩溃 | 所有待处理请求收到 `"rerank 子进程已退出"` 错误，已处理请求降级 |
| 子进程崩溃后 | **永不重启**，所有后续请求降级。恢复需重启服务 |

> **设计决策：** 子进程不自动重启，原因：模型重载需 2-5s，期间所有请求超时，不如直接降级。

---

## 3. LLM / Embedding 服务降级

### 3.1 LLM 调用降级

**代码位置：** `adapter/llm_client.go:284-309, 173-209`

| 场景 | 同步 ChatCompletion | 流式 ChatCompletionStream |
|------|---------------------|--------------------------|
| API 不可达 | 不重试，立即返回错误 | 重试 3 次（指数退避） |
| HTTP 429/503 | 重试 3 次 | 重试 3 次 |
| 连接超时 | 不重试，返回错误 | 重试（进入通用重试分支） |
| 流式中途断开 | — | 发送 `error` chunk，流终止 |

**LLM 兜底文本（3 种场景）：**

| 场景 | 兜底文本 | 位置 |
|------|----------|------|
| RAG 检索无结果 | `"暂未找到足够匹配的知识，建议提交申告由运维人员人工处理。"` | `llm_service.go:207` |
| LLM 客户端为 nil | 检索到的 chunks 直接作为文本列表返回 | `llm_service.go:157` |
| LLM API 调用失败（同步） | `"AI 服务不可用，请稍后重试或提交申告。"` | `llm_service.go:151` |
| AI 服务完全未初始化 | `"当前 AI 服务暂不可用，请提交申告由人工处理"` | `chat_service.go:24` |

### 3.2 Embedding 调用降级

**代码位置：** `adapter/embedding_client.go:96-108, 130-155`

| 场景 | 行为 |
|------|------|
| API 不可达 | 不重试，立即返回错误 |
| API 返回非 200 | 不重试，立即返回错误 |
| 批次失败 | 快速失败——丢弃全部已完成批次的向量 |

> **注意：** Embedding API 调用失败**不**是 `retryableError`。连接/超时不重试——与 LLM 的流式调用行为不对称。

---

## 4. 知识库降级矩阵

### 4.1 文章发布管道

```
分块 → Embedding → 删除旧向量 → 写入新向量 → 更新状态
```

| 失败步骤 | 文章状态 | 向量状态 | 是否可恢复 |
|----------|----------|----------|-----------|
| 分块（空内容） | 保持审核通过 | 旧向量未动 | ✓ 重试发布 |
| Embedding API 失败 | 保持审核通过 | 旧向量未动（删除在 Embedding 之后） | ✓ 重试发布 |
| pgvector 写入失败 | 保持审核通过 | **旧向量已删除，新向量未写入** | ⚠️ 暂时无向量，重试恢复 |
| 状态更新失败 | 保持审核通过 | 向量已写入 | ⚠️ 数据库不一致 |

### 4.2 文档上传与处理

```
上传MinIO → 创建Article → 提交处理队列 → 异步：解析→分块→Embedding→写向量
```

| 失败场景 | 行为 | 一致性 |
|----------|------|--------|
| MinIO 上传失败 | 返回 `20003`，Article 未创建 | ✅ |
| MinIO 成功 + Article 创建失败 | 尝试清理 MinIO 文件（best-effort） | ⚠️ 清理失败→MinIO 孤立文件 |
| Article 创建成功 + 处理器提交失败 | Article 留存（草稿），文件在 MinIO | ⚠️ 通过 RetryDocument 恢复 |
| 处理器 Worker panic | `recover()` 捕获，标记 `process_status=failed` | ✅ |
| 处理器队列满 | 返回 `"处理队列已满，请稍后重试"` | ✅ |

### 4.3 重试机制

**代码位置：** `knowledge_service.go:780-798`

- **触发条件：** 仅 `process_status == "failed"` 可重试
- **操作：** 重置为 `pending`，重新提交到处理队列
- **缺失：** 无自动重试、无指数退避、无死信队列

---

## 5. 申告工单降级矩阵

| 操作 | 失败场景 | 行为 | 一致性 |
|------|----------|------|--------|
| 创建工单 | DB 插入失败 | 返回原始错误 | ✅（无副作用） |
| 状态更新 | 事务内任意步骤失败 | 全部回滚 | ✅ |
| 消息通知 | 通知发送失败 | **静默吞没**，仅 `slog.Warn` | ✅（通知是尽力而为） |
| 知识候选生成 | 文章创建失败 | 返回错误，工单不受影响 | ✅（后置独立操作） |
| 自动关闭调度 | 单条记录插入失败 | 事务回滚，**所有工单的关闭被阻塞** | ⚠️ 一个坏记录阻塞全量 |
| 自动关闭调度 | 整体失败 | 1 小时后重试 | ✅ |
| 调度器生命周期 | `ctx.Done()` | 不等待进行中的自动关闭完成即退出 | ⚠️ |

---

## 6. 存储适配层降级

### 6.1 pgvector (VectorStore)

| 操作 | 失败行为 | 回滚 |
|------|----------|------|
| `BatchInsert` | 单条 SQL，语句级原子性 | ✅ PG 保证全或无 |
| `DeleteByArticle` | 返回错误给调用者 | ✅ 单条 DELETE |
| `CosineSearch` | 返回错误给调用者 | ✅ 只读操作 |
| 启动时 pgvector 不可用 | **不阻塞启动**，`slog.Warn` | ⚠️ `vectorRetriever` 为 nil → 后续使用会 panic |

> **🔴 严重缺陷：** `main.go:222-225` 中，如果 pgvector 初始化失败，`vectorRetriever` 为 nil。但这个 nil 指针被赋值给 `Retriever` 接口后变成非 nil 接口值。`retriever.go:30-32` 检查 `r.store == nil` 返回空结果，但 `retriever.go` **不检查 `r` 自身是否为 nil**。当 `VectorRetriever` 为 nil 且赋值给接口后调用方法时，Go 会尝试解引用 nil 接收者——如果方法访问了 `r.store` 之外的字段，会导致 **nil pointer dereference panic**。

### 6.2 MinIO (StorageClient)

| 操作 | 失败行为 | 备注 |
|------|----------|------|
| 上传 | 返回 `20003`，Article 未创建 | ✅ |
| 下载 | **惰性求值**：`GetObject` 成功不代表数据可读 | ⚠️ 读取阶段失败无明确信号 |
| 删除 | 失败仅 `slog.Warn`，不阻塞 | ⚠️ 无验证删除是否生效 |
| 启动时 MinIO 不可用 | **阻塞启动**（`ensureBucket` 失败则 os.Exit） | ⚠️ |

### 6.3 Reranker 子进程

| 场景 | 行为 |
|------|------|
| 子进程未安装/启动失败 | 降级，`reranker == nil`，管道跳过重排序 |
| 子进程运行中崩溃 | 所有待处理请求通知错误，已处理请求降级 |
| 崩溃后恢复 | **不重启**，永久降级直到服务重启 |
| Context 取消/超时 | 返回错误，降级为原始排序 |

---

## 7. 会话与流式降级

### 7.1 SSE 流式事件

**代码位置：** `llm_service.go:186-291`

| 场景 | SSE 事件 | 问题 |
|------|----------|------|
| RAG 管道失败 | `{"type":"error"}` | 无后续 `done` 事件 |
| LLM 流式中断 | `{"type":"error"}` | 已累积的答案缓冲区**被丢弃** |
| Context 取消 | 通道关闭（无事件） | 前端既无 `error` 也无 `done` |
| DB 持久化失败 | `done` 事件正常发送 | **答案返回给用户但未持久化** |

### 7.2 数据库失败伪装

**代码位置：** `chat_service.go:133-136, 242-244`

```
DB 宕机时的错误链路：
  FindByID → gorm 返回 "connection refused" 
    → chat_service 包装为 AppError{Code: 10004, Message: "会话不存在"}
```

| 实际原因 | 返回给用户 |
|----------|-----------|
| DB 连接拒绝 | `"会话不存在"` (10004) |
| DB 超时 | `"会话不存在"` (10004) |
| 会话真的不存在 | `"会话不存在"` (10004) |

> 🔴 三种完全不同的故障模式被折叠为同一个错误码和消息。用户无法判断需要"重试"还是"创建新会话"。

### 7.3 错误事件无 sendOrCancel 保护

**代码位置：** `llm_service.go:201, 243, 257`

```go
eventCh <- StreamEvent{Type: "error", Error: err.Error()}  // ← 直接 channel 发送
```

其他所有事件使用 `sendOrCancel` 包装以避免消费者已停止时的 goroutine 泄漏，但错误事件是裸发送。如果消费者已退出，这条 goroutine 会永久阻塞。

---

## 8. 配置热替换

**代码位置：** `service/llm_config_service.go:47-53, main.go:272-297`

```
Admin API: PUT /api/v1/admin/llm-configs/:id {is_default: true}
  → llm_config_service.store(cfg)
    → clone := *cfg                          // struct 拷贝
    → m.current.Store(&clone)                // atomic.Value 写入
    → m.onChange()                           // 回调
      → 创建新的 OpenAIClient(baseURL, apiKey, timeout)
      → llmService.SetLLMClient(newLLM)       // 指针替换
```

| 场景 | 行为 |
|------|------|
| 进行中的流式对话 | **不受影响**——goroutine 持有旧 client 指针 |
| 新请求 | 使用新 client |
| 删除默认配置 | **被拒绝**——必须先设置其他为默认 |
| 默认配置丢失（人为/故障） | 静默降级为硬编码 fallback（模型: `qwen3-4b`, 无系统提示词） |
| 并发热替换 | `s.llmClient` 指针替换无同步——**数据竞争** |

---

## 9. 跨层缺陷清单

### 🔴 Critical（会导致崩溃或数据丢失）

| # | 缺陷 | 位置 | 影响 |
|---|------|------|------|
| 1 | **nil VectorRetriever → panic**。pgvector 初始化失败后，nil 接收者赋值给接口，调用 `Retrieve` 可能 panic | `main.go:222-225` `retriever.go:30-32` | 向量检索触发服务崩溃 |
| 2 | **tryRefreshIndex panic 无 recovery**。`buildIndex()` panic 时 `building[kbID]` 标志未清除 + 写锁未释放 → 永久死锁 | `bm25.go:378-386` | BM25 索引永久不可用 |
| 3 | **DB 宕机伪装为"会话不存在"**。所有 `FindByID` 失败返回 `10004`，不区分 DB 故障和真的不存在 | `chat_service.go:133-136` | 用户收到误导性错误 |

### 🟠 High（会导致功能异常或不一致）

| # | 缺陷 | 位置 | 影响 |
|---|------|------|------|
| 4 | **流式中断丢弃已累积答案**。`answerBuf` 在 `error` 事件时被丢弃，前端收不到部分回答 | `llm_service.go:256-258` | 用户看到报错但实际已有大部分有效回答 |
| 5 | **多路检索失败返回 nil error**。LLM 调用失败时 `MultiRoute` 返回 `nil` 而非 `error`，管道日志显示 `Success=true` | `multi_route.go:57-59` | 故障不可观测 |
| 6 | **llmClient 指针替换无同步**（数据竞争）。`SetLLMClient` 写与 `StreamChat` 读无 mutex 保护 | `llm_service.go:104 vs 141,220` | 理论上可导致读到损坏指针 |
| 7 | **自动关闭：一条坏记录阻塞全量**。事务中任一条 Record 插入失败，所有工单的回滚关闭 | `ticket_service.go:515-528` | 定期任务可能长期空转 |
| 8 | **发布失败后向量丢失（临时窗口）**。旧向量删除成功但新向量写入失败，文章暂时无向量 | `knowledge_service.go:433-454` | RAG 搜索出现空洞 |

### 🟡 Medium（影响可观测性或恢复能力）

| # | 缺陷 | 位置 | 影响 |
|---|------|------|------|
| 9 | **重排序子进程崩溃后永不重启**。恢复需重启服务 | `rerank_client.go:178-181` | 运维负担 |
| 10 | **MinIO 惰性下载——读取错误被吞**。`defer reader.Close()` 不检查错误 | `storage_client.go:125-131` `processor.go:201` | 文档处理失败但未记录 |
| 11 | **ListSessions 泄露原始 DB 错误**。未包装为 AppError | `chat_service.go:322-324` | 可能泄露 SQL/连接信息 |
| 12 | **Embedding 连接/超时不重试**。与 LLM 流式调用不对称 | `embedding_client.go:150-152` | 临时网络抖动即失败 |
| 13 | **错误事件未使用 sendOrCancel**。消费者退出时 goroutine 可能永久阻塞 | `llm_service.go:201,243,257` | Goroutine 泄漏 |
| 14 | **无去重优先按最高分**。RRF 失败降级为单路结果时保留第一个而非最高分的 chunk | `pipeline.go:249-258` | 检索质量轻微下降 |

### 🟢 Low（体验或一致性微瑕）

| # | 缺陷 | 位置 |
|---|------|------|
| 15 | 无向量检索模式开关——无法做纯 LLM 对话 | `rag/types.go:40-49` |
| 16 | Sync/Stream 兜底文本不一致 | `llm_service.go:151 vs chat_service.go:24` |
| 17 | 默认配置缺失时无日志警告 | `llm_service.go:336-339` |
| 18 | 无 DB/LLM 健康检查或熔断器 | 全局 |
| 19 | 重排序无内部超时，仅依赖调用者 ctx | `rerank_client.go:247-249` |

---

## 10. 修复优先级建议

| 优先级 | 缺陷编号 | 建议修复 | 预计工作量 |
|--------|----------|----------|-----------|
| P0 | #1 | `main.go` 中 vectorRetriever 为 nil 时不传入 Pipeline，或在 Pipeline 中增加 nil 检查并降级 | 0.5h |
| P0 | #2 | `tryRefreshIndex` 增加 `defer recover()` + 清除 building 标志 | 0.5h |
| P1 | #3 | `chat_service` 区分 `gorm.ErrRecordNotFound` 和 DB 连接错误 | 1h |
| P1 | #4 | 流式中断时发送 `done` 事件携带已累积的 `answerBuf` | 1h |
| P1 | #5 | `multi_route.go:57-59` 返回 error 而非 nil | 0.2h |
| P2 | #6 | `SetLLMClient` 加 `sync.Mutex` 或改用 `atomic.Pointer` | 0.5h |
| P2 | #8 | 发布改为先写新向量 → 再删旧向量（或使用事务） | 2h |
| P2 | #15 | `RAGOptions` 增加 `DisableRetrieval` 字段，跳过向量检索直接 LLM 对话 | 1h |
| P3 | #7, #9-#14, #16-#19 | 逐步修复，优先 #9（reranker 自恢复）、#13（sendOrCancel）、#11（错误泄露） | 各 0.5-1h |

---

> **审计结论：** 系统核心路径（向量检索、LLM 生成）的降级策略明确且可追溯。主要风险集中在 **nil pointer safety**（#1, #2）、**错误信息伪装**（#3）、**流式中断数据丢弃**（#4）三处。建议 P0/P1 缺陷在下次迭代中优先修复。
