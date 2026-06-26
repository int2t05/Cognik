# OpsMind — 技术架构文档

> 覆盖系统架构、前后端分层设计、数据库、可靠性、设计系统。关联文档：[PRD](PRD.md) · [TODO](TODO.md) · [API](API/README.md) · [FLOW](FLOW/README.md)

## 1. 系统架构

### 1.1 分层架构

```mermaid
flowchart TB
    subgraph Client["客户端"]
        Portal["门户端 /portal/*<br/>ChatPage / TicketSubmitPage / MessagesPage"]
        Admin["管理后台 /admin/*<br/>DashboardPage / KnowledgeListPage / LLMConfigPage"]
    end

    subgraph Router["Gin Router :8080"]
        Public["/api/v1/auth — 无中间件"]
        JWTGroup["/api/v1/auth/me — JWTAuth"]
        PortalGroup["/api/v1/portal — JWTAuth"]
        AdminGroup["/api/v1/admin — JWTAuth + RBAC"]
    end

    subgraph MW["中间件链"]
        Recovery["Recovery() → RequestID() → CORS() → Logger() → JWTAuth() → RequirePermission()"]
    end

    subgraph Handler["Handler 层"]
        AH["AuthHandler"] --- CH["ChatHandler"] --- KH["KnowledgeHandler"] --- TH["TicketHandler"]
        UH["UserHandler"] --- RH["RoleHandler"] --- LH["LLMConfigHandler"] --- DH["DashboardHandler"]
    end

    subgraph Service["Service 层"]
        AuthSvc["AuthService"] --- LLMSvc["LLMService — RAG 管道 + LLM 编排"]
        ChatSvc["ChatService — 会话生命周期"] --- KnowledgeSvc["KnowledgeService — 发布管道"]
        TicketSvc["TicketService — 状态机 + TxManager"] --- LLMCfgSvc["LLMConfigService — atomic.Value 热替换"]
    end

    subgraph RAG["RAG 引擎 rag/"]
        Pipeline["Pipeline.Execute — 查询改写→多路检索→混合→重排序"]
        Processor["Processor — goroutine pool 异步文档处理"]
        Chunker["Chunker — RecursiveCharacterTextSplitter"]
        Embedder["Embedder — 批量 POST /v1/embeddings"]
    end

    subgraph Adapter["适配层"]
        LLM["LLMClient — OpenAI-compatible"]
        EMB["EmbeddingClient — OpenAI-compatible"]
        VEC["VectorStore — pgvector halfvec + HNSW"]
        STO["StorageClient — MinIO S3"]
    end

    Client --> Router --> MW --> Handler --> Service --> RAG --> Adapter
    Adapter --> Infra["PostgreSQL 18 + pgvector / MinIO / llama.cpp"]
```

### 1.2 请求生命周期

```mermaid
sequenceDiagram
    participant C as 客户端
    participant MW as 中间件链
    participant H as Handler
    participant S as Service
    participant R as Repository
    participant DB as PostgreSQL

    C->>MW: HTTP Request
    MW->>MW: Recovery → RequestID → CORS → Logger
    MW->>MW: JWTAuth: Parse token, c.Set(userID)
    alt admin 路由
        MW->>MW: RBAC: 校验权限
    end
    MW->>H: handler.Method(c)
    H->>H: ShouldBindJSON → getCurrentUserID
    H->>S: svc.BusinessMethod(req, userID)
    S->>S: 业务规则校验
    S->>R: repo.DataAccess()
    R->>DB: GORM Query
    DB-->>R: Result
    R-->>S: Data
    S-->>H: Response
    H-->>C: 200 {code:0, data:{...}}
```

### 1.3 启动流程

```mermaid
flowchart TB
    Start(["main()"]) --> Cfg["config.Load() — Viper 读取 config.yaml + 环境变量"]
    Cfg --> DB["gorm.Open(postgres) — MaxOpenConns=25, MaxIdle=10, Lifetime=5m"]
    DB --> Migrate["AutoMigrate 全部 Model + pgvector 扩展"]
    Migrate --> Adapters["初始化 LLMClient / EmbeddingClient / VectorStore / MinIOClient"]
    Adapters --> RAGInit["初始化 Chunker / Embedder / BM25 / Pipeline / Processor(goroutine pool)"]
    RAGInit --> Repos["初始化 Repository 层"]
    Repos --> Services["初始化 Service 层"]
    Services --> Handlers["初始化 Handler 层"]
    Handlers --> Router["router.Setup → 注册 60+ 路由"]
    Router --> Warmup["LLMConfigService.LoadDefaults() → atomic.Value.Store"]
    Warmup --> Scheduler["Scheduler.Start — autoClose 每小时"]
    Scheduler --> Listen["srv.ListenAndServe(:8080)"]
```

## 2. 后端架构

### 2.1 三层分离

| 层 | 职责 | 禁止 |
|----|------|------|
| Handler | 参数绑定、调用 Service、响应格式化 | 不含业务逻辑 |
| Service | 业务规则、事务编排、调用 Repo/Adapter | 不含 SQL |
| Repository | 数据访问、GORM 查询 | 不含业务规则 |

Handler 层共享工具：`parsePagination` / `parseID` / `getCurrentUserID` / `safeHandler`（消除 nil 检查样板）。

### 2.2 RAG 引擎

12 个文件组成自包含领域引擎，不依赖 HTTP 层：

| 文件 | 职责 |
|------|------|
| `pipeline.go` | 管道编排：`Execute(ctx, query, kbID, opts, onStep)` |
| `query_rewrite.go` | LLM 查询改写 |
| `multi_route.go` | LLM 多路检索路由 |
| `hybrid.go` | RRF 融合：`score = Σ 1/(60+rank_i)` |
| `bm25.go` | Okapi BM25 (k1=1.5, b=0.75) + gse 中文分词 |
| `rerank.go` | Cross-Encoder 重排序（子进程模式） |
| `chunker.go` | RecursiveCharacterTextSplitter (1000/200) |
| `embedder.go` | 批量 Embedding (batch=32) |
| `retriever.go` | 向量检索入口 |
| `processor.go` | goroutine pool 异步文档处理 |
| `document_parser.go` | PDF/DOCX/MD/TXT 解析 |
| `types.go` | 共享类型定义 |

### 2.3 适配层接口

```mermaid
classDiagram
    class LLMClient {
        <<interface>>
        ChatCompletion(ctx, ChatRequest) (*ChatResponse, error)
        ChatCompletionStream(ctx, ChatRequest) (<-chan StreamChunk, error)
    }
    class EmbeddingClient {
        <<interface>>
        CreateEmbeddings(ctx, EmbeddingRequest) (*EmbeddingResponse, error)
    }
    class VectorStore {
        <<interface>>
        BatchInsert(ctx, []VectorChunk) error
        CosineSearch(ctx, kbID, embedding, topK) ([]SearchResult, error)
        DeleteByArticle(ctx, articleID) error
        DeleteByKB(ctx, kbID) error
        ReplaceVectors(ctx, articleID, []VectorChunk) error
    }
    class StorageClient {
        <<interface>>
        Upload(ctx, bucket, key, reader, size) error
        Download(ctx, bucket, key) (io.ReadCloser, error)
    }
```

- `LLMClient` / `EmbeddingClient`：OpenAI-compatible 实现，指数退避重试（maxRetries=3，429/503 可重试）
- `VectorStore`：pgvector 实现，halfvec 半精度 + HNSW 索引，维度一致性校验
- `StorageClient`：MinIO 实现，两桶模型（`opsmind-documents` 临时 + `opsmind-published` 已发布）

## 3. 前端架构

### 3.1 路由映射

```mermaid
flowchart LR
    subgraph Routes["App Router"]
        Login["/login → LoginPage"]
        Portal["/portal/* → PortalLayout"]
        Admin["/admin/* → AdminLayout"]
    end

    subgraph PortalRoutes["门户端"]
        Chat["/portal/chat → ChatPage — SSE 流式"]
        Tickets["/portal/tickets → TicketQueryPage + TicketDetailPage"]
        NewTicket["/portal/tickets/new → TicketSubmitPage"]
        Messages["/portal/messages → MessagesPage"]
    end

    subgraph AdminRoutes["管理后台"]
        Dashboard["/admin/dashboard"]
        AdminTickets["/admin/tickets"]
        Knowledge["/admin/knowledge"]
        Users["/admin/users"]
        Roles["/admin/roles"]
        Audit["/admin/audit"]
        Config["/admin/config/llm + system"]
    end
```

### 3.2 组件分类

**Server Components（无 `'use client'`）：** RootLayout、NotFound、各 Layout 壳、AppleButton、AppleCard、AppleBadge、AppleSpinner

**Client Components（`'use client'`）：** 全部 Page 组件、AdminLayout、PortalLayout、AppleDialog、AppleInput/Textarea、ChatInput、ChatMessage、ChatPipeline、ConfirmDialog、StatusBadge、StatCard、ErrorBoundary

### 3.3 状态管理

```mermaid
flowchart TD
    Root["RootLayout (Server)"] --> Providers["<Providers> (Client)"]
    Providers --> SWR["SWRConfig<br/>revalidateOnFocus:false<br/>dedupingInterval:5000"]
    SWR --> Auth["AuthProvider<br/>token/user/roles/permissions/menus<br/>持久化: localStorage + cookie"]
    Auth --> Theme["ThemeProvider<br/>data-theme 注入"]
    Theme --> Toast["ToastProvider<br/>最多 3 条堆叠"]
    Toast --> ErrorBoundary["ErrorBoundary<br/>全局错误捕获"]
```

- AuthProvider 使用 `useLayoutEffect` 设置 token getter，确保 SWR 首次请求携带 token
- 客户端 fetch 直连后端（`NEXT_PUBLIC_API_URL`），绕过 Next.js rewrite 避免 Turbopack POST 代理 500

### 3.4 API 模块速查

| 前端模块 | 核心函数 | 后端端点 |
|---------|---------|---------|
| `lib/api/auth.ts` | login / refreshToken / changePassword / logout | `/api/v1/auth/*` |
| `lib/api/chat.ts` | createSession / getSessionList / deleteSession / submitFeedback | `/api/v1/portal/chat-sessions/*` |
| `lib/api/knowledge.ts` | getKBList / createKB / getArticleList / publishArticle / uploadDocuments | `/api/v1/admin/knowledge-bases/*` |
| `lib/api/ticket.ts` | createTicket / getMyTickets / supplementTicket / updateTicketStatus | `/api/v1/portal/tickets/*` `/api/v1/admin/tickets/*` |
| `lib/api/user.ts` | getUserList / createUser / freezeUser | `/api/v1/admin/users/*` |
| `lib/api/role.ts` | getRoleList / createRole / updateRoleMenus / getMenus | `/api/v1/admin/roles/*` `/api/v1/admin/menus` |
| `lib/api/llm_config.ts` | getLLMConfigs / createLLMConfig / testLLMConnection | `/api/v1/admin/llm-configs/*` |
| `lib/api/dashboard.ts` | getStats / getTrends | `/api/v1/admin/dashboard/*` |
| `lib/api/audit.ts` | getAuditLogs | `/api/v1/admin/audit-logs` |
| `lib/api/message.ts` | getMessages / markAsRead / getUnreadCount | `/api/v1/portal/messages/*` |
| `lib/api/config.ts` | getConfig / setConfig / getAllConfigs | `/api/v1/admin/configs/*` |

### 3.5 关键 Hooks

| Hook | 用途 |
|------|------|
| `useAuth()` | 全局认证（login/logout/hasPermission），React Context |
| `useTheme()` | 双主题切换（light/dark），localStorage + cookie + data-theme |
| `useToast()` | Toast 通知，分级消失时间 |
| `useChatStream()` | SSE 流式问答状态管理，ReadableStream 解析 + AbortController |
| `useDebounce()` | 搜索防抖，300ms 默认 |
| `useUnreadCount()` | 消息未读数轮询，30s 间隔 |

## 4. 数据库设计

### 4.1 ER 关系

```mermaid
erDiagram
    users ||--o{ user_roles : has
    users ||--o{ chat_sessions : creates
    users ||--o{ tickets : submits
    users ||--o{ messages : receives
    users ||--o{ audit_logs : triggers
    roles ||--o{ user_roles : assigned
    roles ||--o{ role_menus : has
    menus ||--o{ role_menus : bound
    menus ||--o{ menus : parent
    knowledge_bases ||--o{ knowledge_articles : contains
    knowledge_bases ||--o{ knowledge_chunks : owns
    knowledge_articles ||--o{ knowledge_chunks : split
    tickets ||--o{ ticket_records : history
    chat_sessions ||--o{ chat_messages : contains
```

### 4.2 pgvector 配置

| 参数 | 值 | 说明 |
|------|-----|------|
| 向量类型 | `halfvec(1024)` | 半精度，维度可配置 |
| 索引 | HNSW | `halfvec_ip_ops`，内积算子 |
| 距离算子 | `<=>` | 余弦距离 |
| 分块大小 | 1000 字符 | 重叠 200 字符 |

### 4.3 关键索引

| 表 | 索引 | 用途 |
|----|------|------|
| `knowledge_chunks` | HNSW `embedding halfvec_ip_ops` | 向量相似度检索 |
| `knowledge_chunks` | B-tree `kb_id` + `article_id` | 按范围过滤/删除 |
| `tickets` | UNIQUE `ticket_no` | 编号唯一 |
| `tickets` | B-tree `user_id, status, created_at` | 列表查询 + AutoClose |
| `chat_sessions` | B-tree `user_id, created_at` | 会话列表 |

### 4.4 业务域划分

```mermaid
flowchart LR
    subgraph Auth["认证域"]
        U["users"] --> UR["user_roles"] --> R["roles"] --> RM["role_menus"] --> M["menus"]
    end
    subgraph Knowledge["知识域"]
        KB["knowledge_bases"] --> A["knowledge_articles"] --> KC["knowledge_chunks (pgvector)"]
    end
    subgraph Chat["问答域"]
        CS["chat_sessions"] --> CM["chat_messages"]
        CS --> KB
    end
    subgraph Ticket["申告域"]
        T["tickets"] --> TR["ticket_records"]
    end
    subgraph System["系统域"]
        AL["audit_logs"] --- MSG["messages"] --- SC["system_configs"] --- LC["llm_configs"]
    end

    style Knowledge fill:#5e6ad215,stroke:#5e6ad2
    style Chat fill:#f59e0b15,stroke:#f59e0b
    style Ticket fill:#ef444415,stroke:#ef4444
```

## 5. 可靠性设计

### 5.1 RAG 管道降级矩阵

```mermaid
flowchart TD
    Start(["Pipeline.Execute()"]) --> QR{"查询改写?"}
    QR -->|true,成功| MR{"多路检索?"}
    QR -->|true,失败| QR_DG["降级: 使用原始 query"]
    QR -->|false| MR
    QR_DG --> MR
    MR -->|true,成功| VR["向量检索 pgvector"]
    MR -->|true,失败| VR_DG["降级: 单路检索"]
    MR -->|false| VR
    VR_DG --> VR
    VR -->|成功| BM{"BM25?"}
    VR -->|失败 ❌| VRFail["20002 RAG 不可用"]
    BM -->|true,成功| FUSE["RRF 融合 k=60"]
    BM -->|true,失败| BM_DG["降级: 仅向量结果"]
    BM -->|false| RR
    BM_DG --> RR{"重排序?"}
    FUSE --> RR
    RR -->|true,成功| LLM["LLM 流式生成"]
    RR -->|true,失败| RR_DG["降级: RRF 排序结果"]
    RR -->|false| LLM
    RR_DG --> LLM
    LLM -->|成功| Done(["SSE Stream"])
    LLM -->|失败 ❌| LLMFail["20001 AI 不可用"]

    style VRFail fill:#ef444420,stroke:#ef4444
    style LLMFail fill:#ef444420,stroke:#ef4444
    style QR_DG fill:#f59e0b15,stroke:#f59e0b
    style VR_DG fill:#f59e0b15,stroke:#f59e0b
    style BM_DG fill:#f59e0b15,stroke:#f59e0b
    style RR_DG fill:#f59e0b15,stroke:#f59e0b
```

核心原则：非核心步骤失败降级不阻塞。向量检索和 LLM 生成是核心路径，失败返回明确错误码。

### 5.2 置信度评分

三层次设计：

| 层次 | 计算方式 | 用途 |
|------|---------|------|
| Chunk Score | pgvector `<=>` 距离归一化 | 单块相关性 |
| Conf_raw | `α × S_retrieval + (1-α) × S_qa` | 综合评分 |
| 置信等级 | P30/P70 分位数阈值 | 前端展示 |

- `S_retrieval`：Top-K chunk 得分加权聚合
- `S_qa`：问题-答案 embedding 余弦相似度校验
- 阈值通过分位数动态计算（P30/P70），带完整性检查（P30≥0.3, P70≥0.6 不满足则回退硬编码）

### 5.3 外部服务重试策略

| 服务 | 重试 | 策略 | 关键保护 |
|------|------|------|---------|
| LLM API | 3 次 | 指数退避，仅 429/503 | 超时 120s |
| Embedding API | 3 次 | 指数退避，连接/超时始终重试 | batch=32 分批 |
| Reranker 子进程 | 自动重启 | 崩溃后 3s 重连 | 内部 30s 超时 |
| pgvector | 无 | 瞬时故障返回 20002 | HNSW 索引 |
| MinIO | 无 | 惰性检查 | io.LimitReader(100MB) |

### 5.4 并发安全

- `llmClient` 指针：`sync.Mutex` 保护读写，通过 `getLLMClient()` 访问
- 申告状态更新：CAS `UPDATE WHERE id=? AND status=?` 防并发覆盖
- Processor goroutine pool：`stopped` 原子标志 + channel 关闭幂等
- BM25 索引构建：`building` 原子标志 + defer recover 防 panic 残留

## 6. 配置与环境

### 6.1 环境变量（核心）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `POSTGRES_PASSWORD` | 数据库密码 | opsmind_dev |
| `JWT_SECRET` | JWT 签名密钥 | 需手动设置 |
| `LLM_BASE_URL` | LLM API 地址 | http://llama-cpp:8080/v1 |
| `LLM_API_KEY` | API 密钥（OpenAI 需要） | — |
| `LLM_MODEL` | LLM 模型名称 | qwen3-4b |
| `LLM_MAX_TOKENS` | 最大生成 Token | 8192 |
| `EMBEDDING_MODEL` | Embedding 模型 | bge-m3 |
| `EMBEDDING_DIMENSION` | 向量维度 | 1024 |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | MinIO 凭证 | minioadmin |
| `AI_CONFIDENCE_THRESHOLD` | 置信度阈值 | 0.6 |
| `AI_DEFAULT_TOP_K` | 默认检索 TopK | 5 |

> 完整 28 项环境变量见 `.env.example` 和 `docker-compose.yml`。

### 6.2 LLM 配置热替换

```mermaid
flowchart LR
    subgraph Write["写入"]
        W1["CreateConfig / UpdateConfig<br/>isDefault=true"] --> W2["Transaction: ClearDefault + Save"] --> W3["cfg.Store(newConfig)<br/>atomic.Value 即时可见"]
    end
    subgraph Read["读取（每次请求）"]
        R1["getModelConfig()"] --> R2["cfg.Load().(*LlmConfig)<br/>无锁读取"] --> R3["返回 model + client"]
    end
    W3 -.->|即时生效| R2
```

## 7. 设计系统

### 7.1 色彩

| 令牌 | 色值 | 用途 |
|------|------|------|
| Action Blue | `#0066cc` | 唯一品牌色，pill 按钮 |
| Focus Blue | `#2997ff` | 聚焦环 |
| Surface White | `#f5f5f7` | 浅色背景 |
| Surface Dark | `#1d1d1f` | 暗色背景 |
| Ink | `#1d1d1f` / `#f5f5f7` | 浅/暗主题正文字 |
| Ink Muted | `rgba(0,0,0,0.48)` | 辅助文字 |
| Hairline | `rgba(0,0,0,0.08)` | 分隔线 |

### 7.2 字体与圆角

- 字体：Inter Variable，正文字号 17px，标题 28px/20px，辅助 13px
- 按钮：完全圆角 pill（`9999px`）
- 卡片：`18px` 圆角，无边框，微阴影（`0 1px 3px rgba(0,0,0,0.04)`）
- 输入框：`12px` 圆角，hairline 边框

### 7.3 核心组件

| 组件 | 特征 |
|------|------|
| AppleButton | 4 变体：pill（蓝底白字）/ ghost（透明）/ utility（灰底）/ pearl（白底灰框） |
| AppleCard | 白底 + hairline 边框 + 12px 圆角，可选 hover 可点击 |
| AppleInput | 标准输入 + pill 搜索变体，forwardRef |
| AppleTable | 泛型 `<T>`，loading/empty 状态内置 |
| ApplePagination | 页码 + pageSize 选择器 |
| AppleDialog | Radix Dialog 封装，Apple 风格 |

## 8. 错误码

| 错误码 | HTTP | 说明 |
|--------|------|------|
| 0 | 200 | 成功 |
| 10001 | 401 | 未登录或令牌过期 |
| 10002 | 403 | 无权限 |
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突 |
| 10006 | 400 | 用户已冻结 |
| 10007 | 400 | 用户已正常 |
| 20001 | 503 | AI 服务不可用 |
| 20002 | 503 | RAG 服务不可用 |
| 20003 | 503 | 存储服务不可用 |
| 99999 | 500 | 内部错误 |

## 9. 项目结构

```
server/cmd/main.go           入口：配置→DB→RAG→Service→Handler→Router→Scheduler
server/internal/
├── config/                   Viper 配置
├── middleware/               JWT / RBAC / CORS / Logger
├── router/                   路由注册 + safeHandler
├── handler/                  11 个 Handler
├── service/                  11 个 Service + scheduler + tx_manager
├── repository/               9 个 Repository
├── model/                    GORM 模型 + 枚举
├── rag/                      自建 RAG 引擎（12 文件）
├── adapter/                  LLM / Embedding / VectorStore / StorageClient
├── dto/                      request/ + response/
server/pkg/                   jwt / hash / response / errcode
web/src/
├── app/                      Next.js App Router（login/portal/admin）
├── components/ui/            Apple Design 组件
├── components/layout/        AdminLayout / PortalLayout
├── components/shared/        StatusBadge / ConfirmDialog / StatCard
├── components/chat/          ChatInput / ChatMessage / ChatPipeline
├── lib/api/                  10 个 API 客户端模块
├── hooks/                    6 个自定义 Hooks
└── styles/                   Apple Design Tokens + 双主题 CSS
```
