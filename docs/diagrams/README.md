# OpsMind 架构与业务流程图

> 按业务模块组织，每文件包含流程图 + 时序图，从输入到输出体现完整函数调用链。

## 快速导航

| 我想了解... | 看这个文件 | 核心调用链 |
|-------------|-----------|-----------|
| **系统整体架构** | [architecture.md](architecture.md) | `main()` → `router.Setup` → `ListenAndServe` |
| **登录认证和权限** | [auth-rbac.md](auth-rbac.md) | `AuthHandler.Login` → `JWTAuth` → `RequirePermission` |
| **RAG 智能问答** | [chat-rag.md](chat-rag.md) | `ChatHandler.StreamChatMessage` → `Pipeline.Execute` → SSE |
| **知识库和文档** | [knowledge.md](knowledge.md) | `Publish` → `Chunker.Split` → `Embedder.Embed` → `VectorStore.BatchInsert` |
| **申告工单** | [ticket.md](ticket.md) | `CreateTicket` → `UpdateStatus`(状态机) → `AutoClose` |
| **LLM 配置和调用** | [llm-config.md](llm-config.md) | `CreateConfig` → `atomic.Value.Store` → `OpenAIClient.ChatCompletionStream` |
| **看板和审计** | [dashboard-audit.md](dashboard-audit.md) | `DashboardService.GetStats` → 7 条原生 SQL 并行 |
| **数据库模型** | [data-model.md](data-model.md) | ER 图 + 索引策略 + 业务域划分 |
| **全部 API 全景** | [master-data-flow.md](master-data-flow.md) | 62 个端点 + 10 模块端到端 + API 完整性矩阵 |

## 图表索引

| 文件 | 业务模块 | 内容 |
|------|---------|------|
| [architecture.md](architecture.md) | 系统架构 | 分层全景 + 请求生命周期 + 模块依赖 + 启动流程 + 依赖注入 + 优雅关闭 (6 图) |
| [auth-rbac.md](auth-rbac.md) | 认证与权限 | 登录全链路 + Token 刷新 + 中间件链 + 路由分组 + 用户 CRUD + 角色菜单 (7 图) |
| [chat-rag.md](chat-rag.md) | 智能问答 RAG | 端到端数据流 + 会话创建 + 流式时序 + 降级矩阵 + SSE 协议 + 数据形态表 (6 图) |
| [knowledge.md](knowledge.md) | 知识管理 | 双路径入库 + 状态机 + 发布管道 + 发布时序 + KB CRUD + 删除级联 + goroutine pool (7 图) |
| [ticket.md](ticket.md) | 申告管理 | 全生命周期 + 状态机 + 转换时序 + 自动关闭 + 编号生成 + 补充流程 + 数据形态表 (8 图) |
| [llm-config.md](llm-config.md) | LLM 配置 | CRUD 热替换 + LLM 调用链 + Embedding 调用链 + pgvector 调用链 + atomic.Value 原理 (5 图) |
| [dashboard-audit.md](dashboard-audit.md) | 看板与审计 | Dashboard 统计 + 趋势 + 审计日志 + 配置读写 + 跨模块事件驱动 (5 图) |
| [data-model.md](data-model.md) | 数据模型 | 核心 ER 图 + 索引策略 + 业务域划分 (3 图) |
| [master-data-flow.md](master-data-flow.md) | 全业务总览 | 全景路由映射 + 10 模块端到端时序 + API 完整性矩阵 (62 端点) |

## 架构层次

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
| 智能问答 | `ChatHandler.StreamChatMessage` | `ChatService.StreamChat` → `LLMService.StreamChat` | `Pipeline.Execute` → `OpenAIClient.ChatCompletionStream` + `writeSSEEvent` |
| 会话创建 | `ChatHandler.CreateChatSession` | `ChatService.CreateSession` | `KnowledgeRepo.FindKBByID` → `ChatRepo.Create` |
| 知识发布 | `KnowledgeHandler.Publish` | `KnowledgeService.Publish` → `Chunker.Split` → `Embedder.Embed` | `VectorStore.BatchInsert` + `DeleteByArticle` |
| 知识库删除 | `KnowledgeHandler.DeleteKB` | `KnowledgeService.DeleteKB` → `VectorStore.DeleteByKB` | `KnowledgeRepo.DeleteKB` (事务级联) |
| 文档上传 | `KnowledgeHandler.UploadDocuments` | `KnowledgeService.UploadDocuments` (格式/大小校验) | `DocParser.Parse` → `Processor.Submit` → goroutine pool |
| 申告创建 | `TicketHandler.CreateTicket` | `TicketService.CreateTicket` (编号生成) | `TicketRepo.Create` |
| 申告处理 | `TicketHandler.UpdateStatus` | `TicketService.UpdateStatus` (状态机 + TxManager) | `TicketRepo` + `MessageService.NotifySupplement` |
| 自动关闭 | `Scheduler.runAutoCloseLoop` | `TicketService.AutoClose` (批量事务) | `TicketRepo.AutoCloseTickets` |
| LLM 配置 | `LLMConfigHandler.CreateConfig` | `LLMConfigService.CreateConfig` → `LLMConfigManager` | `atomic.Value.Store` 热替换 |
| LLM 调用 | — | `LLMService` / `Pipeline` 各步骤 | `OpenAIClient.ChatCompletion` / `ChatCompletionStream` |
| Embedding | — | `Embedder.Embed` (batchSize=32) | `OpenAIEmbeddingClient.CreateEmbeddings` |
| 向量存储 | — | — | `PgvectorStore.CosineSearch` / `BatchInsert` / `DeleteByArticle` / `DeleteByKB` |
| 认证 | `AuthHandler.Login` | `AuthService.Login` → `bcrypt.CompareHashAndPassword` | `jwt.GenerateAccessToken` + `jwt.GenerateRefreshToken` |
| 中间件 | `JWTAuth` middleware | `jwt.ParseWithClaims` + TokenType 校验 | `c.Set("userID")` → `c.Next()` |
| 权限 | `RequirePermission` middleware | `RoleService` → `UserRepo.GetUserPermissions` | `BatchGetRoleMenus` → `buildTree` |
| 系统启动 | `main()` | `NewPipeline` / `NewProcessor` / `NewScheduler` | 全部 `New*` 构造函数 + `router.Setup` |
