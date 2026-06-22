# Knowledge 数据流 — 每个 API 端点

> 涉及文件: `handler/knowledge.go`, `service/knowledge_service.go`, `repository/knowledge_repo.go`, `repository/audit_repo.go`, `rag/chunker.go`, `rag/embedder.go`, `rag/processor.go`, `rag/document_parser.go`, `adapter/vector_store.go`, `adapter/storage_client.go`, `model/knowledge.go`, `model/audit.go`

---

## 知识库 CRUD

### GET /api/v1/admin/knowledge-bases &emsp; 全部 KB &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.ListKBs (handler/knowledge.go:123)
  → KnowledgeService.ListKBs (service/knowledge_service.go:225)
    ├─ KnowledgeRepo.ListKBs (repository/knowledge_repo.go:46) → SELECT ... ORDER BY id ASC
    └─ KnowledgeRepo.CountArticlesByKB (repository/knowledge_repo.go:58)
        → SELECT kb_id, COUNT(*) FROM knowledge_articles WHERE status!=0 GROUP BY kb_id
```

### GET /api/v1/portal/knowledge-bases &emsp; 门户 KB 列表 &emsp; [仅 JWT]

```
KnowledgeHandler.ListKBsForPortal (handler/knowledge.go:42)
  → KnowledgeService.ListKBs (同上) → 仅返回 id/name/description
```

### POST /api/v1/admin/knowledge-bases &emsp; 创建 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.CreateKB (handler/knowledge.go:64)
  → KnowledgeService.CreateKB (service/knowledge_service.go:158)
    ├─ 生成 workspace slug
    └─ KnowledgeRepo.CreateKB (repository/knowledge_repo.go:29)
        → INSERT INTO knowledge_bases (...)
```

### PUT /api/v1/admin/knowledge-bases/:id &emsp; 更新 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.UpdateKB (handler/knowledge.go:83)
  → KnowledgeService.UpdateKB (service/knowledge_service.go:177)
    ├─ KnowledgeRepo.FindKBByID → 校验存在
    └─ KnowledgeRepo.UpdateKB → 更新 name/description/embedding/vectorDimension
```

### GET /api/v1/admin/knowledge-bases/:kb_id/articles &emsp; 文章列表 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.ListArticles (handler/knowledge.go:284)
  → KnowledgeService.ListArticles (service/knowledge_service.go:541)
    └─ KnowledgeRepo.ListArticles (repository/knowledge_repo.go:100)
        → SELECT COUNT(*) + SELECT * ... WHERE kb_id=? [AND status=?] [AND source_type=?]
          ORDER BY updated_at DESC LIMIT ? OFFSET ? (Preload KnowledgeBase)
```

### GET /api/v1/admin/articles/:id &emsp; 文章详情 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.GetArticleDetail (handler/knowledge.go:307)
  → KnowledgeService.GetArticleDetail (service/knowledge_service.go:588)
    ├─ KnowledgeRepo.FindArticleByID (repository/knowledge_repo.go:87)
    │   → SELECT * FROM knowledge_articles WHERE id=? (Preload KnowledgeBase)
    └─ UserRepo.FindByIDs → 批量查询创建人/审核人姓名
```

### DELETE /api/v1/admin/knowledge-bases/:id &emsp; 删除 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.DeleteKB (handler/knowledge.go:106)
  → KnowledgeService.DeleteKB (service/knowledge_service.go:202)
    ├─ PgvectorStore.DeleteByKB (adapter/vector_store.go:230)
    │   → DELETE FROM knowledge_chunks WHERE kb_id = ?  (先清向量)
    └─ KnowledgeRepo.DeleteKB (repository/knowledge_repo.go:155)
        → 事务: DELETE articles → DELETE kb
```

---

## 文章 CRUD + 审核 + 发布

### POST /api/v1/admin/knowledge-bases/:kb_id/articles &emsp; 创建文章 &emsp; [PermKnowledgeWrite]

**输入** `{"title":"VPN配置","content":"...# 步骤1...","category":"网络","tags":["VPN"]}`

```
KnowledgeHandler.CreateArticle (handler/knowledge.go:140)
  → KnowledgeService.CreateArticle (service/knowledge_service.go:264)
    ├─ KnowledgeRepo.FindKBByID → 校验知识库
    ├─ marshalTags(tags) → JSONB, 最多10个, 去重
    └─ KnowledgeRepo.CreateArticle (repository/knowledge_repo.go:83)
        → INSERT INTO knowledge_articles (status=1 draft)
```

### PUT /api/v1/admin/articles/:id &emsp; 编辑 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.UpdateArticle → KnowledgeService.UpdateArticle (service/knowledge_service.go:293)
  ├─ KnowledgeRepo.FindArticleByID → 仅 draft/rejected 可编辑
  └─ KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/submit-review &emsp; 提交审核 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.SubmitReview → KnowledgeService.SubmitReview (service/knowledge_service.go:312)
  ├─ KnowledgeRepo.FindArticleByID
  ├─ Status != Draft(1) → 拒绝
  └─ article.Status = Reviewing(2) → KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/review &emsp; 审核 &emsp; [PermKnowledgeReview]

**输入** `{"approved":true, "review_comment":""}`

```
KnowledgeHandler.Review → KnowledgeService.Review (service/knowledge_service.go:327)
  ├─ Status != Reviewing(2) → 拒绝
  ├─ 审核人 ≠ 创建人 → 防止自审
  ├─ 驳回需填写 review_comment
  ├─ approved → Status=Approved(3) / rejected → Status=Rejected(5)
  ├─ KnowledgeRepo.UpdateArticle
  └─ AuditRepo.Create (repository/audit_repo.go:50) → "knowledge.review"
```

### POST /api/v1/admin/articles/:id/publish &emsp; 发布 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Publish (handler/knowledge.go:230)
  → KnowledgeService.Publish (service/knowledge_service.go:367)
    ├─ Status != Approved(3) → 拒绝
    └─ republishFromApproved (核心管道):
        ├─ Step 1: Chunker.Split (rag/chunker.go:56)
        │   → chunkSize=1000, overlap=200, 优先级 \n\n→\n→。→.→空格
        │   → 预处理: CRLF→LF, 全角→半角 ASCII
        ├─ Step 2: Embedder.Embed (rag/embedder.go:57)
        │   → batchSize=20, fail-fast → POST /v1/embeddings
        │   → 维度一致性校验
        ├─ Step 3: PgvectorStore.BatchInsert (adapter/vector_store.go:115)
        │   → INSERT INTO knowledge_chunks (embedding::halfvec) VALUES ...
        │   → NaN/Inf → 0.0; 先写新向量
        ├─ Step 4: PgvectorStore.DeleteByArticle (adapter/vector_store.go:220)
        │   → DELETE FROM knowledge_chunks WHERE article_id = ? (幂等, 后删旧)
        ├─ Step 5: article.Status = Published(4)
        │   → KnowledgeRepo.UpdateArticle
        └─ Step 6: AuditRepo.Create → "knowledge.publish"
```

### POST /api/v1/admin/articles/:id/disable &emsp; 禁用 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Disable → KnowledgeService.Disable (service/knowledge_service.go:483)
  ├─ Status != Published(4) → 拒绝
  ├─ PgvectorStore.DeleteByArticle → 删除 pgvector 向量
  └─ Status=Disabled(0) → KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/enable &emsp; 启用 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Enable → KnowledgeService.Enable (service/knowledge_service.go:520)
  ├─ Status != Disabled(0) → 拒绝
  ├─ article.Status = Approved(3) (临时, 绕过发布状态校验)
  └─ republishFromApproved → 复用完整发布管道
```

---

## 文档上传 + 异步处理

### POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload &emsp; 上传 &emsp; [PermKnowledgeWrite]

**输入** `multipart/form-data files: [运维手册.pdf, FAQ.docx]`

```
KnowledgeHandler.UploadDocuments (handler/knowledge.go:329)
  ├─ parseID("kb_id"), c.Request.ParseMultipartForm(32MB)
  ├─ sniffFileType → http.DetectContentType(前512字节)
  └─ for each file:
      → KnowledgeService.UploadDocuments (service/knowledge_service.go:656)
        ├─ KnowledgeRepo.FindKBByID → 校验
        ├─ io.ReadAll(LimitReader(content, 50MB)) → 读取全部内容
        ├─ 分支:
        │   ├─ storageClient != nil: MinIOClient.Upload (adapter/storage_client.go:102)
        │   │   → PUT object → article.MinioPath = "documents/<ts>_<name>"
        │   │   → task = ProcessTask{Bucket, Key, FileType}
        │   └─ storageClient == nil: DocParser.Parse (rag/document_parser.go:45)
        │       → pdf: ledongthuc/pdf 逐页提取 / docx: ZIP → XML 解析
        │       → article.Content = text; task = ProcessTask{Content}
        ├─ KnowledgeRepo.CreateArticle → INSERT; MinIO失败则回滚
        └─ Processor.Submit (rag/processor.go:106)
            → 非阻塞 channel 发送 (buffer=100) → worker 异步处理
```

### GET /api/v1/admin/knowledge-bases/:kb_id/documents/:id/status &emsp; 状态查询 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.GetDocumentStatus (handler/knowledge.go:435)
  → KnowledgeService.GetDocumentStatus (service/knowledge_service.go:755)
    └─ 校验 article.KBID == kbID → 返回 process_status/process_error
```

### POST /api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry &emsp; 重试 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.RetryDocument (handler/knowledge.go:457)
  → KnowledgeService.RetryDocument (service/knowledge_service.go:776)
    ├─ process_status != "failed" → 拒绝
    └─ Processor.Submit → 重新入队
```

### 异步 Worker: Processor.processTask

```
Processor worker goroutine (rag/processor.go):
  ├─ context.WithTimeout(10min), panic recovery
  ├─ Stage 1: DocParser.Parse → 解析文本
  ├─ Stage 2: Chunker.Split → 分块
  ├─ Stage 3: Embedder.Embed → 向量化
  └─ Stage 4: PgvectorStore.BatchInsert → 写入 pgvector

回调:
  OnStatusChange → KnowledgeRepo.UpdateArticleProcessStatus
  OnMetrics → KnowledgeRepo.UpdateArticleMetrics (word_count, chunk_count)
```

---

## 状态机速查

```
Draft(1) → submit-review → Reviewing(2) → approve → Approved(3) → publish → Published(4)
                                           → reject → Rejected(5)
Published(4) → disable → Disabled(0) → enable → (republish) → Published(4)
```

## 关键组件参数

| 组件 | 文件 | 关键参数 |
|------|------|---------|
| Chunker | `rag/chunker.go` | size=1000, overlap=200, 优先级 \n\n→。→.→空格 |
| Embedder | `rag/embedder.go` | batchSize=20, fail-fast, 维度一致性校验 |
| Processor | `rag/processor.go` | pool=2, buffer=100, timeout=10min, panic recovery |
| DocParser | `rag/document_parser.go` | pdf/docx/md/txt, max 100MB |
