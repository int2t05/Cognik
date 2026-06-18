# OpsMind 架构与业务流程图

> 基于实际代码函数调用链绘制，与 `server/` 源码保持同步。最后更新：2026-06-18

## 图表索引

| 文件 | 核心文件 | 图表数 | 说明 |
|------|----------|--------|------|
| [master-data-flow.md](master-data-flow.md) | 全部模块 | 10 | 🆕 **全业务数据流总览** — 62 个端点全景 + 10 个模块端到端流程 + API 完整性矩阵 |
| [architecture.md](architecture.md) | `main.go`, `router/`, `middleware/` | 3 | 系统架构总览 — 分层全景 + 请求生命周期 + 模块依赖 |
| [chat-rag-flow.md](chat-rag-flow.md) | `handler/chat.go` → `rag/pipeline.go` → `adapter/llm_client.go` | 3 | 智能问答 RAG 管道 — SSE 流式(writeSSEEvent) + 非流式 + 降级矩阵 |
| [knowledge-publish-flow.md](knowledge-publish-flow.md) | `handler/knowledge.go` → `rag/chunker.go` → `adapter/vector_store.go` | 5 | 知识发布管道 — 文章生命周期 + Publish + 🆕DeleteKB 流程 |
| [document-upload-flow.md](document-upload-flow.md) | `handler/knowledge.go` → `rag/document_parser.go` → `rag/processor.go` | 3 | 文档上传 — Service 层校验 + io.LimitReader(100MB) + 异步处理 |
| [ticket-lifecycle.md](ticket-lifecycle.md) | `handler/ticket.go` → `service/ticket_service.go` → `service/scheduler.go` | 3 | 申告完整生命周期 — AutoClose Service 编排 + TxManager 事务 |
| [ticket-state-machine.md](ticket-state-machine.md) | `service/ticket_service.go` | 1 | 申告状态机 — 5 态转换规则 + supplement_count 原子检查 |
| [auth-flow.md](auth-flow.md) | `handler/auth.go` → `middleware/auth.go` → `middleware/rbac.go` | 4 | 认证 RBAC — TokenType access/refresh + /auth/me + 中间件链 |
| [user-rbac-flow.md](user-rbac-flow.md) | `handler/user.go` → `service/role_service.go` | 2 | 用户管理 + 角色权限 — CountUsersByRole 删除保护 + BatchGetRoleMenus |
| [llm-config-flow.md](llm-config-flow.md) | `handler/llm_config.go` → `service/llm_config_service.go` | 4 | LLM 配置 — 构造函数注入 + MarshalJSON 脱敏 + atomic.Value 热替换 |
| [dashboard-audit-flow.md](dashboard-audit-flow.md) | `handler/dashboard.go` → `repository/dashboard_repo.go` | 2 | 看板统计 + 审计日志 — DashboardRepo 聚合查询 |
| [request-lifecycle.md](request-lifecycle.md) | `middleware/` → `router/` | 2 | 请求生命周期 — Recovery→RequestID→CORS→Logger→JWTAuth→RBAC |

## 架构层次对应

```
Handler 层   →  handler/xxx.go             请求绑定、响应格式化
Service 层   →  service/xxx.go             业务逻辑、TxManager 事务编排
Repository   →  repository/xxx.go          数据访问（GORM）、聚合查询
RAG 引擎     →  rag/xxx.go                 Pipeline / BM25 / HybridFuse / Rerank / Chunker / Embedder / Processor
Adapter 层   →  adapter/xxx.go             LLMClient / EmbeddingClient / VectorStore(pgvector) / StorageClient(MinIO)
Middleware   →  middleware/xxx.go           Recovery / RequestID / CORS / Logger / JWTAuth / RBAC
```

## 关键函数速查

| 流程 | Handler 入口 | Service 核心 | Repository / Adapter |
|------|-------------|-------------|---------------------|
| 智能问答 | `ChatHandler.StreamChatSession` | `ChatService.CreateChatSession` → `Pipeline.Execute` | `OpenAIClient.ChatCompletionStream` + `writeSSEEvent` |
| 知识发布 | `KnowledgeHandler.Publish` | `KnowledgeService.Publish` → `Chunker.Split` → `Embedder.Embed` | `VectorStore.BatchInsert` (halfvec) |
| 知识库删除 | `KnowledgeHandler.DeleteKB` | `KnowledgeService.DeleteKB` → `VectorStore.DeleteByKB` → `repo.DeleteKB` | `PgvectorStore.DeleteByKB` + 事务级联 |
| 文档上传 | `KnowledgeHandler.UploadDocuments` | `KnowledgeService.UploadDocuments` (含格式/大小校验) | `DocParser.Parse` → `Processor.Submit` |
| 申告管理 | `TicketHandler.UpdateStatus` | `TicketService.UpdateStatus` (状态机) | `TicketRepo` + `MessageService.NotifySupplement` |
| 自动关闭 | `Scheduler.runAutoCloseLoop` | `TicketService.AutoClose` (TxManager 事务) | `TicketRepo.AutoCloseTickets` (纯数据) |
| LLM 配置 | `LLMConfigHandler.TestConnection` | `LLMConfigService` → `LLMConfigManager` | `atomic.Value` 热替换 |
| 认证 | `AuthHandler.Login` | `AuthService.Login` → `bcrypt` | `jwt.GenerateAccessToken("access")` |
| 权限 | `RBAC` middleware | `RoleService` → `UserRepo.GetUserPermissions` | `BatchGetRoleMenus` |
