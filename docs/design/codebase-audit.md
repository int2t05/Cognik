# OpsMind 架构深化审计报告

> v1.2 分支 · 前后端全量扫描 · 8 个深化机会 · Top 3 推荐

## 审计范围

- 后端：`server/internal/` 全部包（domain/infra/shared/rag/parser/router）
- 前端：`web/src/` 全部组件/页面/hooks/lib
- 方法：浅模块识别、跨模块跳跃、消费者接口 drift、测试可达性、耦合热点

## 健康基线（无需深化）

以下模块已是好的深模块（小接口 + 大实现）：

| 模块 | 接口 | 实现 | 评估 |
|---|---|---|---|
| VectorStore | 8 方法 | 361 行（pgvector + halfvec + HNSW） | 健康 |
| StorageClient | 4 方法 | 双实现（MinIO 165 行 + Local 120 行） | 健康 |
| Reranker | 1 方法 | 259 行（Python 子进程 + 自动重启） | 健康 |
| LLMClient | 2 方法 | 420 行（同步 + 流式 SSE + 重试） | 健康 |
| Parser | 1 方法 | 双引擎（MinerU → 本地降级） | 健康 |
| 前端依赖链 | — | AppShell → hooks → api → client（4 层） | 健康 |

## 深化机会

### 1. knowledge 重复消费者接口（Worth exploring）

**文件**：`domain/knowledge/service.go` L64-74, `rag/types.go` L24-31, `parser/parser.go`

**问题**：knowledge 包定义 3 个消费者接口，签名与 rag/parser 包完全相同：
- `knowledgeChunker.Split(text string) []string` ≡ `rag.TextChunker.Split`
- `knowledgeEmbedder.Embed(ctx, texts, model) (...)` ≡ `rag.TextEmbedder.Embed`
- `knowledgeDocParser.Parse(reader, fileType) (...)` ≡ `parser.Parser.Parse`

这些本地接口不保护任何依赖边界——是纯粹的重复定义。签名变化会静默漂移。

**方案**：WithChunker/WithEmbedder/WithDocParser 参数类型改为 `rag.TextChunker` / `rag.TextEmbedder` / `*parser.Parser`。删除 3 个本地接口。

**收益**：locality（rag/parser 拥有契约唯一所有权）+ leverage（签名变更编译期捕获）

---

### 2. 知识发布流程跨模块跳跃（Worth exploring）

**文件**：`domain/knowledge/service.go`, `rag/processor.go`, `parser/parser.go`, `infra/storage`, `infra/adapter/vector_store.go`

**问题**：ProcessTask 携带 2 个 `func` 回调（OnStatusChange / OnMetrics），闭包捕获 knowledge 的 repo，制造 rag→knowledge 隐式依赖。理解"发布一篇文章"需追踪 5 文件 3 包。

**方案**：回调替换为显式事件通道：Processor.Submit 返回 `<-chan ProcessEvent`，knowledge 作为消费者订阅。事件类型定义在 rag 包。

```
Before: knowledge → Processor(闭包捕获repo) → 回调到knowledge
After:  knowledge → Processor → 事件channel → knowledge(显式事件处理)
```

**收益**：locality（协议集中）+ leverage（未来多消费者通过显式事件交互）

---

### 3. AuditWriter 漂移 — 7 处 3 种形状（Strong）

**文件**：`system/audit/service.go`（权威）+ 6 个消费者

**问题**：3 种形状：
- 形状 A（Write + WriteWithTx）：ticket, user/role, user/account — 4 处
- 形状 B（仅 Write）：knowledge, system/config, chat/llm_config — 3 处，无法事务内写审计
- 形状 C（Create(ctx, log any)）：chat/session — 1 处，接口名和签名不同

形状 B 的 3 个包无法在事务中写审计——这是实际正确性差异（knowledge.Publish 审计在事务外，ticket.UpdateStatus 审计在事务内）。

**方案**：system/audit 定义单一 AuditWriter 接口（Write + WriteWithTx），所有消费者 import。删除 6 处本地副本。

**收益**：locality（契约唯一所有权）+ leverage（签名变更编译期捕获）+ 正确性（所有消费者获得事务能力）

---

### 4. ChatStreamProvider 不可测状态机（Strong）

**文件**：`contexts/ChatStreamProvider.tsx`（consume 函数，90 行）

**问题**：5 条隐式行为契约困在 React 组件中，零测试覆盖：
1. rAF token 批处理（合并 50+ token/s 为 ≤60fps）
2. seq 去重（断线续传安全）
3. reasoning 独立 rAF 槽位
4. cancel 回滚（移除末尾消息对）
5. done/error rAF 清理

consume 直接调用 `patch`（setStreams）和 `requestAnimationFrame`，无法在 vitest 中单测。

**方案**：提取纯函数 `StreamReducer`：`reduce(state: SessionStream, event: SSEEvent) → SessionStream`。所有协议逻辑移入纯函数，ChatStreamProvider 只负责 fetch + rAF 调度。

**收益**：locality（协议集中可读）+ leverage（组件变薄）+ testability（从 0 到有测试覆盖）

---

### 5. parser Python 子进程测试（Speculative）

**文件**：`parser/local/doc.go`

**评估**：seam 已干净（Parse 接口 + 三级降级策略）。Python 依赖是旧格式解析的固有复杂度，非设计缺陷。no-mock 策略与外部进程依赖冲突，但这是约束而非设计问题。

---

### 6. wireApp 509 行手工 DI（Worth exploring）

**文件**：`cmd/main.go`（wireApp, 509 行）

**问题**：单函数装配 5 adapter + 9 repo + 9 service + LLM 热替换回调（25 行内联）+ BM25 重建闭包。新增领域必须改此函数。

**方案**：按领域拆分 Wire 函数：每个 domain 导出 `Wire(db, deps...) (*Service, *Handler)`，main.go 缩减为 ~80 行组合根。

**收益**：locality（每领域拥有自己 wiring）+ leverage（新增领域只改对应 Wire + main 一行）

---

### 7. 两阶段构造 — SetFeedbackMarker setter（Worth exploring）

**文件**：`cmd/main.go` L301/349, `domain/ticket/service.go` L63-75

**问题**：TicketService 构造时传 `nil` 作为 feedbackMarker，ChatService 创建后再 `SetFeedbackMarker(chatService)` 注入。

**方案**：依赖图无环（knowledge→llm→chat→ticket），重排构造顺序即可在构造时直接注入，删除 2 个 setter，构造后不可变。

---

### 8. router.Handlers 聚合（Speculative）

**文件**：`router/router.go` L27-39

**评估**：可接受的浅模块。深化会分散路由定义，降低可读性。

## Top 3 推荐

| # | 机会 | 强度 | 核心收益 |
|---|---|---|---|
| 1 | 提取 SSE 流协议状态机 | Strong | 从 0 到有测试覆盖，5 条契约变可见 |
| 2 | 统一 AuditWriter 接口 | Strong | 消除正确性差异 + 编译期 drift 检测 |
| 3 | 消除 knowledge 重复接口 + 重排构造 | Worth exploring | 纯删除无行为变更 + 构造后不可变 |

**未选但记录**：
- 发布流程回调→事件通道：收益高但改动面大，适合 V2.0 agent 重构时处理
- wireApp 拆分：真实耦合热点，可作为单独迭代
