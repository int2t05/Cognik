# 知识发布管道 — 函数级调用链

> 代码基准：`handler/knowledge.go` → `service/knowledge_service.go` → `rag/chunker.go` / `rag/embedder.go` / `adapter/vector_store.go`
> 更新于 2026-06-17 — 文章状态机重构：
>
> - Disable 仅允许 `Published(4) → Disabled(0)`，拒绝其他状态直接 Disable
> - Enable 直接 `Disabled(0) → Published(4)`，**复用发布管道**（不再走 Draft）
> - 状态机与 process_status 解耦：文档处理进度不再污染 Article.Status

## 1. 文章生命周期（创建→审核→发布→停用→重新启用）

**审核状态机**（`Article.Status` 字段，由人工操作流转）：

```mermaid
stateDiagram-v2
    [*] --> Draft_1 : CreateArticle()
    Draft_1 --> Reviewing_2 : SubmitReview()
    Reviewing_2 --> Approved_3 : Review(approved=true)
    Reviewing_2 --> Rejected_5 : Review(approved=false)
    Approved_3 --> Published_4 : Publish()<br/>(分块→embedding→pgvector)
    Published_4 --> Disabled_0 : Disable()<br/>(校验 Published 才允许)
    Disabled_0 --> Published_4 : Enable()<br/>(重跑发布管道，不走 Draft)

    note right of Disabled_0
        Enable 是 Publish 管道同款路径：
        chunker.Split → embedder.Embed →
        store.BatchInsert → store.DeleteByArticle
        → Status=Published(4)
    end note

    note left of Published_4
        停用后向量已删除，
        启用必须重建向量。
    end note
```

**文档处理状态机**（`Article.ProcessStatus` 字段，与 Status 互不污染）：

```mermaid
stateDiagram-v2
    [*] --> pending : UploadDocuments()
    pending --> parsing : Processor 启动
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

    note right of completed
        Article.Status 仍由人工流转，
        ProcessStatus 仅反映处理进度。
    end note
```

**完整生命周期序列图：**

```mermaid
sequenceDiagram
    actor A as 知识库管理员
    actor R as 审核人
    participant KH as KnowledgeHandler<br/>handler/knowledge.go
    participant KS as KnowledgeService<br/>service/knowledge_service.go
    participant KR as KnowledgeRepo<br/>repository/knowledge_repo.go
    participant DB as PostgreSQL

    Note over A,DB: ====== 1. 创建知识库 ======
    A->>KH: POST /api/v1/admin/knowledge-bases<br/>{name, description}
    KH->>KH: getCurrentUserID(c) → (userID, bool)
    KH->>KS: CreateKB(req, userID)
    KS->>KR: CreateKB(&KnowledgeBase{Name, Description, CreatedBy})
    KR->>DB: INSERT INTO knowledge_bases
    DB-->>KR: kb.ID
    KH-->>A: 200

    Note over A,DB: ====== 2. 创建文章 (草稿) ======
    A->>KH: POST /api/v1/admin/knowledge-bases/:kb_id/articles<br/>{question, answer, category}
    KH->>KH: parseID(c, "kb_id")
    KH->>KS: CreateArticle(req, userID)

    KS->>KR: FindKBByID(kbID) → 校验知识库存在
    KR->>DB: SELECT FROM knowledge_bases WHERE id=?
    KS->>KR: CreateArticle(&KnowledgeArticle{<br/>KBID, Question, Answer, Status: Draft=1})
    KR->>DB: INSERT INTO knowledge_articles
    KS-->>KH: *KnowledgeArticle

    Note over A,DB: ====== 3. 提交审核 ======
    A->>KH: PUT .../articles/:id/submit
    KH->>KS: SubmitForReview(articleID)
    KS->>KS: 状态校验: Status == Draft(1) → Status = Reviewing(2)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusReviewing)
    KR->>DB: UPDATE knowledge_articles SET status=2 WHERE id=?

    Note over A,DB: ====== 4. 审核通过 ======
    R->>KH: PUT .../articles/:id/approve
    KH->>KS: Approve(articleID)
    KS->>KS: 状态校验: Status == Reviewing(2) → Status = Approved(3)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusApproved)

    Note over A,DB: ====== 5. 驳回 ======
    R->>KH: PUT .../articles/:id/reject
    KH->>KS: Reject(articleID)
    KS->>KS: 状态校验: Status == Reviewing(2) → Status = Rejected(5)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusRejected)

    Note over A,DB: ====== 6. 停用 ======
    A->>KH: PUT .../articles/:id/disable
    KH->>KS: Disable(articleID)
    KS->>KS: 校验 Status == Published(4) 才允许<br/>其他状态返回 code=10003
    KS->>KR: DeleteByArticle(id)<br/>(清空 pgvector)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusDisabled=0)

    Note over A,DB: ====== 7. 重新启用（重跑发布管道） ======
    A->>KH: PUT .../articles/:id/enable
    KH->>KS: Enable(ctx, articleID, userID)
    KS->>KS: 校验 Status == Disabled(0) 才允许
    KS->>KS: republishFromApproved(article, userID)<br/>= Publish 管道同款路径<br/>(分块→embedding→pgvector)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusPublished=4)
```

## 2. 发布管道（pgvector 向量写入）

> 更新于 2026-06-17 — ctx 传递、先写后删、失败记录 process_status

```mermaid
sequenceDiagram
    actor A as 管理员
    participant KH as KnowledgeHandler.Publish<br/>handler/knowledge.go
    participant KS as KnowledgeService.Publish<br/>service/knowledge_service.go:290
    participant KR as KnowledgeRepo<br/>repository/knowledge_repo.go
    participant CH as Chunker.Split<br/>rag/chunker.go:37
    participant EM as Embedder.Embed<br/>rag/embedder.go:56
    participant EC as EmbeddingClient.CreateEmbeddings<br/>adapter/embedding_client.go:106
    participant VS as VectorStore<br/>adapter/vector_store.go
    participant DB as PostgreSQL(pgvector)

    A->>KH: POST /api/v1/admin/articles/:id/publish
    KH->>KH: parseID(c, "id") → getCurrentUserID(c)
    KH->>KS: Publish(c.Request.Context(), articleID, userID)

    Note over KS: === 0. 管道非空校验 ===
    KS->>KS: chunker/embedder/store == nil?<br/>→ ErrRAGUnavailable(20002)

    Note over KS,DB: === 1. 获取文章和知识库 ===
    KS->>KR: FindArticleByID(articleID)
    KR->>DB: SELECT * FROM knowledge_articles WHERE id=?
    DB-->>KR: *KnowledgeArticle{Status, Content}
    KS->>KS: 状态校验: 仅 ArticleStatusApproved(3) 可发布

    Note over KS,DB: === 2. 分块 ===
    KS->>CH: Split(article.Content)
    CH->>CH: RecursiveCharacterTextSplitter(chunkSize=1000, overlap=200)
    CH-->>KS: []string chunks

    Note over KS,DB: === 3. Embedding ===
    KS->>EM: Embed(ctx, chunks)
    EM->>EC: CreateEmbeddings(ctx, {Model: embeddingModel, Input: chunks})
    EC-->>EM: EmbeddingResponse{Embeddings, Dimension}
    EM-->>KS: [][]float32 vectors
    alt Embed 失败
        KS->>KR: UpdateArticleProcessStatus(id, "failed", error)
        Note over KS,KR: process_status=failed 持久化，前端可展示重试
    end

    Note over KS,DB: === 4. 先写入新向量（BatchInsert）===
    KS->>VS: BatchInsert(ctx, chunkRecords)
    VS->>DB: INSERT INTO knowledge_chunks (embedding::halfvec)
    alt BatchInsert 失败
        KS->>KR: UpdateArticleProcessStatus(id, "failed", error)
        Note over KS: 旧向量仍在（未被删除），文章仍可检索
    end

    Note over KS,DB: === 5. 新向量写入成功后删除旧向量 ===
    KS->>VS: DeleteByArticle(ctx, articleID)
    alt 删除失败
        Note over KS: slog.Warn（新向量已生效，旧向量残留可后续清理）
    end

    Note over KS: === 6. 更新文章状态 ===
    KS->>KR: UpdateArticle(&KnowledgeArticle{Status: Published=4})
    KR->>DB: UPDATE knowledge_articles SET status=4
    KS-->>KH: nil
    KH-->>A: 200 {code:0}
```

## 3. 文章状态机

```mermaid
stateDiagram-v2
    [*] --> Draft : CreateArticle()

    Draft --> Reviewing : SubmitForReview()
    Reviewing --> Published : Approve()
    Reviewing --> Rejected : Reject()
    Published --> Disabled : Disable()<br/>(status = ArticleStatusDisabled=0)
    Disabled --> Draft : Enable()<br/>(校验 Status==ArticleStatusDisabled(0) 才可启用)

    note right of Published
        Publish() 执行分块→Embedding→pgvector
        embeddingModel 从 KB.EmbeddingModel 读取
    end note
```

## 4. 知识库删除流程（🆕 2026-06-17）

```mermaid
sequenceDiagram
    actor A as 管理员
    participant KH as KnowledgeHandler.DeleteKB<br/>handler/knowledge.go
    participant KS as KnowledgeService.DeleteKB<br/>service/knowledge_service.go:129
    participant KR as KnowledgeRepo<br/>repository/knowledge_repo.go
    participant VS as VectorStore.DeleteByKB<br/>adapter/vector_store.go:234
    participant DB as PostgreSQL+pgvector

    A->>KH: DELETE /api/v1/admin/knowledge-bases/:id
    KH->>KH: parseID(c, "id")
    KH->>KS: DeleteKB(id)

    Note over KS: === 1. 存在性校验 ===
    KS->>KR: FindKBByID(id)
    KR->>DB: SELECT * FROM knowledge_bases WHERE id=?
    alt 不存在
        KR-->>KS: gorm.ErrRecordNotFound
        KS-->>KH: AppError{10004, "知识库不存在"}
        KH-->>A: 404
    end

    Note over KS,DB: === 2. 删除 pgvector 向量分块 ===
    KS->>VS: DeleteByKB(ctx, kbID)
    VS->>DB: DELETE FROM knowledge_chunks WHERE kb_id=?
    Note over VS: 幂等操作——无分块也不报错

    Note over KS,DB: === 3. 级联删除文章+KB（事务） ===
    KS->>KR: DeleteKB(id)
    KR->>DB: BEGIN
    KR->>DB: DELETE FROM knowledge_articles WHERE kb_id=?
    KR->>DB: DELETE FROM knowledge_bases WHERE id=?
    KR->>DB: COMMIT

    KS-->>KH: nil
    KH-->>A: 200 {code:0}
```

## 5. KB 删除决策流程图

```mermaid
flowchart TD
    Start([DeleteKB 请求]) --> Auth{JWTAuth<br/>+ RBAC?}
    Auth -->|NO| E401[401/403]
    Auth -->|OK| Parse[parseID → kbID]
    Parse --> FindKB{FindKBByID<br/>KB 存在?}
    FindKB -->|NO| E404["404<br/>AppError{10004, '知识库不存在'}"]
    FindKB -->|OK| DelVec{store != nil?}
    DelVec -->|YES| VecDel[VectorStore.DeleteByKB<br/>DELETE knowledge_chunks<br/>WHERE kb_id=?]
    DelVec -->|NO| DelDB
    VecDel -->|OK| DelDB[KnowledgeRepo.DeleteKB]
    VecDel -->|fail| Warn["slog.Warn<br/>向量删除失败<br/>不阻塞DB删除"]
    Warn --> DelDB
    DelDB --> TX[事务: DELETE articles<br/>+ DELETE knowledge_bases]
    TX -->|OK| OK["200 {code:0}"]
    TX -->|fail| E500[500 数据库错误]

    style E401 fill:#ef444420,stroke:#ef4444
    style E404 fill:#ef444420,stroke:#ef4444
    style E500 fill:#ef444420,stroke:#ef4444
    style OK fill:#22c55e20,stroke:#22c55e
```
