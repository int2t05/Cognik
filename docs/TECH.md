# OpsMind — 技术架构文档

| 项目 | 内容 |
| --- | --- |
| 日期 | 2026-06-12 |
| 关联文档 | [PRD](PRD.md) · [API 文档](API/README.md) · [架构图](diagrams/) |

## 1. 架构总览

OpsMind 采用**单体分层架构（Modular Monolith）**，按 Handler → Service → Repository 三层分离。RAG 引擎（`rag/`包）为自包含领域模块，不依赖 HTTP 层。

```
客户端 (Vue 3 SPA)
  │
  ├─ HTTP/REST + SSE ──▶ Go 后端 (Gin, 端口 8080)
  │                        │
  │                        ├─ Handler 层 — 参数校验、响应格式
  │                        ├─ Service 层 — 业务逻辑、事务编排
  │                        ├─ Repository 层 — GORM 数据访问
  │                        ├─ RAG 引擎 (rag/) — 检索管道、文档处理
  │                        └─ Adapter 层 — LLM/Embedding/pgvector/MinIO
  │                             │
  └─────────────────────────────┼──────────────────
                                ▼
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                  ▼
      pgvector/pg18        MinIO           llama.cpp (可选)
      (业务+向量)         (对象存储)       (OpenAI-compat API)
```

## 2. 分层设计

### 2.1 Handler 层

- 职责：请求参数解析与校验、调用 Service、格式化统一响应
- 每个 Handler 对应一个 API 域（Auth / User / Role / Ticket / Knowledge / Chat / LLMConfig / Dashboard / Audit / Config / Message），共 11 个
- 共享工具函数：`parsePagination`、`parseID`、`getCurrentUserID`、`handleServiceError`

### 2.2 Service 层

- 职责：业务逻辑、状态机校验、事务编排、跨模块调用
- 消费者接口模式：每个 Service 定义它所需的依赖接口，隐式满足而非显式声明
- LLM 配置通过 `atomic.Value` 热替换
- **LLMService**（`service/llm_service.go`）：RAG 检索 + 动态 prompt 构建 + LLM 流式/同步调用统一编排。`StreamChat` 用于 SSE 流式路径，`SyncChat` 用于非流式 JSON 路径。ChatService 通过 LLMService 间接使用 LLMClient，不再直接持有适配层引用。

### 2.3 Repository 层

- 职责：GORM 数据访问，无业务逻辑
- 通用分页辅助函数 `Paginate[T]` 消除各 Repo 重复

### 2.4 RAG 引擎（`server/internal/rag/`）

自包含的检索增强生成模块，不依赖 HTTP / Handler / Service 层：

| 文件 | 职责 |
|------|------|
| `pipeline.go` | 管道编排器，串联全部步骤 |
| `query_rewrite.go` | LLM 查询改写（消除指代歧义） |
| `multi_route.go` | LLM 多路检索（JSON 数组输出，count 钳位 [2,4]） |
| `hybrid.go` | RRF 融合（向量 + BM25） |
| `bm25.go` | Okapi BM25 算法（gse 中文分词） |
| `rerank.go` | cross-encoder 重排序（adapter.Reranker 接口） |
| `retriever.go` | VectorRetriever（embedder + vector store 包装） |
| `chunker.go` | RecursiveCharacterTextSplitter 分块 |
| `embedder.go` | 批量 Embedding 生成 |
| `document_parser.go` | PDF/DOCX/MD/TXT 多格式解析 |
| `processor.go` | 异步文档处理 goroutine pool |
| `types.go` | 共享类型和接口定义 |

### 2.5 Adapter 层（`server/internal/adapter/`）

封装外部协议而非特定服务：

| 接口 | 实现 | 说明 |
|------|------|------|
| `LLMClient` | `OpenAIClient` | ChatCompletion + ChatCompletionStream |
| `EmbeddingClient` | `OpenAIEmbeddingClient` | `/v1/embeddings` |
| `VectorStore` | `PgvectorStore` | pgvector batch insert / cosine search / delete |
| `StorageClient` | `MinIOClient` | 对象上传/下载 / presigned URL / 删除 |

## 3. 数据库设计

### 3.1 核心表

| 表 | 说明 |
|------|------|
| `users` | 用户（bcrypt 密码哈希） |
| `roles` / `user_roles` / `role_menus` | RBAC 角色权限 |
| `knowledge_bases` | 知识库，关联 `llm_configs.id` |
| `knowledge_articles` | 知识文章（手动+上传），状态机 + 处理状态 |
| `knowledge_chunks` | 文章分块 + halfvec 向量（HNSW 索引） |
| `chat_sessions` / `chat_messages` | 问答会话和消息 |
| `tickets` / `ticket_records` | 申告和操作记录 |
| `llm_configs` | LLM/Embedding 提供商配置（独立 Base URL，最多一个默认） |
| `audit_logs` | 操作审计日志 |
| `configs` | 系统配置键值对（如 `app_name`） |
| `messages` | 站内消息通知 |

### 3.2 pgvector 配置

- 类型：`halfvec(N)`（半精度 float16，节省 50% 存储）
- 索引：HNSW（`halfvec_cosine_ops`），查询 < 50ms（10000 分块）
- 算子：`<=>`（cosine distance）

分块参数：chunk_size=1000, overlap=200（RecursiveCharacterTextSplitter）

### 3.3 文章状态机

```
草稿(1) → 待审核(2) → 审核通过(3) → 已发布(4) → 已停用(0)
              ↓
           驳回(5)
```

对应 `model/enums.go`：ArticleStatusDisabled=0, ArticleStatusDraft=1, ArticleStatusReviewing=2, ArticleStatusApproved=3, ArticleStatusPublished=4, ArticleStatusRejected=5

文档处理状态：`chunking → embedding → indexing → completed`（失败 → `failed`，可重试）

## 4. RAG 管道配置

```go
type RAGOptions struct {
    TopK         int                 // 返回分块数，默认 5（零值 → Normalize() 填 5）
    QueryRewrite bool                // 查询改写，默认 true
    MultiRoute   bool                // 多路检索，默认 true
    Hybrid       bool                // BM25 混合检索，默认 true
    Rerank       bool                // 重排序，默认 true
    RouteCount   int                 // 子查询数，默认 3（零值 → Normalize() 填 3）
    RerankCount  int                 // 进入重排序候选数，默认 topK*3（零值 → Normalize() 填 topK*3）
    History      []map[string]string // 对话历史（不入 JSON），用于查询改写上下文消歧
}
```

`Normalize()` 方法在 Pipeline.Execute 入口处自动调用，零值字段按以下规则填充：
- TopK ≤ 0 → 5
- RouteCount ≤ 0 → 3
- RerankCount ≤ 0 → TopK × 3
- RerankCount < TopK → TopK × 3（确保重排序候选池≥目标数）

降级矩阵：

| 步骤 | 失败行为 | 额外条件 |
|------|----------|----------|
| 查询改写 | 降级——使用原始 question | llmClient == nil 时静默跳过 |
| 多路检索 | 降级——使用单路检索 | llmClient == nil 时静默跳过 |
| 向量检索 | **阻塞**——核心路径，返回错误 | — |
| BM25 检索 | 降级——仅用向量结果 | — |
| RRF 融合 | 降级——使用单路结果 | — |
| 重排序 | 降级——使用融合排序结果 | reranker == nil 时静默跳过；Python 子进程崩溃时降级；重排前按 RerankCount 截断候选池 |
| LLM 生成 | **阻塞**——核心路径，返回错误 | — |

重排序使用 cross-encoder 模型（BAAI/bge-reranker-base, FP16），通过 Python 子进程（`rerank_server.py`）stdin/stdout JSON Lines 协议通信。
模型在子进程启动时加载常驻内存（~560MB），单次推理延迟约 50ms。
不再使用 LLM prompt 方案做重排序。

## 5. 配置与环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_JWT_SECRET` | JWT 签名密钥 | 生产环境必须设置 |
| `OPSMIND_LLM_BASE_URL` | LLM API 地址 | `http://llama-cpp:8080/v1` |
| `OPSMIND_LLM_API_KEY` | API 密钥（OpenAI 需要；llama.cpp 留空） | — |
| `OPSMIND_LLM_MODEL` | LLM 模型名称 | `qwen3-4b` |
| `OPSMIND_LLM_MAX_TOKENS` | 最大生成 Token 数 | 8192 |
| `OPSMIND_EMBEDDING_BASE_URL` | Embedding API 地址（空则回退到 LLM_BASE_URL） | — |
| `OPSMIND_EMBEDDING_API_KEY` | Embedding API 密钥（空则回退到 LLM_API_KEY） | — |
| `OPSMIND_EMBEDDING_MODEL` | Embedding 模型名称 | `bge-m3` |
| `OPSMIND_EMBEDDING_DIMENSION` | 向量维度 | 1024 |
| `OPSMIND_DATABASE_PASSWORD` | PostgreSQL 密码 | `opsmind_dev` |
| `OPSMIND_MINIO_ACCESS_KEY` / `OPSMIND_MINIO_SECRET_KEY` | MinIO 管理员凭据 | `minioadmin` |
| `OPSMIND_MINIO_ENDPOINT` | MinIO 端点 | `localhost:9000` |
| `OPSMIND_MINIO_USE_SSL` | MinIO SSL | `false` |
| `OPSMIND_SERVER_PORT` | 服务端口 | `8080` |
| `OPSMIND_SERVER_MODE` | 运行模式 | `debug` |
| `OPSMIND_JWT_ACCESS_EXPIRE` | Access Token 有效期 | `2h` |
| `OPSMIND_JWT_REFRESH_EXPIRE` | Refresh Token 有效期 | `168h` |
| `OPSMIND_CORS_ALLOW_ORIGINS` | CORS 允许域名 | `localhost:5173` |
| `OPSMIND_AI_DEFAULT_TOP_K` | 默认检索 TopK | `5` |
| `OPSMIND_AI_CONFIDENCE_THRESHOLD` | 置信度阈值 | `0.6` |
| `OPSMIND_DATABASE_HOST` | PostgreSQL 主机 | `localhost` |
| `OPSMIND_DATABASE_PORT` | PostgreSQL 端口 | `5432` |
| `OPSMIND_DATABASE_USER` | PostgreSQL 用户 | `opsmind` |
| `OPSMIND_DATABASE_NAME` | PostgreSQL 数据库名 | `opsmind` |
| `OPSMIND_DATABASE_SSLMODE` | PostgreSQL SSL 模式 | `disable` |
## 6. 模块接口

### 6.1 LLMClient

```go
type LLMClient interface {
    ChatCompletion(ctx, ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx, ChatRequest) (<-chan StreamChunk, error)
}
```

### 6.2 EmbeddingClient

```go
type EmbeddingClient interface {
    CreateEmbeddings(ctx, EmbeddingRequest) (*EmbeddingResponse, error)
}
```

### 6.3 VectorStore

```go
type VectorStore interface {
    BatchInsert(ctx, chunks []VectorChunk) error
    CosineSearch(ctx, kbID int64, embedding []float32, topK int) ([]SearchResult, error)
    DeleteByArticle(ctx, articleID int64) error
    DeleteByKB(ctx, kbID int64) error
    CountByKB(ctx, kbID int64) (int64, error)
    GetChunksByArticle(ctx, articleID int64) ([]ChunkContent, error)
}
```

## 7. 错误码

| code | HTTP | 说明 |
|------|------|------|
| 0 | 200 | 成功 |
| 10001 | 401 | 未登录或令牌过期 |
| 10002 | 403 | 无权限 |
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（如账号名重复） |
| 10006 | 400 | 用户已被冻结 (ErrAlreadyFrozen) |
| 10007 | 400 | 用户已处于正常状态 (ErrAlreadyActive) |
| 20001 | 503 | AI 服务不可用 (ErrAIUnavailable) |
| 20002 | 503 | RAG 服务不可用 (ErrRAGUnavailable) |
| 20003 | 503 | 存储服务不可用 (ErrStorageUnavailable) |
| 99999 | 500 | 未知错误 |

## 8. 自动关闭申告 (AutoClose)

`Scheduler` (`service/scheduler.go`) 每小时执行 `TicketAutoCloseJob`：

1. `Scheduler.runAutoCloseLoop` → 每小时触发一次
2. `Scheduler.RunAutoClose` → 调用 `ticketSvc.AutoClose(olderThan)`
3. `TicketService.AutoClose(olderThan)` → **单事务中原子执行**：
   - 调用 `TicketRepo.AutoCloseTickets(olderThan)` 原子关闭并返回 ID 列表
   - 在同一事务内为每个 ticket 创建 `action="auto_close"` 的 `TicketRecord`
   - 事务回滚时 UPDATE 和 Record 创建同时撤销
4. `TicketRepo.AutoCloseTickets(olderThan)` → **单 SQL 原子操作**：`UPDATE ... RETURNING id`，消除 SELECT-then-UPDATE 的 TOCTOU 竞态

关闭条件：`status IN (1,2,3) AND created_at < now - 7 days`

### 8.1 事务管理 (TxManager)

`TxManager` 接口（`service/tx_manager.go`）提供统一的事务抽象：

```go
type TxManager interface {
    Transaction(fn func(tx *gorm.DB) error) error
}
```

`TicketService` 通过构造函数注入 `TxManager`（而非直接持有 `*gorm.DB`），事务编排由 Service 层完成，Repository 只做纯数据操作。

**事务使用场景：**
- `UpdateStatus`：UpdateStatus(CAS) + CreateRecord 在同一事务中执行
- `SupplementTicket`：CreateRecord + UpdateStatus(CAS) 在同一事务中执行，避免孤立记录
- `AutoClose`：`UPDATE...RETURNING id` + 批量 TicketRecord 创建在同一事务中执行

**CAS（Compare-And-Swap）状态转换：**

所有状态更新使用 `WHERE id=? AND status=?` 条件，确保仅在当前状态匹配时才执行更新：
- 防止两操作者从同一旧状态并发操作产生双重记录
- RowsAffected=0 时返回"状态已变更，请刷新后重试"

**状态机常量：**

`model/enums.go` 提供完整的工单状态和操作常量，Service 层编译期约束：
- 状态：`TicketStatusPending`(1) / `TicketStatusProcessing`(2) / `TicketStatusNeedSupplement`(3) / `TicketStatusResolved`(4) / `TicketStatusClosed`(5)
- 操作：`TicketActionStart` / `TicketActionRequestInfo` / `TicketActionSupplement` / `TicketActionResolve` / `TicketActionClose`

**request_info 通知：**

`request_info` 操作成功后，同步调用 `MessageService.NotifySupplement` 向申告人发送站内消息。通知失败仅记录警告日志，不回滚工单状态变更。

## 9. 项目结构

```
server/
├── cmd/main.go                     # 入口：DI → 路由 → 调度器 → 优雅关闭
├── internal/
│   ├── config/                     # Viper 配置加载
│   ├── database/                   # GORM 连接 + AutoMigrate
│   ├── middleware/                  # JWT / RBAC / CORS / Logger / RequestID
│   ├── router/                     # 路由注册（public/portal/admin 三组）
│   ├── handler/                    # HTTP Handler 层（11 个模块 + common.go）
│   ├── service/                    # 业务逻辑层（12 个服务 + tx_manager.go + scheduler.go）
│   ├── repository/                 # 数据访问层（10 个 Repo + pagination.go）
│   ├── model/                      # GORM 数据模型（10 个文件）
│   ├── rag/                        # RAG 引擎（12 个文件）
│   ├── adapter/                    # 外部适配层（LLM/Embedding/pgvector/MinIO）
│   └── dto/                        # 请求/响应 DTO
├── pkg/                            # 公共工具（errcode/jwt/hash/response）
├── migrations/                     # 数据库迁移脚本 + 演示数据
└── tests/                          # 测试代码（config/database/model/service/handler/adapter/rag）

web/
├── src/
│   ├── api/                        # Axios API 封装（12 个文件）
│   ├── stores/                     # Pinia 状态管理（auth/chat/app）
│   ├── router/                     # Vue Router + 路由守卫
│   ├── views/                      # 页面（auth/portal/admin）
│   ├── components/                 # 通用组件（布局/分页/状态标签）
│   ├── utils/                      # 工具函数（request/auth）
│   └── styles/                     # Linear Design 暗色主题
└── nginx.conf                      # Nginx 反向代理配置
```
