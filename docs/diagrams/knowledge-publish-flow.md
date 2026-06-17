# 知识发布管道 — 函数级调用链

> 代码基准：`handler/knowledge.go` → `service/knowledge_service.go` → `rag/chunker.go` / `rag/embedder.go` / `adapter/vector_store.go`
> 更新于 2026-06-17 — 新增 KB 删除流程（§4-5），文章状态机含 Disable→Draft 重启用路径

## 1. 文章生命周期（创建→审核→发布→停用）

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
    KS->>KS: 状态校验: Status == Draft → Status = Reviewing(2)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusReviewing)
    KR->>DB: UPDATE knowledge_articles SET status=2 WHERE id=?

    Note over A,DB: ====== 4. 审核通过 ======
    R->>KH: PUT .../articles/:id/approve
    KH->>KS: Approve(articleID)
    KS->>KS: 状态校验: Status == Reviewing → Status = Published(3)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusPublished)

    Note over A,DB: ====== 5. 驳回 ======
    R->>KH: PUT .../articles/:id/reject
    KH->>KS: Reject(articleID)
    KS->>KS: 状态校验: Status == Reviewing → Status = Rejected(5)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusRejected)

    Note over A,DB: ====== 6. 停用 ======
    A->>KH: PUT .../articles/:id/disable
    KH->>KS: Disable(articleID)
    KS->>KS: Status → ArticleStatusDisabled(4)
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusDisabled)

    Note over A,DB: ====== 7. 重新启用 ======
    A->>KH: PUT .../articles/:id/enable
    KH->>KS: Enable(articleID)
    KS->>KS: 状态校验: Status == ArticleStatusDisabled(4) 才可启用
    KS->>KR: UpdateArticleStatus(articleID, Draft=1)
```

## 2. 发布管道（pgvector 向量写入）

```mermaid
sequenceDiagram
    actor A as 管理员
    participant KH as KnowledgeHandler.Publish<br/>handler/knowledge.go
    participant KS as KnowledgeService.Publish<br/>service/knowledge_service.go:269
    participant KR as KnowledgeRepo<br/>repository/knowledge_repo.go
    participant CH as Chunker.Split<br/>rag/chunker.go:37
    participant EM as Embedder.Embed<br/>rag/embedder.go:56
    participant EC as EmbeddingClient.CreateEmbeddings<br/>adapter/embedding_client.go:106
    participant VS as VectorStore.BatchInsert<br/>adapter/vector_store.go
    participant DB as PostgreSQL(pgvector)

    A->>KH: POST /api/v1/admin/knowledge-bases/:id/articles/:aid/publish
    KH->>KH: parseID(c, "id") + parseID(c, "aid")
    KH->>KS: Publish(articleID, kbID)

    Note over KS: === 1. 获取文章和知识库 ===
    KS->>KR: FindArticleByID(articleID)
    KR->>DB: SELECT * FROM knowledge_articles WHERE id=?
    DB-->>KR: *KnowledgeArticle{Status, Answer}
    KS->>KR: FindKBByID(kbID)
    KR->>DB: SELECT * FROM knowledge_bases WHERE id=?<br/>.Preload("KnowledgeBase")
    DB-->>KR: *KnowledgeBase{EmbeddingModel, VectorDimension}

    Note over KS,DB: === 2. 分块 ===
    KS->>KS: 校验: Status == Published(3) 或 Reviewing(2)
    KS->>KS: embeddingModel = article.KnowledgeBase.EmbeddingModel
    Note over KS: 注意：从 KB 读取，不再硬编码 "bge-m3"
    KS->>CH: Split(article.Answer)
    CH->>CH: 校验 chunkSize 大于 0, 默认 1000
    CH->>CH: 校验 chunkOverlap 大于 0
    CH-->>KS: []string chunks

    Note over KS,DB: === 3. Embedding ===
    KS->>EM: Embed(ctx, chunks)
    EM->>EC: CreateEmbeddings(ctx, EmbeddingRequest{Model: embeddingModel, Input: chunks})
    EC->>EC: doHTTPRequest(ctx, baseURL, apiKey, /v1/embeddings, body)
    Note over EC: 429/503 指数退避重试 (maxRetries=3)
    EC-->>EM: EmbeddingResponse{Embeddings [][]float32, Dimension}
    EM-->>KS: [][]float32 vectors

    Note over KS,DB: === 4. 写入 pgvector ===
    KS->>VS: BatchInsert(ctx, chunkRecords)
    VS->>DB: INSERT INTO knowledge_chunks<br/>(article_id, kb_id, content, chunk_index, embedding)<br/>VALUES ($1, $2, $3, $4, $5::halfvec)
    DB-->>VS: ok

    Note over KS: === 5. 更新文章状态 ===
    KS->>KR: UpdateArticleStatus(articleID, ArticleStatusPublished)
    KR->>DB: UPDATE knowledge_articles SET status=3 WHERE id=?
    KS-->>KH: nil
    KH-->>A: 200
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
