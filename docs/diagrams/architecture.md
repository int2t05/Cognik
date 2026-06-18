# 系统架构

> 覆盖分层架构、请求生命周期、启动流程与依赖注入。

---

## 1. 分层架构全景

```mermaid
flowchart TB
    subgraph Client["客户端层"]
        Browser["浏览器 (Next.js + Radix UI)"]
        Portal["门户端 /portal/*<br/>ChatPage / TicketSubmitPage / MessagesPage"]
        Admin["后台管理 /admin/*<br/>DashboardPage / KnowledgeListPage / TicketListPage / LLMConfigPage"]
    end

    subgraph Router["Gin Router :8080"]
        Health["GET /health → 无认证"]
        Public["/api/v1/auth → 无中间件<br/>POST login / POST refresh"]
        JWTGroup["/api/v1/auth/me → JWTAuth<br/>POST change-password / POST logout"]
        PortalGroup["/api/v1/portal → JWTAuth<br/>chat-sessions / tickets / messages"]
        AdminGroup["/api/v1/admin → JWTAuth + RBAC<br/>tickets / knowledge-bases / users / roles / llm-configs / dashboard / audit-logs"]
    end

    subgraph MW["中间件链 middleware/"]
        direction LR
        Recovery["Recovery()"]
        RequestID["RequestID()"]
        CORS["CORS()"]
        Logger["Logger()"]
        JWTAuth["JWTAuth(secret)"]
        RBAC["RequirePermission(perm)"]
    end

    subgraph Handler["Handler 层 handler/"]
        AH["AuthHandler<br/>Login / Refresh / ChangePassword / Logout"]
        CH["ChatHandler<br/>CreateChatSession / StreamChatMessage / SubmitFeedback"]
        KH["KnowledgeHandler<br/>CreateKB / CreateArticle / Publish / UploadDocuments"]
        TH["TicketHandler<br/>CreateTicket / UpdateStatus / AddRecord"]
        UH["UserHandler<br/>Create / Freeze / Restore"]
        RH["RoleHandler<br/>Create / UpdateRoleMenus"]
        LH["LLMConfigHandler<br/>CreateConfig / TestConnection"]
        DH["DashboardHandler<br/>GetStats / GetTrends"]
        AuH["AuditHandler<br/>List"]
    end

    subgraph Service["Service 层 service/"]
        AuthSvc["AuthService"]
        LLMSvc["LLMService<br/>StreamChat — RAG 管道 + prompt + LLM 统一编排"]
        ChatSvc["ChatService<br/>会话生命周期管理"]
        KnowledgeSvc["KnowledgeService<br/>Publish — Chunker.Split + Embedder.Embed + VectorStore.BatchInsert"]
        TicketSvc["TicketService<br/>UpdateStatus — 状态机校验 + TxManager"]
        UserRoleSvc["UserService / RoleService"]
        LLMCfgSvc["LLMConfigService<br/>atomic.Value 热替换"]
        DashboardSvc["DashboardService"]
        AuditSvc["AuditService"]
    end

    subgraph RAG["RAG 引擎 rag/"]
        Pipeline["Pipeline.Execute()<br/>QueryRewrite → MultiRoute → HybridRetrieve → Rerank"]
        Processor["Processor.Submit()<br/>goroutine pool 异步文档处理"]
        Chunker["Chunker.Split()<br/>RecursiveCharacterTextSplitter"]
        Embedder["Embedder.Embed()<br/>批量 POST /v1/embeddings"]
        BM25["BM25Retriever<br/>Okapi BM25 + gse 中文分词"]
    end

    subgraph Adapter["适配层 adapter/"]
        LLM["LLMClient 接口<br/>OpenAIClient — ChatCompletion / ChatCompletionStream"]
        EMB["EmbeddingClient 接口<br/>OpenAIEmbeddingClient — CreateEmbeddings"]
        VEC["VectorStore 接口<br/>PgvectorStore — BatchInsert / CosineSearch / DeleteByArticle / DeleteByKB"]
        STO["StorageClient 接口<br/>MinIOClient — Upload / Download"]
    end

    subgraph Infra["基础设施"]
        PG["PostgreSQL 18 + pgvector<br/>业务数据 + halfvec 向量 + HNSW 索引"]
        MinIO["MinIO<br/>Bucket: opsmind-knowledge / opsmind-attachments"]
        LlamaCpp["llama.cpp server (可选)<br/>OpenAI-compat API :8080/v1"]
    end

    Browser --> Router
    Router --> MW
    MW --> Handler
    Handler --> Service
    Service --> RAG
    Service --> Adapter
    RAG --> Adapter
    Adapter --> Infra

    style RAG fill:#5e6ad220,stroke:#5e6ad2
    style Adapter fill:#22c55e20,stroke:#22c55e
    style Infra fill:#f59e0b20,stroke:#f59e0b
```

---

## 2. 请求生命周期

```mermaid
sequenceDiagram
    participant C as 客户端
    participant G as Gin Engine
    participant RID as RequestID
    participant CORS as CORS
    participant LOG as Logger
    participant JWT as JWTAuth
    participant RBAC as RBAC
    participant H as Handler
    participant S as Service
    participant Repo as Repository
    participant DB as PostgreSQL

    C->>G: HTTP Request
    G->>RID: middleware.RequestID() — 注入 X-Request-ID
    RID->>CORS: middleware.CORS() — 跨域头 + OPTIONS 预检
    CORS->>LOG: middleware.Logger() — 记录 method/path/status/latency
    LOG->>JWT: middleware.JWTAuth(secret)

    alt Token 缺失或无效
        JWT-->>C: 401 {code:10001}
    else Token 有效
        JWT->>JWT: c.Set("userID", claims.UserID)
        JWT->>RBAC: middleware.RequirePermission(perm) — admin 路由
        alt 无权限
            RBAC-->>C: 403 {code:10002}
        else 有权限 / portal 路由
            RBAC->>H: handler.Method(c)
            H->>H: c.ShouldBindJSON(&req) → getCurrentUserID(c)
            H->>S: svc.BusinessMethod(req, userID)
            S->>S: 业务规则校验
            S->>Repo: repo.DataAccess()
            Repo->>DB: GORM Query
            DB-->>Repo: Result
            Repo-->>S: Data
            S-->>H: Response
            H->>H: response.Success(c, data)
            H-->>C: 200 {code:0, data:{...}}
        end
    end
```

---

## 3. 模块依赖关系

```mermaid
flowchart LR
    subgraph 入口
        MAIN["cmd/main.go<br/>配置→DB→Repo→Service→Handler→Router→Scheduler"]
    end

    subgraph 核心业务
        AUTH["Auth"] --> USER["User/Role"]
        CHAT["Chat"] --> RAG_ENGINE["RAG Engine"]
        CHAT --> LLM_CFG["LLM Config"]
        KNOWLEDGE["Knowledge"] --> RAG_ENGINE
        KNOWLEDGE --> LLM_CFG
        TICKET["Ticket"] --> MESSAGE["Message"]
        TICKET -.-> KNOWLEDGE
        DASHBOARD["Dashboard"] --> DB_DIRECT["DB (Raw SQL)"]
        AUDIT["Audit"] --> USER
    end

    subgraph 共享
        JWT_LIB["pkg/jwt"]
        HASH_LIB["pkg/hash"]
        ERRCODE["pkg/errcode"]
    end

    MAIN --> AUTH
    MAIN --> CHAT
    MAIN --> KNOWLEDGE
    MAIN --> TICKET
    MAIN --> DASHBOARD
    MAIN --> AUDIT
    AUTH --> JWT_LIB
    AUTH --> HASH_LIB
    CHAT --> ERRCODE
    KNOWLEDGE --> ERRCODE

    style RAG_ENGINE fill:#5e6ad230,stroke:#5e6ad2
    style LLM_CFG fill:#22c55e30,stroke:#22c55e
```

---

## 4. 系统启动流程（main.go → ListenAndServe）

```mermaid
flowchart TB
    Start(["main()"]) --> Cfg["config.Load()<br/>Viper 读取 config.yaml + 环境变量"]

    Cfg --> DB_Init["database.Init(cfg)<br/>└─ gorm.Open(postgres, cfg.DSN)<br/>└─ SetMaxOpenConns(25) / SetMaxIdleConns(10)<br/>└─ SetConnMaxLifetime(5m)"]

    DB_Init --> AutoMigrate["database.AutoMigrate(db)<br/>└─ GORM AutoMigrate 全部 Model<br/>└─ pgvector 扩展自动创建"]

    AutoMigrate --> Adapters["适配层初始化"]
    Adapters --> LLM["NewOpenAIClient(baseURL, apiKey, 120s)"]
    Adapters --> EMB["NewOpenAIEmbeddingClient(baseURL, apiKey, model)"]
    Adapters --> VS["NewPgvectorStore(db) — 复用 GORM 连接池"]
    Adapters --> MIO["NewMinIOClient(endpoint, accessKey, secretKey)"]

    LLM --> RAG
    EMB --> RAG
    VS --> RAG
    MIO --> RAG

    subgraph RAG_Init["rag/ 包"]
        R1["NewDocParser()"]
        R2["NewChunker(chunkSize=1000, overlap=200)"]
        R3["NewEmbedder(embClient, model) — batchSize=32"]
        R4["NewVectorRetriever(embedder, vectorStore)"]
        R5["NewBM25Retriever(bm25Dir) — 延迟加载 + TTL"]
        R6["NewPipeline(vectorRet, bm25Ret, llmClient, embedder, reranker)"]
        R7["NewProcessor(parser, chunker, embedder, store, storage, poolSize=2)<br/>└─ go worker(id) × poolSize"]
    end

    RAG_Init --> Repos["Repository 层: NewUserRepo / NewRoleRepo / NewTicketRepo / ..."]

    Repos --> Services["Service 层: NewAuthService / NewChatService / NewTicketService / ..."]

    Services --> Handlers["Handler 层: NewAuthHandler / NewChatHandler / NewTicketHandler / ..."]

    Handlers --> Router["router.Setup(engine, handlers)"]

    Router --> ConfigWarm["LLMConfigService.LoadDefaults()<br/>└─ DB 加载默认配置 → atomic.Value.Store"]

    ConfigWarm --> Sched["Scheduler 启动"]
    Sched --> SC["NewScheduler(ticketService)<br/>└─ scheduler.Start(ctx)<br/>    └─ go runAutoCloseLoop — 每小时 AutoClose"]

    Sched --> Listen["srv.ListenAndServe(:8080)"]

    style Start fill:#3b82f610,stroke:#3b82f6
    style RAG_Init fill:#5e6ad215,stroke:#5e6ad2
    style Listen fill:#22c55e20,stroke:#22c55e
```

---

## 5. 依赖注入拓扑

```mermaid
flowchart LR
    subgraph Infra["基础设施"]
        DB[("*gorm.DB")]
        Cfg["*config.Config"]
        JWT["jwtSecret"]
    end

    subgraph Adapters["适配层"]
        LLM["*OpenAIClient"]
        EMB["*OpenAIEmbeddingClient"]
        VS["*PgvectorStore"]
        MIO["*MinIOClient"]
    end

    subgraph RAG["RAG 引擎"]
        Pipe["*Pipeline"]
        Proc["*Processor"]
        Chunk["*Chunker"]
        EmbR["*Embedder"]
        BM25["*BM25Retriever"]
    end

    subgraph Repos["Repository"]
        UR["*UserRepo"]
        RR["*RoleRepo"]
        TR["*TicketRepo"]
        KR["*KnowledgeRepo"]
        CR["*ChatRepo"]
    end

    subgraph Services["Service"]
        Auth["*AuthService"]
        User["*UserService"]
        LLMSvc["*LLMService"]
        Chat["*ChatService"]
        Knowledge["*KnowledgeService"]
        Ticket["*TicketService"]
    end

    subgraph Handlers["Handler"]
        AH["*AuthHandler"]
        UH["*UserHandler"]
        CH["*ChatHandler"]
        KH["*KnowledgeHandler"]
        TH["*TicketHandler"]
    end

    DB --> VS
    Cfg --> LLM
    Cfg --> EMB
    Cfg --> MIO
    JWT --> Auth
    LLM --> Pipe
    LLM --> Proc
    EMB --> Pipe
    EMB --> Proc
    VS --> Pipe
    VS --> Proc
    DB --> UR
    DB --> RR
    DB --> TR
    DB --> KR
    DB --> CR
    UR --> Auth
    JWT --> Auth
    UR --> User
    RR --> User
    KR --> Knowledge
    Chunk --> Knowledge
    EmbR --> Knowledge
    VS --> Knowledge
    CR --> Chat
    KR --> Chat
    LLMSvc --> Chat
    TR --> Ticket
    Auth --> AH
    User --> UH
    Chat --> CH
    Knowledge --> KH
    Ticket --> TH

    style Infra fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Adapters fill:#22c55e15,stroke:#22c55e
    style RAG fill:#5e6ad215,stroke:#5e6ad2
    style Repos fill:#22c55e15,stroke:#22c55e
    style Services fill:#f59e0b15,stroke:#f59e0b
    style Handlers fill:#ef444415,stroke:#ef4444
```

---

## 6. 优雅关闭流程

```mermaid
flowchart TB
    Signal["os.Signal: SIGINT / SIGTERM"] --> Ctx["ctx, cancel := context.WithTimeout(30s)"]
    Ctx --> Shutdown["srv.Shutdown(ctx)<br/>└─ 停止接受新请求<br/>└─ 等待现有请求完成"]
    Shutdown --> StopProc["Processor.Stop()<br/>└─ stopped.CompareAndSwap(false, true) — 幂等<br/>└─ cancel() → worker ctx.Done()<br/>└─ close(taskCh)<br/>└─ wg.Wait() — 等待 worker 退出"]
    StopProc --> StopSched["Scheduler.Stop()<br/>└─ cancel() → ticker goroutine 退出"]
    StopSched --> CloseDB["sqlDB.Close()<br/>└─ 关闭 GORM 连接池"]
    CloseDB --> Exit["os.Exit(0)"]

    style Signal fill:#ef444410,stroke:#ef4444
    style Exit fill:#22c55e10,stroke:#22c55e
```

---

> 相关文件：`server/cmd/main.go` / `server/internal/router/router.go` / `server/internal/middleware/`
