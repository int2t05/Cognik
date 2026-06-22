# 知识发布数据流 — 手动创建 / 文档上传 / 审核 / 发布

> **聚焦：后端数据逻辑。不含前端调用链和用户故事。**

---

## 1. 路由注册

```
router.Setup() → registerAdminRoutes():
  GET    /admin/knowledge-bases              → KnowledgeHandler.ListKBs          [PermKnowledgeRead]
  POST   /admin/knowledge-bases              → KnowledgeHandler.CreateKB         [PermKnowledgeWrite]
  PUT    /admin/knowledge-bases/:id          → KnowledgeHandler.UpdateKB         [PermKnowledgeWrite]
  DELETE /admin/knowledge-bases/:id          → KnowledgeHandler.DeleteKB         [PermKnowledgeWrite]
  GET    /admin/knowledge-bases/:kb_id/articles → KnowledgeHandler.ListArticles  [PermKnowledgeRead]
  POST   /admin/knowledge-bases/:kb_id/articles → KnowledgeHandler.CreateArticle [PermKnowledgeWrite]
  PUT    /admin/articles/:id                 → KnowledgeHandler.UpdateArticle    [PermKnowledgeWrite]
  GET    /admin/articles/:id                 → KnowledgeHandler.GetArticleDetail [PermKnowledgeRead]
  POST   /admin/articles/:id/submit-review   → KnowledgeHandler.SubmitReview     [PermKnowledgeWrite]
  POST   /admin/articles/:id/review          → KnowledgeHandler.Review           [PermKnowledgeReview]
  POST   /admin/articles/:id/publish         → KnowledgeHandler.Publish          [PermKnowledgeReview]
  POST   /admin/articles/:id/disable         → KnowledgeHandler.Disable          [PermKnowledgeReview]
  POST   /admin/articles/:id/enable          → KnowledgeHandler.Enable           [PermKnowledgeReview]
  POST   /admin/knowledge-bases/:kb_id/documents/upload → KnowledgeHandler.UploadDocuments [PermKnowledgeWrite]
  GET    /admin/knowledge-bases/:kb_id/documents/:id/status → KnowledgeHandler.GetDocumentStatus [PermKnowledgeRead]
  POST   /admin/knowledge-bases/:kb_id/documents/:id/retry → KnowledgeHandler.RetryDocument [PermKnowledgeWrite]

registerPortalRoutes():
  GET    /portal/knowledge-bases → KnowledgeHandler.ListKBsForPortal  [仅 JWT，无 RBAC]
```

---

## 2. 文章状态机

```
Draft(1) ── SubmitReview ──→ Reviewing(2) ── Review(approved) ──→ Approved(3)
    │                              │                                       │
    │                              └── Review(rejected) ──→ Rejected(5)    │
    │                                   (驳回需填写理由)          │         │
    │                                                            │         │
    └──────────────────────(可重新编辑提交)─────────────────────┘         │
                                                                          │
Approved(3) ── Publish ──→ Published(4) ── Disable ──→ Disabled(0)       │
                                                             │            │
Disabled(0) ── Enable ──→ (status=3, republish) ──→ Published(4) ────────┘
```

---

## 3. 场景 A：手动创建文章 → 审核 → 发布

### 3.1 创建文章 — `KnowledgeService.CreateArticle(ctx, req, userID)`

```
1. KnowledgeRepo.FindKBByID(ctx, req.KBID) — 校验知识库存在

2. 构建 &KnowledgeArticle{
     KBID:       req.KBID,
     Title:      req.Title,
     Content:    req.Content,
     SourceType: 1 (手动),
     Status:     1 (草稿),
     Category:   req.Category,
     Tags:       marshalTags(req.Tags),   // JSONB, 最多10个, 去重
     CreatedBy:  userID,
   }

3. KnowledgeRepo.CreateArticle(ctx, article)
   → SQL: INSERT INTO knowledge_articles (...) VALUES (...)
```

### 3.2 提交审核 — `KnowledgeService.SubmitReview(ctx, id, userID)`

```
1. KnowledgeRepo.FindArticleByID(ctx, id) — 预加载 KnowledgeBase
2. 状态机: Status != Draft(1) → 拒绝
3. article.Status = Reviewing(2)
4. KnowledgeRepo.UpdateArticle(ctx, article)
```

### 3.3 审核 — `KnowledgeService.Review(ctx, id, reviewerID, approved, comment)`

```
1. KnowledgeRepo.FindArticleByID(ctx, id)
2. 状态机: Status != Reviewing(2) → 拒绝
3. 审核人 ≠ 创建人: article.CreatedBy == reviewerID → 拒绝
4. 驳回必填理由: !approved && comment == "" → 拒绝

5. approved → Status = Approved(3); 否则 Status = Rejected(5), ReviewComment = comment
6. article.ReviewedBy = &reviewerID
7. KnowledgeRepo.UpdateArticle(ctx, article)
8. AuditRepo.Create(ctx, &AuditLog{Action:"knowledge.review", ...})
```

### 3.4 发布 — `KnowledgeService.Publish(ctx, id, publisherID)`

```
1. 组件非空校验: chunker/embedder/store 任一为 nil → ErrRAGUnavailable
2. KnowledgeRepo.FindArticleByID(ctx, id)
3. 状态机: Status != Approved(3) → 拒绝
4. republishFromApproved(ctx, article, publisherID) — 核心管道
```

### 3.5 核心管道: `KnowledgeService.republishFromApproved(ctx, article, publisherID)`

```
Step 1 — 分块 (chunker.Split):
   ├─ ChunkSize=1000, ChunkOverlap=200（clamp 到 chunkSize/2）
   ├─ 分割优先级: \n\n → \n → 。 → . → 空格 → 字符级硬切
   ├─ 预处理: CRLF→LF, 合并空白行, 合并水平空格, 全角→半角 ASCII
   └─ mergeSplits: 合并小片段到接近 chunkSize → 返回 []string

Step 2 — Embedding (embedder.Embed):
   ├─ 批大小=20, fail-fast
   ├─ EmbeddingClient.CreateEmbeddings → HTTP POST /v1/embeddings（重试3次）
   ├─ 维度一致性校验: 所有批次 dimension 必须相等
   └─ 返回 [][]float32, dimension

Step 3 — 批量写入 pgvector (store.BatchInsert) — 先写新向量:
   ├─ SQL: INSERT INTO knowledge_chunks (article_id,kb_id,content,chunk_index,
   │       embedding,embedding_model,vector_dimension,created_at)
   │       VALUES ($1,...,$7::halfvec,NOW()), ...
   └─ float32ToPgVector: NaN/Inf → 0.0 降级

Step 4 — 删除旧向量 (store.DeleteByArticle) — 幂等:
   ├─ SQL: DELETE FROM knowledge_chunks WHERE article_id = $1
   └─ 失败不阻塞发布（旧向量残留优于全部丢失）

Step 5 — 更新文章状态:
   ├─ article.Status = Published(4)
   ├─ article.PublishedBy = &publisherID
   └─ KnowledgeRepo.UpdateArticle(ctx, article)

Step 6 — 审计日志:
   └─ AuditRepo.Create(ctx, &AuditLog{Action:"knowledge.publish", ...})
```

### 3.6 禁用/启用

```
Disable(ctx, id):
  1. FindArticleByID → Status != Published(4) → 拒绝
  2. store.DeleteByArticle(ctx, id) — 删除 pgvector 向量
  3. Status = Disabled(0), UpdateArticle

Enable(ctx, id, publisherID):
  1. FindArticleByID → Status != Disabled(0) → 拒绝
  2. article.Status = Approved(3) — 临时设为已审核（绕过状态校验）
  3. republishFromApproved(ctx, article, publisherID) — 复用发布管道
```

---

## 4. 场景 B：文档上传 → 异步处理

### 4.1 上传 — `KnowledgeService.UploadDocuments(ctx, kbID, userID, filename, fileType, fileSize, reader)`

```
1. KnowledgeRepo.FindKBByID(ctx, kbID) — 校验知识库存在
2. 格式白名单: pdf/docx/md/txt
3. 大小上限: fileSize > 50MB → 拒绝
4. io.ReadAll(io.LimitReader(content, 50MB)) → data; len(data)==0 → 拒绝

5. 分支处理:
   ┌─ MinIO 路径 (storageClient != nil):
   │   a. storageClient.Upload(ctx, bucket, key, reader, size, "") — MinIO PutObject
   │   b. article.MinioPath = "opsmind-documents/documents/<ts>_<filename>"
   │   c. task = ProcessTask{Bucket, Key, FileType, OnStatusChange, OnMetrics}
   │
   └─ 降级路径 (storageClient == nil):
       a. docParser.Parse(reader, fileType) → text
          ├─ .txt/.md: 直接读取
          ├─ .pdf: ledongthuc/pdf 逐页提取（容错单页失败）
          └─ .docx: ZIP 解压 → XML 解析（含 namespace 回退 → 正则）
       b. text == "" → 返回错误
       c. article.Content = text
       d. task = ProcessTask{Content, OnStatusChange, OnMetrics}

6. KnowledgeRepo.CreateArticle(ctx, article) — 插入文章记录
   └─ 若 MinIO 已写但 DB 失败 → storageClient.Delete(bucket, key) 回滚

7. processor.Submit(task) — 非阻塞 channel 发送 (buffer=100)
   └─ channel 满 → "处理队列已满"
   └─ processor 已停止 → "处理器已关闭"
```

### 4.2 异步 Worker: `Processor.processTask(ctx, task)`

```
每个 worker goroutine 循环消费 taskCh:

1. context.WithTimeout(ctx, 10min) — 单任务超时保护
2. processWithRecovery(id, task) — panic recovery 包装

3. 处理管道:
   阶段 1 — 解析 (status=parsing):
     ├─ MinIO: storage.Download → parser.Parse
     └─ 纯文本: 直接用 task.Content
     └─ 失败 → OnStatusChange(articleID, "failed", errMsg)

   阶段 2 — 分块 (status=chunking):
     chunker.Split(content) → []string
     OnMetrics(articleID, wordCount, chunkCount)
       → repo.UpdateArticleMetrics(ctx, id, wordCount, chunkCount)

   阶段 3 — Embedding (status=embedding):
     embedder.Embed(ctx, chunks) → [][]float32
     len(vectors) != len(chunks) → failed

   阶段 4 — 写入 pgvector (status=indexing):
     store.BatchInsert(ctx, vectorChunks)
     成功 → OnStatusChange(articleID, "completed", "")
```

### 4.3 状态回调

```
OnStatusChange → KnowledgeService.onProcessStatusChange()
  → KnowledgeRepo.UpdateArticleProcessStatus(ctx, id, status, errMsg)
  → SQL: UPDATE knowledge_articles SET process_status=?, process_error=? WHERE id=?

OnMetrics → KnowledgeService.onProcessMetrics()
  → KnowledgeRepo.UpdateArticleMetrics(ctx, id, wordCount, chunkCount)
  → SQL: UPDATE knowledge_articles SET word_count=?, chunk_count=? WHERE id=?
```

### 4.4 文档处理状态机

```
pending → parsing → chunking → embedding → indexing → completed
    │        │         │          │           │
    └────────┴─────────┴──────────┴───────────┴──→ failed
                                                     │
                                     POST .../retry ─┘（重新入队 Submit）
```

---

## 5. 知识库 CRUD

### 5.1 创建/更新/删除 KB

```
CreateKB: 生成 workspace slug → INSERT INTO knowledge_bases
UpdateKB: 校验存在 → 更新 name/description/embedding/vectorDimension
DeleteKB:
  1. store.DeleteByKB(ctx, id) — 先删 pgvector 向量
     → SQL: DELETE FROM knowledge_chunks WHERE kb_id = ?
  2. KnowledgeRepo.DeleteKB(ctx, id) — 事务级联删文章 + KB

ListKBs:
  1. ListKBs(ctx) — 按 id ASC
  2. CountArticlesByKB(ctx) — 批量统计（排除已禁用）
     → SQL: SELECT kb_id, COUNT(*) FROM knowledge_articles WHERE status != 0 GROUP BY kb_id
```

---

## 6. 关键组件参数

| 组件 | 关键参数 |
|------|---------|
| Chunker | ChunkSize=1000, ChunkOverlap=200, 分段优先级: \n\n→\n→。→.→空格→硬切 |
| Embedder | BatchSize=20, fail-fast, 维度一致性校验 |
| Processor | PoolSize=2 workers, Channel=100 buffer, 10min timeout/task, panic recovery |
| DocParser | 支持 pdf/docx/md/txt, 最大 100MB |
| pgvector | halfvec 类型, HNSW 索引, NaN/Inf→0.0, 先写新向量后删旧向量 |
