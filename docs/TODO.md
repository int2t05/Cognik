# OpsMind 代码改进清单

> 基于 2026-06-17 全量深检（11 个并行 Agent 覆盖 ~171 源文件，300+ 发现项，交叉验证去重）。
> 覆盖 [diagrams/](diagrams/) 中 10 份业务流程图对应的全部模块 + [API/](API/) 9 份接口文档。
> 优先级：🔴 P0 生产隐患 / 🟡 P1 架构债务 / 🟢 P2 优化改进
> ⭐ 标记为 2026-06-17 审计新发现。
> 📝 标记为文档一致性缺陷（代码实现与 API 文档/PRD/TECH.md 不一致）。
> 📌 标记为代码中已存在 TODO 注释，本文档同步收录。

---

## 1. 认证与授权

> 对应图：[auth-flow.md](diagrams/auth-flow.md) — 登录 → JWT 双令牌 → 中间件链 → RBAC 权限校验
> 对应文档：[API/auth.md](API/auth.md)

### 密码策略

- ✅ [pkg/hash/hash.go](/server/pkg/hash/hash.go) — 字节计数 `utf8.RuneCountInString`
- ✅ [pkg/hash/hash.go](/server/pkg/hash/hash.go) — `unicode.IsLower/IsUpper/IsDigit` 统一检测
- 🟢 [pkg/hash/hash.go](/server/pkg/hash/hash.go) — bcrypt cost=10 硬编码，应可配置以应对硬件升级

### JWT 令牌

- ✅ [pkg/jwt/jwt.go](/server/pkg/jwt/jwt.go) — alg 限制 `WithValidMethods([]string{"HS256"})`
- ✅ [pkg/jwt/jwt.go](/server/pkg/jwt/jwt.go) — secret 为空时 `generateToken` 显式校验
- ✅ [pkg/jwt/jwt.go](/server/pkg/jwt/jwt.go) — 标准声明 Issuer/Subject/ID(jti)/IssuedAt

### 认证服务

- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — jwtSecret 构造注入 `config.JWT.Secret`
- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — token 有效期读取 `AccessExpire`/`RefreshExpire`
- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — RefreshToken 校验 `TokenType != "refresh"`
- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — 登录失败限流器（5 次/15 分钟滑动窗口）
- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — Login 内 slog 记录失败审计日志（不泄露用户名是否存在）
- ✅ [service/auth_service.go](/server/internal/service/auth_service.go) — Logout 内存黑名单 + RefreshToken 前置校验
- ✅ **[2026-06-17]** [service/auth_service.go](/server/internal/service/auth_service.go) — **blacklistCleanupLoop goroutine 泄漏**。**修复：新增 `stopCh chan struct{}` + `Shutdown()` 方法，loop 通过 `select { case <-stopCh: return }` 优雅退出。main.go 优雅关闭链中调用 `authService.Shutdown()`。**
- ✅ **[2026-06-17]** [service/auth_service.go](/server/internal/service/auth_service.go) — **ChangePassword 全字段 Save 丢失更新**。**修复：`s.db.Model(&model.User{}).Where("id = ?", userID).Updates(...)` 仅写 `password_hash`/`first_login` 两个字段，不再覆盖并发写入。**
- 🟡 [service/auth_service.go](/server/internal/service/auth_service.go) — 管理员检测硬编码角色名 `"系统管理员"`，角色更名后静默失效。Role 模型应增加 `IsAdmin bool` 字段。
- ✅ **[2026-06-17]** [service/auth_service.go](/server/internal/service/auth_service.go) — Login/Logout/RefreshToken/ChangePassword 缺少 `context.Context`。**修复：4 个方法全部新增 `ctx context.Context` 首参，Handler 传递 `c.Request.Context()`。**
- ✅ **[2026-06-17]** [service/auth_service.go](/server/internal/service/auth_service.go) — `buildLoginResponse` 中子调用错误用 `fmt.Errorf` 而非 `errcode.AppError`。**修复：9 处 `fmt.Errorf` 全部替换为 `AppError{Code: errcode.ErrUnknown, Message: ...}`，移除 `"fmt"` 导入。**

### 中间件链

- ✅ [middleware/auth.go](/server/internal/middleware/auth.go) — JWT 中间件检查用户冻结（status==2）
- ✅ [middleware/auth.go](/server/internal/middleware/auth.go) — secret 空值检查，拒绝所有请求
- ✅ [middleware/auth.go](/server/internal/middleware/auth.go) — `claims.TokenType != "access"` 校验
- ✅ [middleware/cors.go](/server/internal/middleware/cors.go) — release 模式 DNS 重绑定防护
- ✅ [middleware/request_id.go](/server/internal/middleware/request_id.go) — X-Request-ID 长度限制(128) + 字符集校验
- ✅ [middleware/rbac.go](/server/internal/middleware/rbac.go) — 通配权限 `*` 全匹配 + `prefix:*` 前缀通配
- ✅ [middleware/rbac.go](/server/internal/middleware/rbac.go) — 空 permissions 时 slog.Warn 告警

### 路由与 DTO

- ✅ [router/router.go](/server/internal/router/router.go) — Auth 路由路径 `/api/v1/auth/me` 与文档一致
- ✅ [dto/request/auth.go](/server/internal/dto/request/auth.go) — LogoutRequest DTO（`refresh_token` 字段）
- ✅ **[2026-06-17]** [dto/response/auth.go](/server/internal/dto/response/auth.go) + [service/auth_service.go](/server/internal/service/auth_service.go) — `Permissions []string` nil 值序列化为 JSON `null`。**修复：`buildLoginResponse` 中查询权限后添加 `if permissions == nil { permissions = []string{} }` 守卫。**

---

## 2. 智能问答 RAG

> 对应图：[chat-rag-flow.md](diagrams/chat-rag-flow.md) — SSE 流式 → Pipeline 执行 → 多路检索 → 混合融合 → 重排序 → LLM 生成
> 对应文档：[API/chat.md](API/chat.md)

### RAG 引擎核心

- ✅ [rag/types.go](/server/internal/rag/types.go) — StepMetric 注释 `hybrid_retrieve` → `vector_retrieve / bm25_retrieve / hybrid_fuse`
- ✅ [rag/chunker.go](/server/internal/rag/chunker.go) — `mergeSplits` chunkOverlap 尾部前缀保留
- ✅ [rag/chunker.go](/server/internal/rag/chunker.go) — `normalizeText` 全角→半角 + 换行/空白归一化
- ✅ [rag/document_parser.go](/server/internal/rag/document_parser.go) — PDF `bytes.NewReader(b)` 替代 `string(b)` 转换
- ✅ [rag/document_parser.go](/server/internal/rag/document_parser.go) — DOCX `w:tbl` 表格提取 + tab/br 语义标记
- ✅ [rag/document_parser.go](/server/internal/rag/document_parser.go) — DOCX 命名空间 regex 回退（兼容非标准生成器）
- ✅ [rag/document_parser.go](/server/internal/rag/document_parser.go) — PDF 单页解析失败 `slog.Warn` + 全页失败报错
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — Stop 幂等 `atomic.Bool` + `sync.Once` + recover 防护
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — `ProcessTask.EmbeddingModel` 从 KB 配置读取
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — worker panic recovery `processWithRecovery`
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — 每任务独立 `context.WithTimeout`（10 分钟）
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — 移除重复 `updateStatus("indexing")` 调用
- ✅ [rag/rerank.go](/server/internal/rag/rerank.go) — LLM prompt → cross-encoder 子进程重排序
- ✅ [rag/chunker.go](/server/internal/rag/chunker.go) — **mergeSplits 可产生 1.5× ChunkSize 的分块**：重叠拼接后 `newCurrent = overlapTail + s` 未做大小校验，超限块直接进入 merged 列表。
- ✅ [rag/bm25.go](/server/internal/rag/bm25.go) — **BuildIndex building 标志位 panic 后永不释放**：defer 解锁只保护 mutex，若 buildIndex 因 panic 或 OOM 中断，`building[kbID]=true` 永久阻塞该 KB 的所有后续索引构建。
- ✅ [rag/processor.go](/server/internal/rag/processor.go) — **processTask 不检查 ctx 取消**：`context.WithTimeout` 创建的 ctx 在整个流程（下载/解析/分块/embedding/写入）中从未被检查。超时后 goroutine 仍继续消耗资源直到自然完成或 I/O 边界。
- ✅ [rag/bm25.go](/server/internal/rag/bm25.go) — 文档长度用 rune 计数而非 token 数，中英文长度拉伸不均匀。
- ✅ [rag/bm25.go](/server/internal/rag/bm25.go) — gse 词典加载失败时静默降级返回空结果，调用方无感知。
- ✅ [rag/document_parser.go](/server/internal/rag/document_parser.go) — DOCX 正则回退使用 200 字节启发式检测段落边界，长文本节点可能丢失段落结构。

### RAG 管道

- ✅ [rag/pipeline.go](/server/internal/rag/pipeline.go) — `llmClient != nil` 守卫（nil 时跳过辅助步骤）
- ✅ [rag/pipeline.go](/server/internal/rag/pipeline.go) — `RAGOptions.History` 字段传入会话历史
- ✅ [rag/pipeline.go](/server/internal/rag/pipeline.go) — RerankCount 预截断候选池 + Normalize() 入口
- ✅ [rag/query_rewrite.go](/server/internal/rag/query_rewrite.go) — llm 为 nil 时降级返回原 query
- ✅ [rag/multi_route.go](/server/internal/rag/multi_route.go) — JSON 数组解析（容错 markdown 包裹）+ k 钳位 [2,4]
- ✅ [rag/hybrid.go](/server/internal/rag/hybrid.go) — 单路结果按 topK 截断
- ✅ [rag/bm25.go](/server/internal/rag/bm25.go) — 超量 `recordLargeIndex` warn / building 并发守卫 / LoadDict 错误 / `isValidToken` 过滤 / topK 默认 10
- ✅ [rag/embedder.go](/server/internal/rag/embedder.go) — fail-fast / 维度一致性校验 / nil client 守卫
- ✅ [rag/pipeline.go](/server/internal/rag/pipeline.go) — 多路向量检索结果含重复 ChunkID，纯向量模式或混合融合失败回退时不做去重。
- ✅ [rag/hybrid.go](/server/internal/rag/hybrid.go) — `rrfK` 常量定义但零引用；k 值硬编码在调用方。应删除常量或改用常量替代形参。

### SSE 流式输出

- ✅ [handler/chat.go](/server/internal/handler/chat.go) + [service/llm_service.go](/server/internal/service/llm_service.go) — 流式答案与存储答案一致（单次 LLM 调用）
- ✅ [handler/chat.go](/server/internal/handler/chat.go) — SSE error 事件 `StreamEvent{Type: "error"}`
- ✅ [handler/chat.go](/server/internal/handler/chat.go) — 真实 `ChatCompletionStream` token 级流式
- ✅ [handler/chat.go](/server/internal/handler/chat.go) — SSE 响应头在 Flusher 检查后发送
- ✅ [service/llm_service.go](/server/internal/service/llm_service.go) — RAG 步骤事件 `onStep` callback 实时推送
- ✅ [service/llm_service.go](/server/internal/service/llm_service.go) — 多轮历史滑动窗口截断（`maxHistoryMessages`）
- ✅ [service/llm_service.go](/server/internal/service/llm_service.go) — `maxConfidence` 钳位 [0,1]
- ✅ [service/llm_service.go](/server/internal/service/llm_service.go) — `getModelConfig` 移除 `"default"` 硬编码回退
- 🟢 [service/llm_service.go](/server/internal/service/llm_service.go) — 系统 prompt 硬编码在 `buildMessages`，不支持按知识库定制 AI 角色
- 🟢 [service/llm_service.go](/server/internal/service/llm_service.go) — `SyncChat` 为死代码：`ChatService` 仅调用 `StreamChat`，`SyncChat` 无内部调用方
- 🟢 [service/llm_service.go](/server/internal/service/llm_service.go) — 历史截断按消息条数而非 token 数，长短消息混合时浪费上下文窗口或截断不足

### 聊天服务

- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — `CreateSession` 传播 `ctx context.Context`
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — `GetChatDetail` 校验 `session.UserID` 归属
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — `SubmitFeedback` 校验下沉 Service 层
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — done 持久化失败 `slog.Error` 记录
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) + [config/](/server/internal/config/) — `RAGDefaults` env 配置化
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — Sources 持久化写入
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — pipeline 死存储移除
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — **FindMessagesBySession 错误静默丢弃**：`msgs, _ := s.chatRepo.FindMessagesBySession(...)` 查询失败时 history 为空，多轮对话降级为单轮——无对话上下文导致查询改写失效。
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — **ListSessions N+1**：每个 session 逐一调用 `CountMessagesBySession`，20 个 session = 21 次 DB 查询。
- ✅ [service/chat_service.go](/server/internal/service/chat_service.go) — `json.Unmarshal` 错误在 `GetChatDetail` 中静默丢弃，Sources 列损坏时前端无来源展示且无错误追踪。

### DTO 与 Model

- ✅ [dto/request/chat.go](/server/internal/dto/request/chat.go) — Question `max=2000` + `route_count`/`rerank_count`
- ✅ [dto/response/chat.go](/server/internal/dto/response/chat.go) — `PipelineStep` 类型 + `Pipeline` 字段
- ✅ [model/chat.go](/server/internal/model/chat.go) — `ChatMessage.SessionID` GORM 索引
- ✅ [model/chat.go](/server/internal/model/chat.go) — ChatMessage 增加 `pipeline_metrics JSONB` 字段持久化 RAG 各步骤耗时（ChatSession 重复 TODO 已移除，StreamChat done 事件自动序列化写入）

### 适配层

- ✅ [adapter/vector_store.go](/server/internal/adapter/vector_store.go) — 跨 chunk 维度校验 / CosineSearch 防护 / NaN/Inf `math.IsNaN`/`math.IsInf`
- ✅ [repository/chat_repo.go](/server/internal/repository/chat_repo.go) — `CreateBatch` 调用方修复

### 启动配置

- ✅ [cmd/main.go](/server/cmd/main.go) — BM25 TTL `OPSMIND_AI_BM25_REBUILD_MINUTES` env 配置
- ✅ [cmd/main.go](/server/cmd/main.go) — Processor pool `OPSMIND_AI_PROCESSOR_WORKERS` env 配置

---

## 3. 知识库与文档管理

> 对应图：[knowledge-publish-flow.md](diagrams/knowledge-publish-flow.md) + [document-upload-flow.md](diagrams/document-upload-flow.md) — 文章生命周期 → 分块 → Embedding → pgvector；文档上传 → 解析 → 异步处理
> 对应文档：[API/knowledge.md](API/knowledge.md)

### 知识发布管道

- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — `DeleteByArticle` + `BatchInsert` 非原子。**修复：先 BatchInsert 写入新向量，成功后再 DeleteByArticle 删旧向量。**
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — Publish 使用 `context.Background`。**修复：Publish(ctx, id, publisherID)，Handler 传 c.Request.Context()。**
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — 管道未初始化时映射为 `ErrRAGUnavailable`。
- ✅ **[2026-06-17]** [service/knowledge_service.go:323](/server/internal/service/knowledge_service.go) — 发布失败设置 process_status=failed + process_error。
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — Disable 使用 `context.Background`。**修复：Disable(ctx, id)。**
- ✅ [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — **UploadDocuments 双倍内存分配**：`io.ReadAll`（50MB `[]byte`）后再 `string(data)`（再 50MB），并发上传迅速耗尽内存。已改用 `bytes.NewReader(data)` 避免二次分配。
- ✅ [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — `CountArticlesByKB` 错误静默丢弃，查询失败时所有 KB 文章计数显示为 0。
- ✅ [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — `allowedTypes` map 在每个上传请求中重建，已提取为包级常量 `allowedDocumentTypes`。
- ✅ [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — ProcessTask 回调中 `_ = s.repo.UpdateArticleProcessStatus(...)` 丢弃错误。已提取为 `onProcessStatusChange`/`onProcessMetrics` 方法，内部 slog.Warn 记录失败。
- ✅ [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — 构造函数 8 个位置参数，已重构为 functional options 模式（`WithUserNames`/`WithChunker`/`WithEmbedder`/`WithVectorStore`/`WithDocParser`/`WithProcessor`/`WithStorage`）。

### 文章状态机

- ✅ **[2026-06-17]** [model/enums.go](/server/internal/model/enums.go) vs [API/knowledge.md](API/knowledge.md) — **文章状态编号统一**。
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — Enable 复用 `republishFromApproved` 走完整发布管道。
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — Disable 强校验当前状态。
- ✅ **[2026-06-17]** [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — 文章 `status` 与 `process_status` 状态机解耦。
- ✅ **[2026-06-17]** [repository/knowledge_repo.go](/server/internal/repository/knowledge_repo.go) — `UpdateArticleStatus` 检查 `RowsAffected`。

### 文档上传与异步处理

- ✅ **[2026-06-17]** `ensureBucket` 静默失败 — `NewMinIOClient` 改为返回 error
- ✅ **[2026-06-17]** `io.LimitReader` 静默截断 — 增加上限检测
- ✅ **[2026-06-17]** MIME sniffing 文件类型检测 — `http.DetectContentType` 替代仅信任扩展名
- ✅ **[2026-06-17]** 多文件上传 + 字段名对齐 — `file`→`files`

### 文档上传 API 重构

- ✅ **[2026-06-17]** 删除重复路由 — 统一走 `/knowledge-bases/:kb_id/documents/:id/retry`
- ✅ **[2026-06-17]** kbID 校验下沉 Service
- ✅ **[2026-06-17]** 文档响应 DTO 化 — `DocumentUploadItem`/`DocumentUploadResponse`/`DocumentStatusResponse`
- ✅ **[2026-06-17]** `MaxDocumentSize` 包级常量
- ✅ **[2026-06-17]** `mapProcessStatus` 死代码删除

### KB 管理

- ✅ **[2026-06-17]** KB 删除 API 补齐 — DELETE /knowledge-bases/:id（Handler→Service→Repo 三层）

### 文档响应形状对齐

- ✅ **[2026-06-17]** KB 响应填充 `llm_config_id`/`article_count`
- ✅ **[2026-06-17]** `CreateKB` 使用 `req.LlmConfigID`
- ✅ **[2026-06-17]** Article 响应填充 `source_type_text`/`created_by_name`/`published_by_name`
- ✅ `SendMessageRequest` 已有 `route_count`/`rerank_count` 字段

### 内容质量

- ✅ `mergeSplits` 已实现 chunkOverlap
- ✅ `Split()` 入口已调用 `normalizeText`
- ✅ DOCX 表格 `w:tbl` 提取已实现
- ✅ **[2026-06-17]** tags trim/去重/限制数量
- ✅ 门户端 `ListKBsForPortal` 已过滤内部字段

### 代码 TODO

- ✅ **[2026-06-17]** `UpdateArticle` userID 参数已移除
- ✅ **[2026-06-17]** source_type/process_status 筛选已实现
- ✅ 详情响应已使用 `process_status`/`process_error`
- ✅ **[2026-06-17]** 枚举 Text 方法已统一（ArticleStatusText/ArticleSourceTypeText/TicketUrgencyText/TicketImpactText/ProcessStatusText）
- ✅ **[2026-06-17]** ChunkResponse.CreatedAt 已填充

### RAG 重排序

- ✅ [rag/rerank.go](/server/internal/rag/rerank.go) — LLM prompt 方案 → cross-encoder 子进程重排序
- ✅ [rag/rerank.go](/server/internal/rag/rerank.go) — `_ = i` 调试残留删除

---

## 4. 申告管理

> 对应图：[ticket-lifecycle.md](diagrams/ticket-lifecycle.md) + [ticket-state-machine.md](diagrams/ticket-state-machine.md) — 创建→处理→补充→解决→关闭 + AutoClose
> 对应文档：[API/tickets.md](API/tickets.md)

### 数据完整性与事务

- ✅ [service/ticket_service.go](/server/internal/service/ticket_service.go) — `supplement_count` 并发竞态：应在 Repository 层用 `UPDATE ... WHERE supplement_count < 3` 原子操作
- ✅ [service/ticket_service.go](/server/internal/service/ticket_service.go) — `UpdateStatus` + `CreateRecord` 不在同一事务
- ✅ [repository/ticket_repo.go](/server/internal/repository/ticket_repo.go) — AutoClose 批量 UPDATE 不创建 TicketRecord
- ✅ [repository/ticket_repo.go](/server/internal/repository/ticket_repo.go) — `SELECT ids + UPDATE` 不是原子操作，AutoClose 有 TOCTOU 竞态
- ✅ [repository/ticket_repo.go](/server/internal/repository/ticket_repo.go) — `UpdateStatus` 应返回 `RowsAffected`
- ✅ [service/ticket_service.go](/server/internal/service/ticket_service.go) — **SupplementTicket 不在事务内**：`CreateRecord` 成功后 `UpdateStatus` 失败时，时间线记录已写入但工单状态未更新，产生孤立记录。

### 状态机

- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — 状态机和 action 使用裸数字（1,2,3,4,5）而非常量 `TicketStatusXxx`~~ → 已替换为 `model.TicketStatusXxx` / `model.TicketActionXxx`
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — **UpdateStatus 无 CAS 条件**~~ → 已改为 `WHERE id=? AND status=?` CAS 式更新
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — close 操作是否允许关闭已解决（Resolved）状态~~ → 已明确：禁止关闭已关闭/已解决的申告
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — `request_info` 后应同步创建站内消息通知~~ → 已注入 MessageService，同步调用 NotifySupplement

### 安全与校验

- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — `ticket_no` 碰撞风险~~ → 改用 crypto/rand 生成 6 位真随机数
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — 门户端 GetDetail 越权~~ → GetDetail(id, userID)，门户传 userID 校验所有权，后台传 0 跳过
- 🟢 [model/ticket.go](/server/internal/model/ticket.go) — `contact_phone` 长度假设 11 位中国手机号
- 🟢 [dto/request/ticket.go](/server/internal/dto/request/ticket.go) — `ChatContext` 应使用结构化对象而非 `string`
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — **ChatContext 无 JSON 校验**~~ → isValidJSON() 前置校验，非法 JSON 返回明确错误
- ✅ ~~[service/ticket_service.go](/server/internal/service/ticket_service.go) — **AddRecord.Detail 无校验 + action 无白名单**~~ → isValidJSON() + validRecordActions 白名单

### 文档一致性

- ✅ ~~[API/tickets.md](API/tickets.md) vs [model/enums.go](/server/internal/model/enums.go) — **影响范围取值不一致**~~ → 已修正为 1=个人, 2=部门, 3=全公司，示例值也修复

### 代码 TODO（申告服务）

- ✅ ~~[handler/ticket.go:60](/server/internal/handler/ticket.go) — ListByUser should reuse parsePagination~~ → ListByUser + ListAll 均改用 parsePagination()
- ✅ ~~[handler/ticket.go:138](/server/internal/handler/ticket.go) — GetDetail 未区分权限范围~~ → 已实现：门户端传 userID，后台传 0
- ✅ ~~[handler/ticket.go:217](/server/internal/handler/ticket.go) — 跨 Handler 直接调用 KnowledgeService~~ → 已移至 TicketService.CreateKnowledgeCandidate，Handler 仅转发调用
- ✅ ~~[service/ticket_service.go:86](/server/internal/service/ticket_service.go) — 校验 ChatContext 是合法 JSON~~ → isValidJSON() 已实现
- ✅ ~~[service/ticket_service.go:267](/server/internal/service/ticket_service.go) — Detail JSON 校验 + action 白名单~~ → 已实现
- ✅ ~~[dto/request/ticket.go:42](/server/internal/dto/request/ticket.go) — action 应使用 binding oneof~~ → 已添加 `binding:"required,oneof=start request_info resolve close"`

### 代码 TODO（消息服务）

- ✅ ~~[service/message_service.go:42](/server/internal/service/message_service.go) — 消息文案应包含 ticket_title~~ → NotifySupplement 增加 ticketTitle 参数，消息包含「申告标题」
- 📌 [service/message_service.go:95](/server/internal/service/message_service.go) — 未读数适合缓存或通过 WebSocket/SSE 推送（架构级优化，暂保留 TODO）
- ✅ ~~[service/message_service.go](/server/internal/service/message_service.go) — **NotifySupplement 死代码**~~ → TicketService 已注入 MessageService，request_info 后同步调用 NotifySupplement
- ✅ ~~[service/message_service.go](/server/internal/service/message_service.go) — MarkAsRead 未校验 userID > 0~~ → 已添加 `if userID <= 0` 守卫

### Repository

- ✅ ~~[repository/ticket_repo.go:112](/server/internal/repository/ticket_repo.go) — ListAll 二次查询失败静默忽略~~ → 改为 return error
- ✅ ~~[repository/message_repo.go:33](/server/internal/repository/message_repo.go) — 增加 is_read/type 过滤~~ → MessageFilter 结构体 + Handler 解析 query params

### Model

- ✅ ~~[model/ticket.go:40](/server/internal/model/ticket.go) — OperatorID=0 conflicts with FK~~ → 注释修正：确认无 FK 约束，0=系统自动操作

---

## 5. 用户与角色管理

> 对应图：[user-rbac-flow.md](diagrams/user-rbac-flow.md) — 用户 CRUD + 角色权限 + 菜单树
> 对应文档：[API/users.md](API/users.md) + [API/roles.md](API/roles.md)

### 数据安全

- 🔴⭐ [repository/role_repo.go](/server/internal/repository/role_repo.go) — **GORM 零值陷阱**：`Delete(&Role{}, 0)` 删除全部角色。需加 `if id <= 0` 守卫或改用 `Where("id = ?", id).Delete()`
- 🔴 [service/user_service.go](/server/internal/service/user_service.go) — `AssignRoles` 内层再开事务（嵌套事务风险：外层事务回滚不包含 AssignRoles 的变更）
- 🟡⭐ [service/user_service.go](/server/internal/service/user_service.go) — **丢失更新竞态**：`Update` 用 `GetByID` → 修改内存 → `Save`，并发 `ChangePassword` 的密码哈希会被 `Save` 覆盖
- 🟡⭐ [service/role_service.go](/server/internal/service/role_service.go) — **TOCTOU 竞态**：`Delete` 先 `CountUsersByRole` 检查再 `Delete`，并发 `AssignRoles` 可在检查后分配用户到此角色
- 🟡⭐ [repository/user_repo.go](/server/internal/repository/user_repo.go) — **TOCTOU 竞态**：`UpdateRoleMenus` 删除-插入非原子，并发更新同角色菜单可致部分关联永久丢失
- 🟡 [service/role_service.go](/server/internal/service/role_service.go) — 禁止删除系统内置角色（当前无 built-in 标记，无删除保护）
- 🟡 [service/user_service.go](/server/internal/service/user_service.go) — 无「最后一个管理员」保护：冻结/修改最后的管理员角色会使系统无法管理
- 🟡⭐ [service/user_service.go](/server/internal/service/user_service.go) — 无自我保护：用户可以冻结自己、降级自己的角色权限
- 🟡 [repository/user_repo.go](/server/internal/repository/user_repo.go) — `AssignRoles` 不校验 roleID 是否存在、不排重、不过滤 ≤0 的非法值

### 查询性能

- 🟡 [service/user_service.go](/server/internal/service/user_service.go) — 列表 N+1 查询角色：每个用户额外一次 DB 查询获取角色名
- 🟢 [repository/user_repo.go](/server/internal/repository/user_repo.go) — `ExistsBy*` 用 `COUNT(*)` 而非 `SELECT 1 LIMIT 1`，大表效率低
- 🟢⭐ [service/user_service.go](/server/internal/service/user_service.go) — Freeze/Restore 用魔法数字 `1`/`2` 而非 `model.StatusActive`/`model.StatusInactive`。

### 权限与角色

- 🔴 [service/role_service.go](/server/internal/service/role_service.go) — `.Count()` 错误被丢弃，绕过 Repository 直调 DB（分层违规）
- 🟡 [service/role_service.go](/server/internal/service/role_service.go) — 角色权限无白名单校验：任意字符串可作为权限写入 JSONB
- 🟡 [repository/role_repo.go](/server/internal/repository/role_repo.go) — `Delete` 不检查 `RowsAffected`
- 🟡 [repository/user_repo.go](/server/internal/repository/user_repo.go) — 角色权限 JSON 解析失败静默跳过 `continue`，数据损坏被掩盖
- 🟡⭐ [service/role_service.go](/server/internal/service/role_service.go) — **`db *gorm.DB` 字段注入但从未使用**：死依赖。应删除字段和构造函数参数。
- 🟡⭐ [service/role_service.go](/server/internal/service/role_service.go) — 菜单操作通过 `UserRepo` 委托而非 `MenuRepo`，层职责不清。`RoleService.ListMenus()`/`UpdateRoleMenus()` 跨层调用 `s.userRepo`。

### 输入校验与 Model

- 🟢 [service/user_service.go](/server/internal/service/user_service.go) — 缺少对 phone/email/realName 的格式/trim 校验
- 🟢 [model/user.go](/server/internal/model/user.go) — phone 字段缺少唯一索引
- 🟢 [model/user.go](/server/internal/model/user.go) — Role 缺少 `is_system` 不可变标记字段

### 代码 TODO

- 📌 [handler/role.go:63](/server/internal/handler/role.go) — Role list should support keyword search
- 📌 [handler/user.go:51](/server/internal/handler/user.go) — 复用 parseID，减少重复错误信息
- 📌 [handler/user.go:71](/server/internal/handler/user.go) — 复用 parsePagination
- 📌 [service/role_service.go:161](/server/internal/service/role_service.go) — 校验 menuIDs 是否全部存在，且按钮权限不能挂到错误父级

---

## 6. LLM 配置与适配层

> 对应图：[llm-config-flow.md](diagrams/llm-config-flow.md) — CRUD + TestConnection + atomic.Value 热替换 + API Key 脱敏
> 对应文档：[API/llm-config.md](API/llm-config.md)

### LLM 客户端

- 🔴⭐ [adapter/llm_client.go](/server/internal/adapter/llm_client.go) — **重试机制完全失效**：`doHTTPRequest` 将 429/503 包装为普通 `fmt.Errorf` 而非 `*retryableError`，`isRetryable()` 永远返回 false。LLM 同步重试和 Embedding 重试均不工作。
- 🔴 [adapter/llm_client.go](/server/internal/adapter/llm_client.go) — 校验 baseURL 非空且合法（空字符串产生误导性 "unsupported protocol scheme" 错误）
- 🔴 [adapter/llm_client.go](/server/internal/adapter/llm_client.go) — `req.Model` 为空时应返回参数错误（而非将空字符串发给 API）
- 🔴 [adapter/llm_client.go](/server/internal/adapter/llm_client.go) — `bufio.Scanner` 默认 64KB 上限，大 SSE `data:` 行会触发 `ErrTooLong` 静默截断流
- 🔴 [adapter/llm_client.go](/server/internal/adapter/llm_client.go) — 流式请求无 429/503 重试（直接 return status code error）

### LLM 配置管理

- 🔴⭐ [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — **事务内使用 repo 原始 DB 而非 tx**：`ClearDefault()` 和 `Create()` 在 `db.Transaction()` 内调但用 `s.repo` 的 `*gorm.DB`，操作在事务外执行，原子性为空壳。`UpdateConfig` 中同样存在。
- 🔴⭐ [handler/llm_config.go](/server/internal/handler/llm_config.go) — **TestConnection 测试的是全局默认客户端而非被测配置**：始终用 `h.llmClient`，选择 `:id` 配置无意义——功能完全失效。📝 [API/llm-config.md](API/llm-config.md) 记载测试「指定配置的连接」，但代码实现与之完全不符。
- 🔴⭐ [handler/llm_config.go](/server/internal/handler/llm_config.go) — **UpdateConfig 会清空 api_key**：若请求不传 `api_key`，零值 `""` 覆盖数据库中已存储的密钥。📝 [API/llm-config.md](API/llm-config.md) 记载「api_key 不传时保留原有密钥（不传 ≠ 清空）」，代码行为与文档相反。
- 🔴 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — `atomic.Store` 直接存指针，未复制 cfg；调用方修改原对象 → 并发读看到中间态（数据竞态）
- 🔴 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — 构造函数不应 panic：mock 类型不匹配会直接崩溃进程，应返回 error
- 🔴⭐ [model/llm_config.go](/server/internal/model/llm_config.go) — **API Key 明文存储**：数据库泄露 = 所有 LLM 提供商密钥暴露。需 AES-256 加密存储。📝 [API/llm-config.md](API/llm-config.md) 记载「api_key 在数据库中以 AES-256 加密存储」，但代码完全未实现加密。
- 🔴⭐ [repository/llm_config_repo.go](/server/internal/repository/llm_config_repo.go) — **缺少 `is_default` 部分唯一索引**：并发创建默认配置可产生多条 `is_default=true` 记录
- 🟡 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — 默认配置切换后未重建 LLM/Embedding 客户端（仅替换了配置值，已初始化的 HTTP 客户端仍指向旧 Base URL）
- 🟡 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — 缺少输入校验：providerType 白名单、baseURL 格式、maxTokens/vectorDimension 范围
- 🟡 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — 删除配置前未检查 FK 引用（knowledge_bases.llm_config_id）
- 🟡 [handler/llm_config.go](/server/internal/handler/llm_config.go) — CreateConfig 默认值 8192/1024 应在 Service 层而非 Handler 层
- 📝⭐ [handler/llm_config.go:216](/server/internal/handler/llm_config.go) — **TestConnection 错误码与 API 文档不一致**：[API/llm-config.md](API/llm-config.md) 记载失败时 `code=0, data.success=false`，代码返回 `code=20001`（ErrAIUnavailable）。

### 适配层通用

- 🔴⭐ [adapter/vector_store.go](/server/internal/adapter/vector_store.go) — **双数据库连接池**：PgvectorStore 创建独立的 `sql.DB`，与 GORM 连接池并存且指向同一 PostgreSQL，浪费连接资源
- 🔴⭐ [adapter/vector_store.go](/server/internal/adapter/vector_store.go) — **PgvectorStore 不暴露 Close()**：独立连接池无法在优雅关闭时清理，连接泄漏。
- 🟡 [adapter/vector_store.go](/server/internal/adapter/vector_store.go) — `fmt.Sprintf("%.6f")` 截断 float32 精度，叠加 halfvec 量化进一步损失召回率
- 🟡 [adapter/storage_client.go](/server/internal/adapter/storage_client.go) — 上传 key 应由上层 helper 统一生成（当前分散在调用方拼接）
- 📌 [adapter/llm_client.go:409](/server/internal/adapter/llm_client.go) — `doHTTPRequest` 将 429/503 包装为 `fmt.Errorf` 而非 `retryableError`，导致 Embedding 客户端的 `isRetryable()` 永远返回 false。与 `tryRequest` 的重试处理不一致。

---

## 7. 数据看板与审计

> 对应图：[dashboard-audit-flow.md](diagrams/dashboard-audit-flow.md) — 7 项聚合统计 + 趋势图 + 审计日志查询
> 对应文档：[API/dashboard.md](API/dashboard.md) + [API/audit-log.md](API/audit-log.md)

### 看板统计

- 🔴 [service/dashboard_service.go](/server/internal/service/dashboard_service.go) — `.Scan()` 错误被丢弃：聚合查询失败时静默返回零值，掩盖数据库故障
- 🟡 [router/admin.go](/server/internal/router/admin.go) — Dashboard 路由使用 `audit:read` 权限控制，应有独立的 `dashboard:read` 权限
- 🟡⭐ [service/dashboard_service.go](/server/internal/service/dashboard_service.go) — `GetTrends` 无日期上限限制，跨年查询生成海量日期序列 + 大响应体。
- 🟢⭐ [service/dashboard_service.go](/server/internal/service/dashboard_service.go) — GetStats 中 7 项统计查询全串行，首屏看板延迟为所有 SQL 延迟之和。
- 📌 [handler/dashboard.go:44](/server/internal/handler/dashboard.go) — granularity 参数已在前端/API 类型中出现，但 Service 当前忽略。Handler TODO 确认后端未实现。

### 看板代码 TODO

- 📌 [service/dashboard_service.go:33](/server/internal/service/dashboard_service.go) — 统计查询串行执行，首屏看板延迟等于所有 SQL 延迟之和
- 📌 [service/dashboard_service.go:88](/server/internal/service/dashboard_service.go) — 校验 endDate >= startDate 且范围上限（如 90 天）
- 📌 [service/dashboard_service.go:117](/server/internal/service/dashboard_service.go) — 双重循环填充趋势数据是 O(days \* rows)
- 📌 [repository/dashboard_repo.go:23](/server/internal/repository/dashboard_repo.go) — created_at::date 会让索引失效
- 📌 [repository/dashboard_repo.go:66](/server/internal/repository/dashboard_repo.go) — 趋势 SQL 固定按日聚合，未支持 week granularity

### 审计日志 — P0 审计写入缺失（零调用方）

`AuditRepo.Create` 存在但零调用方，以下敏感操作全部无审计记录：

- 🔴 [service/user_service.go](/server/internal/service/user_service.go) — `Create`/`Update`/`Freeze`/`Restore` 无审计写入
- 🔴 [service/role_service.go](/server/internal/service/role_service.go) — `Create`/`Update`/`Delete` 无审计写入
- 🔴 [service/knowledge_service.go](/server/internal/service/knowledge_service.go) — `Publish`/`Disable` 无审计写入
- 🔴 [service/ticket_service.go](/server/internal/service/ticket_service.go) — `UpdateStatus` 无审计写入
- 🔴 [service/config_service.go](/server/internal/service/config_service.go) — `UpdateConfig` 无审计写入
- 🔴 [service/llm_config_service.go](/server/internal/service/llm_config_service.go) — `UpdateConfig` 无审计写入
- 🔴 [service/scheduler.go](/server/internal/service/scheduler.go) — `AutoClose` 无审计写入（需 operatorID=0 表示系统操作）

> 修复方式：各 Service 注入 `*AuditRepo`，在关键操作的事务内同步写入审计记录。
> 为什么同步而非异步：MVP 阶段审计写入是轻量 INSERT，同步执行保证事务一致性（CLAUDE.md §4 明确要求）。

### 审计日志 — P0 查询能力不足

- 🔴 [repository/audit_repo.go](/server/internal/repository/audit_repo.go) — `List()` 无日期范围过滤：全表扫描 `created_at` 无时间边界，大时间跨度查询可能超时
- 🔴 [dto/request/audit.go](/server/internal/dto/request/audit.go) — 缺少 `date_from`/`date_to` 参数
- 🟡 [repository/audit_repo.go](/server/internal/repository/audit_repo.go) — 查询维度不足：不支持 `target_type`/`target_id` 筛选，无法按资源维度检索（"谁改过这个申告？"）

### 审计日志 — P1 查询性能与数据质量

- 🟡 [service/audit_service.go](/server/internal/service/audit_service.go) — N+1 查询模式：先查 `audit_logs`，再批量查 `users`（两条 SQL），应改为单条 LEFT JOIN
- 🟡 [service/audit_service.go](/server/internal/service/audit_service.go) — `operatorID=0`（系统操作）未映射为"系统"显示名，前端展示空字符串
- 🟡 [dto/request/audit.go](/server/internal/dto/request/audit.go) — `action` 仅支持精确匹配，不支持前缀/模糊搜索（如 `user.*` 查看所有用户操作）
- 🟡 [service/audit_service.go](/server/internal/service/audit_service.go) — `batchGetOperatorNames` 中 `userRepo.FindByIDs` 失败时静默返回空 map，丢失错误信息

### 审计日志 — P2 测试与文档

- 🟡 [tests/repository/audit_repo_test.go](/server/tests/repository/audit_repo_test.go) — 测试使用 `init()` 直接 panic + 硬编码数据库凭据，不符合其他测试模块的标准模式
- 🟡 缺少 Service 层集成测试（验证各 Service 的审计写入正确性）
- 📝 [API/audit-log.md](/docs/API/audit-log.md) — action 分隔符不一致：文档用 `:`（`user:create`），代码用 `.`（`user.create`）
- 📝 [API/audit-log.md](/docs/API/audit-log.md) — 缺少新增查询参数（`target_type`/`target_id`/`date_from`/`date_to`）
- 📝 [diagrams/dashboard-audit-flow.md](/docs/diagrams/dashboard-audit-flow.md) — 审计查询流程图显示 JOIN 查询，但代码实际用两条 SQL + Go 层拼接

### 审计 Repo 代码 TODO

- 📌 [repository/audit_repo.go:33](/server/internal/repository/audit_repo.go) — 审计写入失败是否阻断主流程需要统一策略

---

## 8. 基础设施与部署

> 对应图：[architecture.md](diagrams/architecture.md) + [request-lifecycle.md](diagrams/request-lifecycle.md) — 启动流程 → 中间件链 → 路由 → 数据库

### 启动流程（main.go）

- 🔴 [cmd/main.go](/server/cmd/main.go) — 初始化流程需拆成 `wireApp()`/`runServer()` 独立函数
- 🔴 [cmd/main.go](/server/cmd/main.go) — 数据库连接池参数应配置化（`MaxOpenConns=25` 等硬编码）
- 🔴 [cmd/main.go](/server/cmd/main.go) — AutoMigrate 不适合生产环境：自动建表/加列可能锁表或破坏数据
- 🔴 [cmd/main.go](/server/cmd/main.go) — LLM/Embedding 超时应区分场景配置化（查询改写 vs 生成 vs embedding 超时需求不同）
- 🔴 [cmd/main.go](/server/cmd/main.go) — VectorStore 初始化失败应返回健康状态（而非仅 warn）
- 🔴 [cmd/main.go](/server/cmd/main.go) — `ReadTimeout`/`WriteTimeout` 应配置化：SSE 路由需更长的 WriteTimeout
- 🔴⭐ [cmd/main.go](/server/cmd/main.go) — **nil 传播**：pgvector/MinIO 初始化失败后 `vectorStore`/`storageClient` 为 nil 但仍传给下游服务，后续调用 panic
- 🔴⭐ [cmd/main.go](/server/cmd/main.go) — **pgDSN 同样存在密码拼接 Bug**：同 database.go 的 DSN 特殊字符问题。
- 🔴⭐ [cmd/main.go](/server/cmd/main.go) — **ListenAndServe 失败调用 `os.Exit(1)` 跳过所有 defer**：cancel/reranker.Close/srv.Shutdown 全部不执行。

### 配置管理

- 🔴⭐ [config/config.go](/server/internal/config/config.go) — **配置错误静默吞掉**：非 `ConfigFileNotFoundError` 的 `ReadInConfig` 错误（如 YAML 格式错误）被丢弃，应用以默认值启动
- 🟡 [config/config.go](/server/internal/config/config.go) — 增加 `Validate()` 统一校验：mode/port 有效性、JWT 非空、AI 阈值范围
- 🟡 [config/config.go](/server/internal/config/config.go) — 日志脱敏 password/api_key/secret：`config.Dump()` 或日志输出前应掩码
- 🟡 [config/config.go](/server/internal/config/config.go) — `BindEnv` 24 处返回值全部忽略
- 🟢 [config/config.go](/server/internal/config/config.go) — `time.Duration` 解析：`OPSMIND_JWT_ACCESS_EXPIRE=3600`（裸数字）会导致解析失败
- 📌 [service/config_service.go:37](/server/internal/service/config_service.go) — Config key whitelist and type definitions needed
- 📌 [service/config_service.go:62](/server/internal/service/config_service.go) — 更新 ai 配置项未同步到运行时
- 📌 [handler/config.go:48](/server/internal/handler/config.go) — binding:"required" 会让 false、0 等合法配置值被判定为缺失

### 数据库

- 🔴 [database/database.go](/server/internal/database/database.go) — DSN 密码直接 `fmt.Sprintf` 拼接：特殊字符（空格、`'`、`\`）导致连接失败
- 🔴 [database/database.go](/server/internal/database/database.go) — 生产环境打印 SQL 可能泄露业务数据和 PII
- 🔴 [database/database.go](/server/internal/database/database.go) — 启动时应 `PingContext` 超时校验：当前完全不 Ping，DB 不可达只在首次查询时暴露
- 🟡 [database/migrate.go](/server/internal/database/migrate.go) — AutoMigrate 不启用 pgvector 扩展，HNSW 索引需手动创建
- 🔴⭐ [database/migrate.go](/server/internal/database/migrate.go) — **ASC+DESC 双重索引**：GORM AutoMigrate 创建 `created_at` ASC 索引，migrate.go 又创建 DESC 索引。且 `IF NOT EXISTS` 用同名 → DESC 索引永远不创建（ASC 索引已占同名）。
- 🔴⭐ [database/migrate.go](/server/internal/database/migrate.go) — **DESC 索引创建 Bug**：`CREATE INDEX IF NOT EXISTS idx_xxx_created_at` 在 GORM 已创建同名 ASC 索引后是 no-op。注释说"重建 DESC 索引"但实际永远不生效。

### 路由与中间件

- 🟡 [router/router.go](/server/internal/router/router.go) — placeholder 路由生产环境应 fail fast（而非返回 501 掩盖配置错误）
- 🟡 [router/router.go](/server/internal/router/router.go) — 增加 `/readyz` 健康探针（含 DB/VectorStore/MinIO/LLM 可达性检查）
- 🟡 [router/admin.go](/server/internal/router/admin.go) — 权限字符串散落各处，建议集中为常量文件
- 🟡 [router/portal.go](/server/internal/router/portal.go) — 门户路由无角色校验：仅需 JWT，不验证用户是否为报障人角色
- 🟡 [middleware/logger.go](/server/internal/middleware/logger.go) — `json.Marshal` 错误被丢弃
- 🟢 [middleware/logger.go](/server/internal/middleware/logger.go) — request-ID/userID 已记录，缺少**业务错误码**写入日志行（当前仅 HTTP status，无法关联 errcode 10001/20001 等）
- ✅ [router/admin.go](/server/internal/router/admin.go) — 文档上传路由缩进已修复

### Repository 层

- 🔴 [repository/pagination.go](/server/internal/repository/pagination.go) — 所有 Repo 方法缺 `context.Context`：HTTP 取消不传播到 DB 查询，追踪无法关联
- 🟡 [repository/pagination.go](/server/internal/repository/pagination.go) — 分页辅助函数零调用方（死代码），各 Repo 自行实现分页
- 🟡 [repository/knowledge_repo.go](/server/internal/repository/knowledge_repo.go) — GORM query 对象复用于 Count 和 Find，session 状态可能泄漏
- 📌 [repository/knowledge_repo.go:34](/server/internal/repository/knowledge_repo.go) — 创建知识库应依赖数据库唯一索引兜底
- ✅ [repository/knowledge_repo.go:117](/server/internal/repository/knowledge_repo.go) — Count/Offset/Limit 复用同一 query 的状态泄漏风险已在 `ListArticles` 重构时消除（每个 WHERE 条件独立添加，GORM clone 了 session）
- 📌 [repository/config_repo.go:46](/server/internal/repository/config_repo.go) — Upsert 会覆盖 description 之外的配置元信息
- 📌 [repository/chat_repo.go:37](/server/internal/repository/chat_repo.go) — FindByID 应支持 userID 条件，用于门户端防止水平越权

### 调度器

- 🟢 [service/scheduler.go](/server/internal/service/scheduler.go) — `Start` 应防重复调用（当前无幂等保护）
- 🟢⭐ [service/scheduler.go](/server/internal/service/scheduler.go) — **调度器首次启动不立即执行 AutoClose**：必须等待首个完整 cron 周期，频繁重启时超期工单可能长时间未关闭
- 🟡⭐ [service/scheduler.go](/server/internal/service/scheduler.go) — `Start` 重复调用覆盖 `cancel` 导致第一组 goroutine 泄漏。

### 事务管理器

- 📌 [service/tx_manager.go:18](/server/internal/service/tx_manager.go) — 校验 db 非 nil，构造期提前暴露装配错误
- 📌 [service/tx_manager.go:24](/server/internal/service/tx_manager.go) — Transaction 可以接收 context.Context 并使用 db.WithContext(ctx)
- 🟡⭐ [service/tx_manager.go](/server/internal/service/tx_manager.go) — **TxManager 接口将 `*gorm.DB` 泄露到 Service 层**：回调签名 `func(tx *gorm.DB) error` 使服务层直接依赖 GORM 具体类型，违反三层抽象。

### 日志与错误

- 🟡 [pkg/response/response.go](/server/pkg/response/response.go) — 错误响应缺少 `request_id`，前端报错难以和服务端日志关联
- 🟡 [pkg/response/response.go](/server/pkg/response/response.go) — 分页响应格式不统一（顶层 `total/page/page_size` vs 部分前端类型期望 `data.items/data.total`）
- 🟢 [middleware/logger.go](/server/internal/middleware/logger.go) — 部分 Handler 未使用 `handleServiceError` 封装（分散在 `auth.go` 而非 `common.go`）
- 🔴⭐ [pkg/response/response.go](/server/pkg/response/response.go) — **ErrAlreadyFrozen(10006)/ErrAlreadyActive(10007) 映射到 HTTP 500 而非 400**：`mapHTTPStatus` switch 中漏掉这两个 errcode，fallthrough 到 default→500。📝 TECH.md 文档记载为 HTTP 400。

### Handler 通用工具

- 📌 [handler/common.go:24](/server/internal/handler/common.go) — page_size max 应可配置
- 📌 [handler/common.go:58](/server/internal/handler/common.go) — `getCurrentUserID` 的 `exists` 返回值被所有 12 个调用方忽略（`userID, _ := ...`），JWT 中间件漏配时 userID=0 静默传入 Service 层。应提供 `mustCurrentUserID()` 变体在缺失时直接返回 401。
- 🟡⭐ [handler/common.go](/server/internal/handler/common.go) — **12 个 `getCurrentUserID` 调用方全部忽略 `exists` 布尔值**：JWT 中间件漏配时 userID=0 静默传入 Service 层，创建归属 user 0 的数据。
- 🟡⭐ [handler/chat.go](/server/internal/handler/chat.go) — `SubmitFeedback` 使用内联匿名 struct 定义请求体，应改为命名 DTO。
- 🟡⭐ [handler/role.go](/server/internal/handler/role.go) + [handler/user.go](/server/internal/handler/user.go) — 手动实现分页/ID 解析而非复用 `parsePagination`/`parseID` 公共 helper。

### Model 层

- 📌 [model/common.go:8](/server/internal/model/common.go) — 分页 Scope 与 repository.Paginate 重复，且没有 pageSize 上限
- 📌 [model/system.go:14](/server/internal/model/system.go) — 配置表缺少 value_type、editable、validation_schema
- 🟡⭐ [model/enums.go](/server/internal/model/enums.go) — **4 组死枚举常量**：`EmbeddingTypeAPI/EmbeddingTypeLocal`、`ChatRoleUser/ChatRoleAssistant`、`ChatFeedbackUnset/Resolved/Unresolved`、`MenuTypeMenu/MenuTypeButton`——零外部引用。应删除或替换内联字面量。
- 🟡⭐ [model/enums.go](/server/internal/model/enums.go) — `ArticleSourceTypeText` 用魔法数字 `1`/`2` 而非命名常量 `SourceTypeManual`/`SourceTypeUpload`。
- 🟡⭐ [model/chat.go](/server/internal/model/chat.go) — 连续两行相同 TODO 注释（`pipeline_metrics JSONB` 重复）。
- 🟢⭐ [model/knowledge.go](/server/internal/model/knowledge.go) — `LlmConfigID` 无 FK 约束，删除 LLM 配置后知识库产生悬空引用。

### 文档一致性

- 📝⭐ docs 目录引用 `docs/v2/` 和 `server/migrations/v2/`，但两个目录均不存在，迁移脚本缺失。
- 📝⭐ [dto/response/knowledge.go](/server/internal/dto/response/knowledge.go) vs [dto/response/user.go](/server/internal/dto/response/user.go) — 时间戳格式不一致：ArticleResponse 用 `time.Time`（RFC3339），UserDetailResponse 用 `string`（自定义格式）。前端需两套时间解析策略。
- ✅ **[2026-06-17]** [dto/response/auth.go](/server/internal/dto/response/auth.go) — `Permissions []string` nil 问题已修复，与其他列表字段行为一致。

---

## 9. 前端架构与交互

> 对应全部业务流程图的前端部分（Vue 3 + TypeScript）

### 认证与安全

- 🔴⭐ [router/index.ts](/web/src/router/index.ts) — **JWT 过期检查 Bug**：`atob` 不兼容 base64url 编码（`-`/`_` 替代 `+`/`/`）。某些 JWT payload 会解码失败返回 null，导致过期检查失效——过期 token 被当作有效。
- 🔴⭐ [api/chat.ts](/web/src/api/chat.ts) — **SSE 流绕过 Axios 拦截器**：Token 刷新（401 响应）对流式请求完全失效。若 token 在流中途过期，连接断开且无恢复。
- 🔴⭐ [utils/request.ts](/web/src/utils/request.ts) — **Token 刷新竞态**：刷新失败时 `refreshSubscribers` 重置为 `[]` 但未通知已订阅者，其 Promise 永久挂起（内存泄漏）。
- 🟡 [views/auth/Login.vue](/web/src/views/auth/Login.vue) — 错误信息提取不完整：`catch` 用 `err?.message`（Axios 通用字符串），后端真实错误在 `err.response?.data?.message`
- 🟡⭐ [views/auth/Login.vue](/web/src/views/auth/Login.vue) — 路由判断基于 `permissions.length > 0`，若 admin 返回空权限数组会误导向 portal。

### P0 缺陷（2026-06-17 审计新发现）

- 🔴⭐ [stores/chat.ts](/web/src/stores/chat.ts) — `crypto.randomUUID()` 无 fallback：HTTP/localhost 环境下 `crypto.randomUUID()` 为 undefined，调用直接抛 TypeError 崩溃整个聊天功能。已有 `generateId()` 工具函数（`utils/__tests__/id.test.ts`）但未使用。
- 🔴⭐ [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — 操作按钮缺 loading 守卫：`doAction()` 和 `doAddRecord()` 未绑定 `:disabled` 到 `<template>` 按钮，用户可多次点击发送重复请求。
- 🔴⭐ [App.vue](/web/src/App.vue) — `NMessageProvider` 死代码：全项目无组件使用 Naive UI `useMessage()`（统一使用自定义 `useToast()`），每次渲染浪费不必要的组件树开销。
- 🔴⭐ [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — **创建配置时测试连接崩溃**：`handleTestConnection` 调 `updateLLMConfig(editingId.value!)`，新建时 `editingId` 为 null，`!` 断言导致运行时崩溃
- 🔴⭐ **[2026-06-17]** [views/portal/TicketSubmit.vue](/web/src/views/portal/TicketSubmit.vue) — **submit 成功后 `submitting` 永不重置**：`submitSuccess=true` 但 `submitting` 保留 true，提交按钮永久显示"提交中..."。用户必须离开页面才能恢复。
- 🔴⭐ **[2026-06-17]** [views/admin/TicketList.vue](/web/src/views/admin/TicketList.vue) — **响应解包 Bug**：`tickets.value = res?.data || res?.items` —— `res.data` 是 `PageResponse` 对象（非数组），赋给期望 `TicketItem[]` 的 ref 导致 `v-for` 遍历对象 key 而非 ticket 行。
- 🔴⭐ **[2026-06-17]** [views/portal/ChatPipelineSteps.vue](/web/src/views/portal/ChatPipelineSteps.vue) — **引用不存在的 `s.success` 属性**：`PipelineMetrics.Step` 只有 `{id, label, duration_ms}`，无 `success` 字段。所有步骤标记始终渲染为 `'failed'` class。
- 🔴⭐ **[2026-06-17]** [views/admin/AuditLog.vue](/web/src/views/admin/AuditLog.vue) — **分页失效**：`total.value = (res as any).total || logs.value.length` —— `total` 总为当前页条目数，Pagination 组件始终显示只有一页。

### 数据流与类型安全

- 🟡⭐ **系统性 `as any` 类型侵蚀**（~20 个文件）：`(res as any).data || res` 模式遍布组件和 Store，TypeScript 类型检查形同虚设。根因是组件不确定响应拦截器是否已解包 `ApiResponse<T>` 包装。
- 🟡⭐ [stores/auth.ts](/web/src/stores/auth.ts) — **循环依赖风险**：`api/auth.ts` 从 `@/stores/auth` 导入类型，若 store 未来从 API 导入则形成循环
- 🟡 [stores/chat.ts](/web/src/stores/chat.ts) — 反馈提交错误仅 `console.error`，用户点击「已解决/未解决」后静默失败
- 🟡 [views/portal/TicketSubmit.vue](/web/src/views/portal/TicketSubmit.vue) — `chat_context` 来自 URL query 参数直接传入 API，无校验（JSON 注入风险）
- 🔴⭐ **[2026-06-17]** [stores/chat.ts](/web/src/stores/chat.ts) — **re-entrant `sendQuestion` 竞态**：旧 stream 的 onDone/onError 回调异步将当前 `abortController` 置 null，覆盖新请求的 controller。后续 abort 调用 TypeError crash。

### 配置管理

- 🟡 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — 每次编辑必须重新输入 API Key（后端返回脱敏值，前端清空表单）
- 🟡⭐ [views/admin/ModelConfig.vue](/web/src/views/admin/ModelConfig.vue) + [views/admin/SystemConfig.vue](/web/src/views/admin/SystemConfig.vue) — **重复配置管理**：两页面独立管理 `ai.default_top_k` 和 `ai.confidence_threshold`，修改互不可见，最后写入胜出。📝 [PRD.md §3.1](PRD.md) 记载这两个参数应为统一 AI 配置，而非分散在两个独立页面。
- 🟡 [views/admin/ModelConfig.vue](/web/src/views/admin/ModelConfig.vue) — `Promise.all` 保存两个 `setConfig`，部分失败无回滚。

### 组件拆分与重复代码

- 🟡 [views/portal/Chat.vue](/web/src/views/portal/Chat.vue) — 组件 >560 行，应拆分为 ChatInput/ChatMessage/ChatPipeline 子组件
- 🟡 [views/admin/LLMConfig.vue](/web/src/views/admin/LLMConfig.vue) — 组件 >610 行，应拆分。注意：行 166-167 有重复 TODO（组件拆分提示写了两次），应合并。
- 🟡 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 组件 >400 行，多文件上传只显示单个汇总结果
- 🟡⭐ **重复 `formatDate`**：[utils/date.ts](/web/src/utils/date.ts)、[utils/format.ts](/web/src/utils/format.ts)、Messages.vue、Dashboard.vue 各有一份独立实现。`utils/format.ts` 全文件为 `utils/date.ts` 的完整副本且零调用方——应删除。
- 🟡 [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — `router.back()` 在直接访问页面时可能离开应用
- 🟡 [components/common/StatusBadge.vue](/web/src/components/common/StatusBadge.vue) — `knowledge` 类型未实现：TEXT_MAP 和 TYPE_MAP 中缺少 knowledge 键，知识文章状态渲染为「未知」
- 🟡⭐ **[2026-06-17]** **跨文件重复代码**：phone/email/password 校验 regex 在 UserList/TicketSubmit/ChangePassword 三处重复。Toast 内联模板在 10+ 文件中重复。Modal overlay 样式在 5+ 文件中重复。
- 🟡⭐ **[2026-06-17]** **`types/menu.ts` vs `api/role.ts`** — 两个 `MenuItem` 类型定义不一致：`types/menu.ts` 含 5 字段，`api/role.ts` 含 8 字段。登录响应丢失 parent_id/sort_order/type。

### P1 缺陷（2026-06-17 审计新发现）

- 🟡⭐ [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 使用原生 `alert()` 而非 `useToast()`：10 处 `alert()` 调用阻塞 UI 线程且无设计系统样式集成。
- 🟡⭐ [views/admin/KnowledgeList.vue](/web/src/views/admin/KnowledgeList.vue) — KB 编辑对话框静默清空 description：`startEditKB` 始终 `description: ''`，保存后服务端描述被覆盖为空字符串。
- 🟡⭐ [views/admin/RoleManage.vue](/web/src/views/admin/RoleManage.vue) — 权限列表硬编码：`availablePermissions` 为静态数组，后端新增权限时前端不可见。应通过 `listMenus` API 动态获取。
- 🟡⭐ [stores/chat.ts](/web/src/stores/chat.ts) — `clearSession()` 不重置 `selectedKBID` 和 `ragOptions`：KP 被管理员删除后，旧的 KB ID 导致下次提问报错。
- 🟡 [views/admin/ModelConfig.vue](/web/src/views/admin/ModelConfig.vue) — `Promise.all` 部分失败：两个 `setConfig` 若第一个成功第二个失败，配置处于不一致状态。
- 🟡⭐ **[2026-06-17]** [views/admin/Dashboard.vue](/web/src/views/admin/Dashboard.vue) — **`avg_confidence` 为 null 时显示 NaN%**：`(null * 100).toFixed(0)` → `"NaN%"`
- 🟡⭐ **[2026-06-17]** [views/admin/KnowledgeList.vue](/web/src/views/admin/KnowledgeList.vue) + [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — `alert()`/`confirm()` 替代 toast/modal，与其余 admin 视图风格不一致。

### 类型与边界情况

- 🟡 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — 申告创建时间显示为原始 ISO 字符串，应使用 `formatDate`
- 🟡 [views/admin/TicketDetail.vue](/web/src/views/admin/TicketDetail.vue) — 知识候选创建：KB ID 自由输入无下拉选择/存在性校验
- 🟡 [views/admin/SystemConfig.vue](/web/src/views/admin/SystemConfig.vue) — Number 强制转换可能丢失前导零（如 `"00123"` → `123`）
- 🟡 [views/admin/UserList.vue](/web/src/views/admin/UserList.vue) — 本地 `UserItem` 接口与 API 类型不一致（自创 `role_ids` 字段）
- 🟡 [views/admin/KnowledgeList.vue](/web/src/views/admin/KnowledgeList.vue) — `(res.data as any).articles || (res.data as any).items || []` 三种回退提取，暴露 API 响应形状不确定
- 🟡 [components/layout/AdminLayout.vue](/web/src/components/layout/AdminLayout.vue) — 菜单路径硬编码字符串，应引用路由名称

### 代码 TODO

- 📌 [composables/useAIConfig.ts:48](/web/src/composables/useAIConfig.ts) — loadConfig swallows errors, uses defaults silently
- 📌 [views/admin/KnowledgeEdit.vue:151](/web/src/views/admin/KnowledgeEdit.vue) — fetchArticle/fetchKBs only console.error on failure
- 📌 [views/admin/KnowledgeEdit.vue:279](/web/src/views/admin/KnowledgeEdit.vue) — 多文件上传时应显示每个文件的独立状态和失败原因
- 📌 [views/portal/TicketDetail.vue:96](/web/src/views/portal/TicketDetail.vue) — API call failures silently set null
- 📌 [views/portal/TicketSubmit.vue:117](/web/src/views/portal/TicketSubmit.vue) — 组件超过 340 行，可提取表单字段组件和验证逻辑
- 📌 [views/portal/Chat.vue:88](/web/src/views/portal/Chat.vue) — 增加显式的输入校验（trim + max rune count）
- 📌 [views/admin/Dashboard.vue:116](/web/src/views/admin/Dashboard.vue) — 统计卡片可增加"更新时间"和手动刷新按钮
- 📌 [views/admin/Dashboard.vue:147](/web/src/views/admin/Dashboard.vue) — 小数值统一最小 4px 会让 0 和 1 视觉差异不明显
- 📌 [views/admin/LLMConfig.vue:209](/web/src/views/admin/LLMConfig.vue) — getLLMConfigs 类型应直接返回 ApiResponse 解包后的 data

### P2 缺陷（2026-06-17 审计新发现）

- 🟢⭐ [views/admin/Dashboard.vue](/web/src/views/admin/Dashboard.vue) — `fetchTrends` 失败静默吞掉：`catch { trendPoints.value = [] }`，用户无法区分"无趋势数据"和"趋势加载失败"。
- 🟢⭐ [views/portal/Messages.vue](/web/src/views/portal/Messages.vue) + [views/portal/TicketQuery.vue](/web/src/views/portal/TicketQuery.vue) — API 失败静默清空数据数组，无用户可见错误提示。
- 🟢⭐ [router/index.ts](/web/src/router/index.ts) — 角色不足时重定向到 `/login` 而非 403 页面，已登录用户看到闪现的登录页。
- 🟢⭐ [utils/knowledge.ts](/web/src/utils/knowledge.ts) — `parsing`/`chunking`/`embedding` 三种状态映射到同一 CSS class `pending`，用户无法区分文档处理阶段。
- 🟢⭐ [stores/app.ts](/web/src/stores/app.ts) — `decrementUnread` 死代码：导出但全项目零调用方。
- 🟢⭐ [components/layout/AdminLayout.vue](/web/src/components/layout/AdminLayout.vue) — 菜单匹配用 `path.startsWith()` 硬编码分组，URL 结构变更时静默失效。
- 🟢⭐ [views/admin/KnowledgeEdit.vue](/web/src/views/admin/KnowledgeEdit.vue) — 无 tag 数量上限，超出服务端限制时返回不友好错误。
- 🟢⭐ **[2026-06-17]** [views/admin/UserList.vue](/web/src/views/admin/UserList.vue) — `page_size: 100` 硬编码，>100 用户时不可见。Freeze/Restore 无确认弹窗。
- 🟢⭐ **[2026-06-17]** [views/auth/ChangePassword.vue](/web/src/views/auth/ChangePassword.vue) — 密码修改成功后直接跳转登录页，无成功提示。confirmPassword 校验在手写 handler 而非表单规则中。

### 基础设施

- 🟢 [utils/request.ts](/web/src/utils/request.ts) — `loadingState` 模块级全局计数器，SSR/并发测试不安全
- 🟢 [utils/request.ts](/web/src/utils/request.ts) — `baseURL` 为空依赖 Vite 代理，应读取 `VITE_API_BASE_URL`
- 🟢 [composables/useToast.ts](/web/src/composables/useToast.ts) — 每个组件独立 toast 状态，多 toast 冲突
- 🟢 [composables/useTheme.ts](/web/src/composables/useTheme.ts) — 模块级 `localStorage` 访问，SSR 不兼容
- 🟢 [api/dashboard.ts](/web/src/api/dashboard.ts) — `granularity` 参数定义但后端未实现（死代码）

---

## 10. 整表空数据（架构性变更）

> 以下表定义了 model 和 repository，但无 Service 层代码实际写入数据：

- 🔴 `audit_logs` — `AuditRepo.Create` 存在但零调用方，详细调用点清单见 §7 审计日志（共 7 个 Service 缺失审计写入）
- 🔴 `chat_sessions.sources` — `CreateChatSession` 未填充 `Sources` 字段，检索引用证据永远为空
- 🟡 `system_configs.description` — `Upsert` 未设置 `Description`，配置说明永远为空
- ✅ `chat_messages` — `ChatRepo.CreateBatch` 多轮对话重构中已修复调用方

---

## 11. P0 项代码 TODO 覆盖验证

> 本节仅验证 P0 项在代码中是否均有对应 TODO 注释，不再罗列非 P0 的 TODO 列表。
> 所有代码 TODO 注释已归入上方 §1-§10 对应章节（以 📌 标记）。

### 11.1 P0 代码 TODO 覆盖状态

经全量扫描，30 个 P0 项对应关系：

| P0 # | 文件 | 问题 | 代码 TODO |
|------|------|------|-----------|
| 1 | `adapter/llm_client.go` | 重试机制完全失效 | ✅ 已有 |
| 2 | `model/llm_config.go` | API Key 明文存储 | ✅ 已有 |
| 3 | `handler/llm_config.go` | TestConnection 测试错误端点 | ✅ 已有 |
| 4 | `handler/llm_config.go` | UpdateConfig 清空 api_key | ✅ 已有 |
| 5 | `service/llm_config_service.go` | 事务用错 DB 句柄 | ✅ 已有 |
| 6 | `repository/role_repo.go` | Delete(0) 删除全部角色 | ✅ 已有 |
| 7 | `cmd/main.go` | nil 传播到下游 panic | ✅ 已有 |
| 8 | `config/config.go` | 配置 YAML 格式错误静默吞掉 | ✅ 已有 |
| 9 | `database/migrate.go` | ASC+DESC 双重索引 | ✅ 已有 |
| 10 | `web/src/router/index.ts` | JWT atob base64url 不兼容 | ✅ 已有 |
| 11 | `web/src/api/chat.ts` | SSE 流绕过 Axios 拦截器 | ✅ 已有 |
| 12 | `views/admin/LLMConfig.vue` | 创建配置时测试连接崩溃 | ✅ 已有 |
| 13 | `repository/pagination.go` | 全层缺少 context.Context | ✅ 已有 |
| 14 | `repository/llm_config_repo.go` | is_default 缺部分唯一索引 | ✅ 已有 |
| 15 | `service/knowledge_service.go` | DeleteByArticle 非原子 | ✅ 已修复 |
| 16 | `model/enums.go` vs `API/knowledge.md` | 文章状态编号不一致 | ✅ 已修复 |
| 17 | `handler/knowledge.go` vs `API/knowledge.md` | 上传 API 字段名不一致 | ✅ 已修复 |
| 18 | `web/src/stores/chat.ts` | crypto.randomUUID() 无 fallback | ✅ 已有 |
| 19 | `views/admin/TicketDetail.vue` | 操作按钮缺 loading 守卫 | ✅ 已有 |
| 20 | `web/src/App.vue` | NMessageProvider 死代码 | ✅ 已有 |
| 21 | ⭐ `views/portal/TicketSubmit.vue` | submitting 永不重置 | 待添加 |
| 22 | ⭐ `views/admin/TicketList.vue` | 响应解包破表 | 待添加 |
| 23 | ⭐ `views/portal/ChatPipelineSteps.vue` | s.success 不存在 | 待添加 |
| 24 | ⭐ `views/admin/AuditLog.vue` | 分页失效 | 待添加 |
| 25 | ⭐ `stores/chat.ts` | re-entrant sendQuestion 竞态 | 待添加 |
| 26 | ⭐ `pkg/response/response.go` | ErrAlreadyFrozen/Active → 500 | 待添加 |
| 27 | ⭐ `database/migrate.go` | DESC 索引 no-op | 待添加 |
| 28 | ⭐ `rag/bm25.go` | BuildIndex panic 死锁 | ✅ 已修复 |
| 29 | ⭐ `rag/chunker.go` | mergeSplits 1.5x 溢出 | 待添加 |
| 30 | ⭐ `service/chat_service.go` | FindMessagesBySession 静默丢弃 | 待添加 |

**覆盖度：20/30 已有 TODO。2026-06-17 新增 10 项待标记。**

---

## 统计

| 业务流程 | 🔴 P0 | 🟡 P1 | 🟢 P2 | 📌 TODO | 合计 |
|----------|-------|-------|-------|---------|------|
| 1. 认证与授权 | 2⭐ | 4 | 2 | 0 | 8 |
| 2. 智能问答 RAG | 4⭐ | 7+3📝 | 5 | 0 | 19 |
| 3. 知识库与文档管理 | 1⭐ | 4 | 1 | 0 | 6 |
| 4. 申告管理 | 9 | 12+1📝 | 2 | 10 | 34 |
| 5. 用户与角色管理 | 2 | 12 | 3 | 4 | 21 |
| 6. LLM 配置与适配层 | 14+2📝 | 12 | 0 | 0 | 28 |
| 7. 数据看板与审计 | 11 | 6 | 2+3📝 | 7 | 29 |
| 8. 基础设施与部署 | 15 | 17+1📝 | 5 | 19 | 57 |
| 9. 前端架构与交互 | 15⭐ | 14+5⭐ | 10+5⭐ | 9 | 58 |
| 10. 整表空数据 | 2 | 1 | 0 | 0 | 3 |
| 11. P0 覆盖验证 | — | — | — | — | (维护) |
| **合计** | **75** | **89** | **30+9📝** | **49** | **~263** |

> ⭐ 标记项为 2026-06-17 审计新发现（前后端共 70+ 项）。
> 📝 标记项为代码与 API 文档/PRD/TECH.md 不一致的文档缺陷。
> 📌 标记项为代码中已存在的 TODO 注释。

### P0 速览（生产环境最优先修复）

1. LLM/Embedding 重试机制完全失效
2. API Key 明文存储 📝
3. TestConnection 测试错误端点 📝
4. UpdateConfig 清空 api_key 📝
5. 事务用错 DB 句柄（`llm_config_service.go` Create/Update 双空壳事务）
6. `role_repo.Delete(0)` 删除全部角色
7. pgvector/MinIO nil 传播 → panic
8. 配置 YAML 格式错误静默吞掉
9. ASC+DESC 双重索引 + DESC 创建 no-op
10. JWT atob base64url 不兼容（前端）
11. SSE 流绕过 Axios 拦截器（前端）
12. 前端创建 LLM 配置崩溃
13. Repository 全层缺 context.Context
14. `is_default` 缺部分唯一索引
15. ~~DeleteByArticle 非原子~~ ✅ 已修复
16. ~~文章状态编号文档 vs 代码不一致~~ ✅ 已修复
17. ~~上传 API 字段名文档 vs 代码不一致~~ ✅ 已修复
18. ⭐ crypto.randomUUID() 无 fallback（前端）
19. ⭐ TicketDetail 缺 loading 守卫（前端）
20. ⭐ NMessageProvider 死代码（前端）
21. ⭐ **[2026-06-17]** TicketSubmit submitting 永不重置（前端）
22. ⭐ **[2026-06-17]** TicketList 响应解包破表（前端）
23. ⭐ **[2026-06-17]** ChatPipelineSteps `s.success` 不存在（前端）
24. ⭐ **[2026-06-17]** AuditLog 分页完全失效（前端）
25. ⭐ **[2026-06-17]** chat.store.ts re-entrant sendQuestion 竞态（前端）
26. ⭐ **[2026-06-17]** ErrAlreadyFrozen/Active 返回 500 而非 400
27. ⭐ **[2026-06-17]** migrate.go DESC 索引 no-op
28. ⭐ ~~BM25 BuildIndex panic~~ ✅ 已修复
29. ⭐ ~~mergeSplits 1.5×~~ ✅ 已修复
30. ⭐ ~~FindMessagesBySession 静默丢弃~~ ✅ 已修复

---

**最后更新**：2026-06-17（§3 知识库 5 项修复：UploadDocuments 双倍内存→bytes.NewReader + CountArticlesByKB 错误日志 + allowedTypes 包级常量 + ProcessTask 回调方法化 + 构造函数 functional options 重构。）
