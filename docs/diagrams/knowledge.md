# 知识管理

> 覆盖 KB CRUD、文章状态机、手动创建与文档上传双路径入库、发布管道、删除级联。

---

## 1. 知识入库双路径（手动创建 vs 文档上传）

```mermaid
flowchart TB
    subgraph Path_A["路径 A: 手动创建 → 审核 → 发布"]
        A1["KnowledgeHandler.CreateArticle<br/>c.ShouldBindJSON → getCurrentUserID"]
        A2["KnowledgeService.CreateArticle(req, userID)<br/>└─ KnowledgeRepo.FindKBByID — 校验 KB 存在<br/>└─ CreateArticle(&KnowledgeArticle{Status:Draft(1)})<br/>   → INSERT INTO knowledge_articles"]
        A3["SubmitReview → Status: Draft(1)→Reviewing(2)"]
        A4["Review(approved=true) → 审核人≠创建人<br/>Status: Reviewing(2)→Approved(3)"]
        A5["Publish → 发布管道 (§3)"]
    end

    subgraph Path_B["路径 B: 文档上传 → 异步处理 → 发布"]
        B1["KnowledgeHandler.UploadDocuments<br/>c.FormFile('file') → file.Open()"]
        B2["KnowledgeService.UploadDocuments(kbID, userID, filename, fileType, fileSize, src)<br/>└─ 格式白名单: pdf/docx/md/txt<br/>└─ 大小上限: 50MB<br/>└─ DocParser.Parse(content, fileType)<br/>    io.LimitReader(100MB) → parsePDF/parseDocx/parseTxt<br/>└─ CreateArticle(Status:Draft, SourceType:upload)<br/>└─ strings.TrimSpace(text) — 空内容检查"]
        B3["Processor.Submit(task{ArticleID, KBID})<br/>rag/processor.go:106<br/>└─ stopped? → err<br/>└─ select → ch ← task (非阻塞)"]
        B4["Processor.worker(id) → processTask(ctx, task)<br/>rag/processor.go:149<br/>└─ context.WithTimeout(10min)<br/>└─ processWithRecovery — panic 恢复<br/>① parsing → ② chunking → ③ embedding → ④ indexing → ⑤ completed"]
    end

    A1 --> A2 --> A3 --> A4 --> A5
    B1 --> B2 --> B3 --> B4
    B4 -.->|OnStatusChange 回调| DONE["process_status=completed<br/>Article.Status 仍由人工流转"]

    style Path_A fill:#5e6ad210,stroke:#5e6ad2
    style Path_B fill:#f59e0b10,stroke:#f59e0b
    style A5 fill:#22c55e15,stroke:#22c55e
```

---

## 2. 文章状态机

```mermaid
stateDiagram-v2
    [*] --> Draft : CreateArticle()

    state 人工流转 {
        Draft --> Reviewing : SubmitReview()
        Reviewing --> Approved : Review(approved=true)
        Reviewing --> Rejected : Review(approved=false)
        Approved --> Published : Publish()<br/>分块→embedding→pgvector
        Published --> Disabled : Disable()<br/>校验: Published(4) 才允许
        Disabled --> Published : Enable()<br/>重跑发布管道
    }

    state 处理进度_独立 {
        [*] --> pending : UploadDocuments()
        pending --> parsing : Processor.worker()
        parsing --> chunking
        chunking --> embedding
        embedding --> indexing
        indexing --> completed
        pending --> failed : 任一阶段出错
        parsing --> failed
        chunking --> failed
        embedding --> failed
        indexing --> failed
        failed --> pending : RetryDocument()
    }

    note right of 人工流转
        Article.Status 字段
        Enable 重跑完整 Publish 管道:
        Chunker.Split→Embedder.Embed→
        VectorStore.BatchInsert→DeleteByArticle
    end note

    note right of 处理进度_独立
        Article.ProcessStatus 字段
        与 Status 互不污染
    end note
```

---

## 3. 发布管道（Publish / Enable 共用）

```mermaid
flowchart LR
    subgraph Entry["入口"]
        E1["Publish(articleID)<br/>状态校验: Approved(3)"]
        E2["Enable(articleID)<br/>状态校验: Disabled(0)"]
    end

    subgraph Pipeline["KnowledgeService.Publish / republishFromApproved"]
        P0["ctx 传递: c.Request.Context()"]
        P1["Chunker.Split(content)<br/>rag/chunker.go<br/>RecursiveCharacterTextSplitter (1000/200)<br/>输入: string → 输出: []string chunks"]
        P2["Embedder.Embed(ctx, chunks)<br/>rag/embedder.go<br/>EmbeddingClient.CreateEmbeddings POST /v1/embeddings<br/>输入: []string chunks → 输出: [][]float32 vectors"]
        P3["VectorStore.BatchInsert(ctx, chunkRecords)<br/>adapter/vector_store.go<br/>INSERT INTO knowledge_chunks (embedding::halfvec)<br/>维度一致性校验 → float32ToPgVector"]
        P4["VectorStore.DeleteByArticle(ctx, id)<br/>DELETE FROM knowledge_chunks WHERE article_id=?<br/>先写后删策略: 新向量写入成功后才删旧"]
        P5["KnowledgeRepo.UpdateArticle<br/>Status=Published(4)"]
    end

    subgraph Output["输出"]
        O1["200 — 文章可被 RAG 检索"]
    end

    E1 --> P0
    E2 --> P0
    P0 --> P1 --> P2 --> P3 --> P4 --> P5 --> O1

    style P1 fill:#f59e0b10,stroke:#f59e0b
    style P2 fill:#5e6ad210,stroke:#5e6ad2
    style P3 fill:#22c55e10,stroke:#22c55e
    style P4 fill:#ef444410,stroke:#ef4444
```

---

## 4. 发布管道 — 详细时序

```mermaid
sequenceDiagram
    actor A as 管理员
    participant KH as KnowledgeHandler.Publish
    participant KS as KnowledgeService.Publish
    participant CH as Chunker.Split
    participant EM as Embedder.Embed
    participant EC as EmbeddingClient.CreateEmbeddings
    participant VS as VectorStore
    participant DB as PostgreSQL+pgvector

    A->>KH: POST /admin/articles/:id/publish
    KH->>KH: parseID → getCurrentUserID
    KH->>KS: Publish(ctx, articleID, userID)

    KS->>KS: chunker/embedder/store == nil? → ErrRAGUnavailable(20002)
    KS->>KS: FindArticleByID → 状态: Approved(3) 才可发布

    KS->>CH: Split(article.Content)
    CH->>CH: RecursiveCharacterTextSplitter(chunkSize=1000, overlap=200)
    CH-->>KS: []string chunks

    KS->>EM: Embed(ctx, chunks)
    EM->>EC: CreateEmbeddings(ctx, {Model, Input:chunks})
    EC-->>EM: EmbeddingResponse{Embeddings, Dimension}
    EM-->>KS: [][]float32 vectors

    alt Embed 失败
        KS->>KS: process_status=failed — 持久化失败信息
    end

    KS->>VS: BatchInsert(ctx, chunkRecords)
    VS->>DB: INSERT INTO knowledge_chunks (embedding::halfvec)
    alt BatchInsert 失败
        KS->>KS: process_status=failed — 旧向量仍在
    end

    KS->>VS: DeleteByArticle(ctx, articleID)
    alt 删除失败
        Note over KS: slog.Warn — 新向量已生效，旧向量残留后续清理
    end

    KS->>DB: UPDATE knowledge_articles SET status=4
    KS-->>KH: nil
    KH-->>A: 200
```

---

## 5. 知识库 CRUD 主干

```mermaid
flowchart TB
    subgraph Create["创建 KB"]
        C1["POST /admin/knowledge-bases<br/>{name, description, embedding_model}"]
        C2["KnowledgeService.CreateKB(req, userID)<br/>→ KnowledgeRepo.CreateKB"]
        C3[("INSERT INTO knowledge_bases")]
    end

    subgraph Update["更新 KB"]
        U1["PUT /admin/knowledge-bases/:id"]
        U2["KnowledgeService.UpdateKB(id, req)<br/>→ FindKBByID → Update"]
        U3[("UPDATE knowledge_bases")]
    end

    subgraph Delete["删除 KB"]
        D1["DELETE /admin/knowledge-bases/:id"]
        D2["KnowledgeService.DeleteKB(id)<br/>→ FindKBByID — 存在性校验<br/>→ VectorStore.DeleteByKB(ctx, kbID) — 删除向量<br/>→ KnowledgeRepo.DeleteKB(id) — 事务级联"]
        D3[("DELETE knowledge_chunks WHERE kb_id=?<br/>BEGIN;DELETE articles;DELETE kb;COMMIT")]
    end

    C1 --> C2 --> C3
    U1 --> U2 --> U3
    D1 --> D2 --> D3

    style Create fill:#22c55e10,stroke:#22c55e
    style Update fill:#f59e0b10,stroke:#f59e0b
    style Delete fill:#ef444410,stroke:#ef4444
```

---

## 6. KB 删除决策流程

```mermaid
flowchart TD
    Start(["DELETE /admin/knowledge-bases/:id"]) --> Auth{"JWTAuth + RBAC?"}
    Auth -->|否| E401["401/403"]
    Auth -->|是| Parse["parseID → kbID"]

    Parse --> FindKB{"KnowledgeRepo.FindKBByID<br/>KB 存在?"}
    FindKB -->|否| E404["404 AppError{10004}"]

    FindKB -->|是| ChkStore{"VectorStore != nil?"}
    ChkStore -->|是| DelVec["VectorStore.DeleteByKB(ctx, kbID)<br/>DELETE FROM knowledge_chunks WHERE kb_id=?"]
    ChkStore -->|否| DelDB

    DelVec -->|成功| DelDB["KnowledgeRepo.DeleteKB(kbID)<br/>BEGIN; DELETE articles; DELETE kb; COMMIT"]
    DelVec -->|失败| Warn["slog.Warn — 不阻塞 DB 删除"]
    Warn --> DelDB

    DelDB -->|成功| OK["200"]
    DelDB -->|失败| E500["500"]

    style E401 fill:#ef444420,stroke:#ef4444
    style E404 fill:#ef444420,stroke:#ef4444
    style E500 fill:#ef444420,stroke:#ef4444
    style OK fill:#22c55e20,stroke:#22c55e
```

---

## 7. 文档异步处理 — goroutine pool

```mermaid
flowchart TB
    subgraph Sync["同步阶段（HTTP 请求内）"]
        S1["UploadDocuments → DocParser.Parse<br/>io.LimitReader(100MB) → 格式分发<br/>输出: string text"]
        S2["CreateArticle → INSERT<br/>输出: articleID"]
        S3["Processor.Submit(task)<br/>stopped? → err / select → ch ← task<br/>返回 200（后台处理）"]
    end

    subgraph Async["异步阶段（goroutine pool）"]
        A1["worker(id): for task := range ch<br/>processWithRecovery → defer recover()"]
        A2["processTask(ctx, task)<br/>context.WithTimeout(10min)<br/>各阶段间检查 ctx.Err()"]
        A3["① parsing: DocParser.Parse (MinIO Download 或 Content)"]
        A4["② chunking: Chunker.Split → RecursiveCharacterTextSplitter"]
        A5["③ embedding: Embedder.Embed → EmbeddingClient.CreateEmbeddings<br/>校验: len(vectors)==len(chunks)"]
        A6["④ indexing: VectorStore.BatchInsert → embedding::halfvec"]
        A7["⑤ OnStatusChange(articleID, 'completed', '')"]
    end

    S1 --> S2 --> S3
    S3 -.-> A1 --> A2 --> A3 --> A4 --> A5 --> A6 --> A7

    style Sync fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Async fill:#5e6ad210,stroke:#5e6ad2
```

---

> 相关文件：`server/internal/handler/knowledge.go` / `server/internal/service/knowledge_service.go` / `server/internal/rag/chunker.go` / `server/internal/rag/embedder.go` / `server/internal/rag/processor.go` / `server/internal/rag/document_parser.go` / `server/internal/adapter/vector_store.go` / `server/internal/adapter/embedding_client.go`
