# Knowledge 数据流 — 每个 API 端点

> 涉及文件: `domain/knowledge/handler.go`, `domain/knowledge/service.go`, `domain/knowledge/repository.go`, `domain/knowledge/frontmatter.go`, `domain/knowledge/metadata_completer.go`, `domain/knowledge/index_builder.go`, `domain/system/audit/repository.go`, `rag/frontmatter.go`, `rag/chunker.go`, `rag/embedder.go`, `rag/processor.go`, `parser/parser.go`, `infra/adapter/vector_store.go`, `infra/storage/storage.go`, `shared/model/knowledge.go`, `shared/model/audit.go`

---

## 知识库 CRUD

### GET /api/v1/admin/knowledge-bases &emsp; 全部 KB &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.ListKBs (domain/knowledge/handler.go:123)
  → KnowledgeService.ListKBs (domain/knowledge/service.go:225)
    ├─ KnowledgeRepo.ListKBs (domain/knowledge/repository.go:46) → SELECT ... ORDER BY id ASC
    └─ KnowledgeRepo.CountArticlesByKB (domain/knowledge/repository.go:58)
        → SELECT kb_id, COUNT(*) FROM knowledge_articles WHERE status!=0 GROUP BY kb_id
```

### GET /api/v1/portal/knowledge-bases &emsp; 门户 KB 列表 &emsp; [仅 JWT]

```
KnowledgeHandler.ListKBsForPortal (domain/knowledge/handler.go:42)
  → KnowledgeService.ListKBs (同上) → 仅返回 id/name/description
```

### POST /api/v1/admin/knowledge-bases &emsp; 创建 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.CreateKB (domain/knowledge/handler.go:64)
  → KnowledgeService.CreateKB (domain/knowledge/service.go:158)
    ├─ 生成 workspace slug
    └─ KnowledgeRepo.CreateKB (domain/knowledge/repository.go:29)
        → INSERT INTO knowledge_bases (...)
```

### PUT /api/v1/admin/knowledge-bases/:id &emsp; 更新 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.UpdateKB (domain/knowledge/handler.go:83)
  → KnowledgeService.UpdateKB (domain/knowledge/service.go:177)
    ├─ KnowledgeRepo.FindKBByID → 校验存在
    └─ KnowledgeRepo.UpdateKB → 更新 name/description/embedding/vectorDimension
```

### GET /api/v1/admin/knowledge-bases/:kb_id/articles &emsp; 文章列表 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.ListArticles (domain/knowledge/handler.go:284)
  → KnowledgeService.ListArticles (domain/knowledge/service.go:541)
    └─ KnowledgeRepo.ListArticles (domain/knowledge/repository.go:100)
        → SELECT COUNT(*) + SELECT * ... WHERE kb_id=? [AND status=?] [AND source_type=?]
          ORDER BY updated_at DESC LIMIT ? OFFSET ? (Preload KnowledgeBase)
```

### GET /api/v1/admin/articles/:id &emsp; 文章详情 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.GetArticleDetail (domain/knowledge/handler.go:307)
  → KnowledgeService.GetArticleDetail (domain/knowledge/service.go:588)
    ├─ KnowledgeRepo.FindArticleByID (domain/knowledge/repository.go:87)
    │   → SELECT * FROM knowledge_articles WHERE id=? (Preload KnowledgeBase)
    └─ UserRepo.FindByIDs → 批量查询创建人/审核人姓名
```

### DELETE /api/v1/admin/knowledge-bases/:id &emsp; 删除 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.DeleteKB (domain/knowledge/handler.go:106)
  → KnowledgeService.DeleteKB (domain/knowledge/service.go:202)
    ├─ PgvectorStore.DeleteByKB (infra/adapter/vector_store.go:230)
    │   → DELETE FROM knowledge_chunks WHERE kb_id = ?  (先清向量)
    └─ KnowledgeRepo.DeleteKB (domain/knowledge/repository.go:155)
        → 事务: DELETE articles → DELETE kb
```

---

## 文章 CRUD + 审核 + 发布

### POST /api/v1/admin/knowledge-bases/:kb_id/articles &emsp; 创建文章 &emsp; [PermKnowledgeWrite]

**输入** `{"title":"VPN配置","content":"...# 步骤1...","category":"网络","tags":["VPN"]}`

```
KnowledgeHandler.CreateArticle (domain/knowledge/handler.go:140)
  → KnowledgeService.CreateArticle (domain/knowledge/service.go:264)
    ├─ KnowledgeRepo.FindKBByID → 校验知识库
    ├─ marshalTags(tags) → JSONB, 最多10个, 去重
    └─ KnowledgeRepo.CreateArticle (domain/knowledge/repository.go:83)
        → INSERT INTO knowledge_articles (status=1 draft)
```

### PUT /api/v1/admin/articles/:id &emsp; 编辑 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.UpdateArticle → KnowledgeService.UpdateArticle (domain/knowledge/service.go:293)
  ├─ KnowledgeRepo.FindArticleByID → 仅 draft/rejected 可编辑
  └─ KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/submit-review &emsp; 提交审核 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.SubmitReview → KnowledgeService.SubmitReview (domain/knowledge/service.go:312)
  ├─ KnowledgeRepo.FindArticleByID
  ├─ Status != Draft(1) → 拒绝
  └─ article.Status = Reviewing(2) → KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/review &emsp; 审核 &emsp; [PermKnowledgeReview]

**输入** `{"approved":true, "review_comment":""}`

```
KnowledgeHandler.Review → KnowledgeService.Review (domain/knowledge/service.go:327)
  ├─ Status != Reviewing(2) → 拒绝
  ├─ 审核人 ≠ 创建人 → 防止自审
  ├─ 驳回需填写 review_comment
  ├─ approved → Status=Approved(3) / rejected → Status=Rejected(5)
  ├─ KnowledgeRepo.UpdateArticle
  └─ AuditRepo.Create (domain/system/audit/repository.go:50) → "knowledge.review"
```

### POST /api/v1/admin/articles/:id/publish &emsp; 发布 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Publish (domain/knowledge/handler.go:230)
  → KnowledgeService.Publish (domain/knowledge/service.go:518)
    ├─ Status != Approved(3) → 拒绝
    └─ republishFromApproved (核心管道):
        ├─ Step 0: ParseArticleMeta → 解析 frontmatter (domain/knowledge/frontmatter.go)
        │   → 若 article_type 缺失/非法 → MetadataCompleter.Complete (LLM 推断 type/tags)
        │   → 补全触发时 → CreateSystemTicket（【元数据复核】工单, source=3）
        │   → LLM 失败降级 guide
        │   → RenderArticleFile 生成含 frontmatter 的 .md 写入存储
        ├─ Step 1: StripFrontmatter (rag/frontmatter.go) → 剥离 frontmatter，仅对 # 标题 + 正文分块
        │   → Chunker.Split (rag/chunker.go:56), chunkSize=500, overlap=100
        ├─ Step 2: Embedder.Embed (rag/embedder.go:57)
        │   → batchSize=20, fail-fast → POST /v1/embeddings, 维度 1536
        ├─ Step 3: PgvectorStore.BatchInsert (infra/adapter/vector_store.go:115)
        │   → INSERT INTO knowledge_chunks (embedding::halfvec(dim)) VALUES ...
        │   → NaN/Inf → 0.0; 先写新向量
        ├─ Step 4: PgvectorStore.DeleteByArticle (infra/adapter/vector_store.go:220)
        │   → DELETE FROM knowledge_chunks WHERE article_id = ? (幂等, 后删旧)
        ├─ Step 5: article.Status = Published(4) → KnowledgeRepo.UpdateArticle
        ├─ Step 6: onKBChanged 回调
        │   → RebuildBM25ForKB（BM25 索引重建）
        │   → RebuildKBIndex（INDEX.md 页目录重建, per-kbID 锁 + dirty-flag）
        └─ Step 7: AuditRepo.Create → "knowledge.publish"
```

### POST /api/v1/admin/articles/:id/disable &emsp; 禁用 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Disable → KnowledgeService.Disable (domain/knowledge/service.go:483)
  ├─ Status != Published(4) → 拒绝
  ├─ PgvectorStore.DeleteByArticle → 删除 pgvector 向量
  └─ Status=Disabled(0) → KnowledgeRepo.UpdateArticle
```

### POST /api/v1/admin/articles/:id/enable &emsp; 启用 &emsp; [PermKnowledgeReview]

```
KnowledgeHandler.Enable → KnowledgeService.Enable (domain/knowledge/service.go:520)
  ├─ Status != Disabled(0) → 拒绝
  ├─ article.Status = Approved(3) (临时, 绕过发布状态校验)
  └─ republishFromApproved → 复用完整发布管道
```

---

## 文档上传 + 异步处理

### POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload &emsp; 上传 &emsp; [PermKnowledgeWrite]

**输入** `multipart/form-data files: [技术手册.pdf, FAQ.docx]`

```
KnowledgeHandler.UploadDocuments (domain/knowledge/handler.go:329)
  ├─ parseID("kb_id"), c.Request.ParseMultipartForm(32MB)
  ├─ sniffFileType → http.DetectContentType(前512字节)
  └─ for each file:
      → KnowledgeService.UploadDocuments (domain/knowledge/service.go:748)
        ├─ KnowledgeRepo.FindKBByID → 校验
        ├─ io.ReadAll(LimitReader(content, 50MB)) → 读取全部内容
        ├─ DocParser.Parse (parser/parser.go:61) — txt/md 纯文本直接本地解析，富文档走 MinerU
        ├─ KnowledgeRepo.CreateArticle → INSERT; article_type 留空
        ├─ CreateSystemTicket（【文档复核】工单, source=3, 关联 article_id + kb_id）
        └─ storageClient.UploadFile → 写入 draft .md 文件
```

> 上传创建草稿文章 + 自动创建 `source=3` 复核工单。分块/embedding 推迟到 Publish 阶段。

### GET /api/v1/admin/knowledge-bases/:kb_id/documents/:id/status &emsp; 状态查询 &emsp; [PermKnowledgeRead]

```
KnowledgeHandler.GetDocumentStatus (domain/knowledge/handler.go:435)
  → KnowledgeService.GetDocumentStatus (domain/knowledge/service.go:755)
    └─ 校验 article.KBID == kbID → 返回 process_status/process_error
```

### POST /api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry &emsp; 重试 &emsp; [PermKnowledgeWrite]

```
KnowledgeHandler.RetryDocument (domain/knowledge/handler.go:457)
  → KnowledgeService.RetryDocument (domain/knowledge/service.go:776)
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
人工路径：Draft(1) → submit-review → Reviewing(2) → approve → Approved(3) → publish → Published(4)
                                              → reject → Rejected(5)
Agent 路径：Draft(1) → CreateAndPublish → Published(4)（绕过审核，语义去重 + auto-publish）
Published(4) → disable → Disabled(0) → enable → (republish) → Published(4)
Published(4) → UpdateAndRepublish → (增量 reindex) → Published(4)
```

## Agent 自迭代闭环

Agent 写回知识库时自动发布进 RAG，无需人工审核：

### CreateAndPublish（kb(action=create)）

```
KnowledgeService.CreateAndPublish (domain/knowledge/service.go:690)
  ├─ CreateArticle（标题去重 + 写 DB Draft + 写 draft/ 文件）
  ├─ checkSemanticDuplicate（embed 标题+首段 → CosineSearch > 0.92 拒绝）
  │   → 重复则删除 Draft + 提示 agent 用 update
  └─ republishFromApproved（metadata 补全 → chunk → embed → pgvector → BM25+INDEX.md 重建）
```

### UpdateAndRepublish（kb(action=update)）

```
KnowledgeService.UpdateAndRepublish (domain/knowledge/service.go:730)
  ├─ 更新正文（保持 Published 状态，不回退）
  ├─ 重传 draft/ 正文文件
  └─ republishFromApproved（增量 reindex，复用 SHA256 跳过未变 chunk）
```

> 防滥用：标题精确去重 + 语义去重（>0.92 拒绝）+ SourceTypeDeepResearch 系统标记（published_by=0 便于回滚）。人工路径（CreateArticle→Review→Publish）不变。

## 关键组件参数

| 组件 | 文件 | 关键参数 |
|------|------|---------|
| Chunker | `rag/chunker.go` | size=500, overlap=100, Markdown-aware 递归分割 |
| Embedder | `rag/embedder.go` | batchSize=20, fail-fast, 维度一致性校验 |
| Processor | `rag/processor.go` | pool=5, buffer=100, timeout=5min, panic recovery |
| DocParser | `parser/parser.go` | pdf/docx/md/txt, max 100MB |
