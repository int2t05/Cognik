# 知识文章发布流 — 从已审核到向量写入

---

## 用户故事

| 角色 | 目标 | 价值 |
|------|------|------|
| 知识库管理员 | 手动编写运维知识文章，经审核后发布到向量库 | 沉淀团队经验，提升 AI 回答质量 |
| 知识库管理员 | 上传 PDF/DOCX/MD/TXT 文档，系统自动解析分块入库 | 批量导入存量知识文档 |
| 审核人 | 审核待发布文章，通过或驳回并附理由 | 保证知识库内容质量 |
| 系统 | 发布时自动完成分块→embedding→pgvector 写入全流程 | 无需人工干预向量化过程 |

---

## 前端调用链路

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           前端组件 → API 映射                                   │
├────────────────────┬───────────────────────────┬──────────────────────────────┤
│        页面         │          组件              │          API 调用             │
├────────────────────┼───────────────────────────┼──────────────────────────────┤
│ /admin/knowledge   │ KnowledgePage             │ GET /admin/knowledge-bases   │
│ (KB 列表)           │  ├─ AppleCard 卡片列表     │  → getKBList()               │
│                    │  ├─ AppleDialog 新建 KB    │ POST /admin/knowledge-bases  │
│                    │  │   → createKB()          │  → createKB(data)            │
│                    │  ├─ AppleDialog 编辑 KB    │ PUT /admin/knowledge-bases   │
│                    │  │   → updateKB()          │   /:id                       │
│                    │  └─ 删除按钮               │ DELETE /admin/knowledge-bases│
│                    │      → deleteKB()          │   /:id                       │
│                    │                           │                              │
│ /admin/knowledge   │ ArticleListPage           │ GET /admin/knowledge-bases   │
│ /[kbId]            │  ├─ AppleTable 文章列表    │   /:kbId/articles            │
│ (文章列表)          │  │   → getArticleList()   │  → apiFetchPage<Article>     │
│                    │  ├─ 状态筛选 pill 按钮      │   ?status=draft              │
│                    │  └─ 点击行 → 跳转编辑页     │                              │
│                    │                           │                              │
│ /admin/knowledge   │ NewArticlePage            │                              │
│ /[kbId]/new        │  ├─ AppleInput +           │ POST /admin/knowledge-bases  │
│ (新建文章)          │  │   AppleTextarea 表单    │   /:kbId/articles            │
│                    │  │   → createArticle()     │  → createArticle(kbId, data) │
│                    │  │                         │                              │
│                    │  └─ 文件上传区              │ POST /admin/knowledge-bases  │
│                    │      <input type="file">   │   /:kbId/documents/upload    │
│                    │      → uploadDocuments()   │  → FormData 上传（fetch 直连）│
│                    │      前端校验 50MB/file.size│                              │
│                    │                           │                              │
│ /admin/knowledge   │ ArticleEditPage           │ GET /admin/articles/:id      │
│ /[kbId]/[articleId]│  ├─ 文章内容编辑表单        │  → getArticle()              │
│ (编辑/审核/发布)    │  │   → updateArticle()     │ PUT /admin/articles/:id      │
│                    │  │                         │                              │
│                    │  ├─ "提交审核" 按钮          │ POST /admin/articles/:id     │
│                    │  │   → submitReview()      │   /submit-review             │
│                    │  │                         │                              │
│                    │  ├─ "通过"/"驳回" 按钮       │ POST /admin/articles/:id     │
│                    │  │   → reviewArticle()     │   /review                    │
│                    │  │   驳回需填写理由          │  {approved, review_comment}  │
│                    │  │                         │                              │
│                    │  ├─ "发布" 按钮              │ POST /admin/articles/:id     │
│                    │  │   → publishArticle()    │   /publish                   │
│                    │  │   (仅已审核状态可用)      │                              │
│                    │  │                         │                              │
│                    │  ├─ "禁用"/"启用" 按钮       │ POST /admin/articles/:id     │
│                    │  │   → disableArticle()    │   /disable or /enable        │
│                    │  │                         │                              │
│                    │  └─ 处理状态轮询             │ GET /admin/knowledge-bases   │
│                    │      (上传文档后自动刷新)     │   /:kbId/documents/:id/status│
│                    │                           │  → getDocStatus()             │
└────────────────────┴───────────────────────────┴──────────────────────────────┘
```

---

## 场景 A：手动创建文章 → 审核 → 发布

> 文章从「草稿」到「已发布 + pgvector 有向量」的完整状态机路径。

### 输入
```
POST /api/v1/admin/articles/:id/publish
Authorization: Bearer <admin_jwt>
```

### 分层数据流

#### 0. 路由 & 中间件

1. `router.Setup()` → `registerAdminRoutes()` 注册路由：
   - `POST /articles/:id/publish` → `middleware.RequirePermission(PermKnowledgeReview)` → `KnowledgeHandler.Publish`
2. `middleware.JWTAuth(userCache, jwtSecret)` — JWT 解析 + 用户状态校验
3. `middleware.RequirePermission("knowledge:review")` — RBAC 权限检查

#### 接入层 — Handler

4. 经由 `KnowledgeHandler.Publish(c)` 处理：
   - `parseID(c, "id")` 解析文章 ID
   - `getCurrentUserID(c)` 提取操作人 ID
   - `h.svc.Publish(c.Request.Context(), id, userID)`

#### 业务层 — Service

5. 经由 `KnowledgeService.Publish(ctx, id, publisherID)` 处理：
   - 管道组件非空校验：`chunker == nil || embedder == nil || store == nil` → `ErrRAGUnavailable`
   - `repo.FindArticleByID(ctx, id)` — 加载文章
   - 状态机校验：`article.Status != ArticleStatusApproved(3)` → 拒绝
   - `republishFromApproved(ctx, article, publisherID)` — 核心发布管道

6. 经由 `KnowledgeService.republishFromApproved(ctx, article, publisherID)` 处理：

   **Step 6.1 — 分块**
   - `chunker.Split(content)` — Chunker.Split（固定长度 1000 字符 + 200 字符重叠）
     - 按段落边界切分，避免在句子中间切断
     - 产出 `[]string` 分块列表

   **Step 6.2 — Embedding**
   - `embedder.Embed(ctx, chunks)` — Embedder.Embed（内部调用 `EmbeddingClient.Embeddings()`）
     - HTTP POST → `/v1/embeddings` (OpenAI-compatible API)
     - 返回 `[][]float32` 向量列表
     - 向量数与分块数不匹配 → `recordPublishFailure(ctx, article, msg)` 设置 process_status=failed
     - 失败直接返回 `ErrRAGUnavailable`

   **Step 6.3 — 批量写入 pgvector**
   - 构建 `[]adapter.VectorChunk` 数据
   - `store.BatchInsert(ctx, vc)` — PgvectorStore.BatchInsert
     - SQL: `INSERT INTO knowledge_chunks (article_id, kb_id, content, chunk_index, embedding, embedding_model, vector_dimension, created_at) VALUES (...)`
     - 每个 chunk 生成 `($1,...,$7)::halfvec` 占位符
     - `float32ToPgVector(v)` 转换向量格式
     - 维度一致性校验（所有 chunk 维度必须相等）
     - 失败 → `recordPublishFailure(ctx, article, "写入向量失败")`

   **Step 6.4 — 删除旧向量**
   - `store.DeleteByArticle(ctx, id)` — 幂等清除旧向量（新向量已写入）
     - SQL: `DELETE FROM knowledge_chunks WHERE article_id = $1`
     - 失败不阻塞发布（旧向量残留可被后续清理，优于全部丢失）

   **Step 6.5 — 更新文章状态**
   - `article.Status = ArticleStatusPublished(4)`
   - `article.PublishedBy = &publisherID`
   - `repo.UpdateArticle(ctx, article)`

   **Step 6.6 — 审计日志**
   - `auditRepo.Create(ctx, &AuditLog{OperatorID, Action:"knowledge.publish", TargetType:"knowledge_article", TargetID})`

#### 数据层

7. `KnowledgeRepo.FindArticleByID(ctx, id)` — 预加载 KnowledgeBase 关联
8. `KnowledgeRepo.UpdateArticle(ctx, article)` — 更新文章状态字段
9. `AuditRepo.Create(ctx, log)` — 写入 audit_logs 表

### 输出
```json
{ "code": 0, "message": "success", "data": null }
```

### 文章状态机

```
Draft(1) ──┐
           ├─ SubmitReview ──→ Reviewing(2) ─┬─ Review(approved) ─→ Approved(3)
           │                                 │
           │                                 └─ Review(rejected) ──→ Rejected(6)
           │                                                            │
           └────────────────────────(编辑后重新)─────────────────────┘

Approved(3) ── Publish ──→ Published(4) ── Disable ──→ Disabled(5)
                                                           │
Disabled(5) ── Enable ──→ Approved(3) ── Publish ──→ Published(4)
```

---

## 场景 B：文档上传 → 异步处理

### 输入
```
POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload
Content-Type: multipart/form-data
字段: files（最多 10 个，支持 pdf/docx/md/txt）
```

### 分层数据流

#### 接入层 — Handler

1. `KnowledgeHandler.UploadDocuments(c)`
   - `parseID(c, "kb_id")` — 解析知识库 ID
   - `c.Request.ParseMultipartForm(32 << 20)` — 32MB 解析限制
   - `c.Request.MultipartForm.File["files"]` — 提取文件列表
   - `len(files) > 10` — 数量上限检查
   - 对每个文件：
     - `sniffFileType(fh)` — MIME 嗅探 + 扩展名判断文件类型
       - `http.DetectContentType(sniff[:512])` — 读取前 512 字节嗅探
       - 支持白名单：pdf/docx/md/txt
     - `h.svc.UploadDocuments(ctx, kbID, userID, filename, fileType, fileSize, reader)`

#### 业务层 — Service

2. `KnowledgeService.UploadDocuments(ctx, kbID, userID, filename, fileType, fileSize, content)`

   **Step 2.1 — 校验**
   - `repo.FindKBByID(ctx, kbID)` — 知识库存在性校验
   - `allowedDocumentTypes[fileType]` — 格式白名单校验
   - `fileSize > MaxDocumentSize(50MB)` — 大小上限校验
   - `io.ReadAll(io.LimitReader(content, MaxDocumentSize))` — 读取全部内容
   - `len(data) == 0` — 空文件校验

   **Step 2.2 — 分支：MinIO 路径 vs 降级路径**

   **MinIO 路径** (storageClient != nil)：
   - `storage.Upload(ctx, "opsmind-documents", key, bytes.NewReader(data), size, "")`
     - `MinIOClient.Upload()` — PUT 对象到 MinIO
   - 设置 `article.MinioPath = "opsmind-documents/documents/<timestamp>_<filename>"`
   - 构建 `rag.ProcessTask{Bucket, Key, FileType, OnStatusChange, OnMetrics}`

   **降级路径** (storageClient == nil)：
   - `docParser.Parse(bytes.NewReader(data), fileType)` — 同步解析文本
     - PDF → 使用 pdf 库提取文本
     - DOCX → 解压 zip 读取 word/document.xml
     - MD/TXT → 直接读取
   - `strings.TrimSpace(text) == ""` → 返回错误
   - 设置 `article.Content = text`
   - 构建 `rag.ProcessTask{Content, OnStatusChange, OnMetrics}`

   **Step 2.3 — 持久化文章**
   - `repo.CreateArticle(ctx, article)` — INSERT INTO knowledge_articles
     - 若失败且 MinIO 已写入：回滚删除 `storage.Delete(ctx, bucket, key)`

   **Step 2.4 — 入队异步处理**
   - `processor.Submit(task)` — Processor.Submit（非阻塞 channel 发送）
     - taskCh 已满 → 返回错误 "处理队列已满"
     - processor 已停止 → 返回错误 "处理器已关闭"

#### RAG 引擎层 — 异步 Worker

3. `Processor.worker(id)` goroutine 循环消费 taskCh：
   - `processWithRecovery(id, task)` — panic recovery 包装
   - `context.WithTimeout(ctx, 10min)` — 单任务超时保护
   - `Processor.processTask(ctx, task)` — 核心处理管道：

     **阶段 1 — 下载/解析** (status=parsing)
     - MinIO 路径：`storage.Download(ctx, bucket, key)` → 下载文件 → `parser.Parse(reader, fileType)` 解析
     - 纯文本路径：直接使用 task.Content
     - 失败 → `OnStatusChange(articleID, "failed", errorMsg)`

     **阶段 2 — 分块** (status=chunking)
     - `chunker.Split(content)` — 与发布管道相同的分块逻辑
     - `OnMetrics(articleID, wordCount, chunkCount)` — 回调更新指标

     **阶段 3 — Embedding** (status=embedding)
     - `embedder.Embed(ctx, chunks)` — 生成向量
     - 向量数 ≠ 分块数 → failed

     **阶段 4 — 写入 pgvector** (status=indexing)
     - `store.BatchInsert(ctx, vectorChunks)` — 批量写入
     - 成功后 → `OnStatusChange(articleID, "completed", "")`

4. 状态回调链路：
   - `OnStatusChange` → `KnowledgeService.onProcessStatusChange()` → `repo.UpdateArticleProcessStatus(ctx, aID, status, errMsg)`
   - `OnMetrics` → `KnowledgeService.onProcessMetrics()` → `repo.UpdateArticleMetrics(ctx, aID, wordCount, chunkCount)`

### 输出
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "documents": [
      {
        "article_id": 42,
        "file_name": "运维手册.pdf",
        "file_size": 1024000,
        "file_type": "pdf",
        "process_status": "pending"
      }
    ]
  }
}
```

> 后续通过 `GET /knowledge-bases/:kb_id/documents/:id/status` 轮询处理状态。

### 文档处理状态机

```
pending ──→ parsing ──→ chunking ──→ embedding ──→ indexing ──→ completed
    │           │           │            │            │
    └───────────┴───────────┴────────────┴────────────┴──→ failed
                                                              │
                                              POST .../retry ─┘（重试重新入队）
```

---

## 场景 C：文章启用 (Disabled → Published)

### 输入
```
POST /api/v1/admin/articles/:id/enable
```

### 分层数据流

1. `KnowledgeHandler.Enable(c)` → `KnowledgeService.Enable(ctx, id, publisherID)`
2. 状态机校验：`article.Status != ArticleStatusDisabled` → 拒绝
3. 绕开 Publish 的状态校验，直接调用：
   - `article.Status = ArticleStatusApproved`（虚拟审核通过）
   - `republishFromApproved(ctx, article, publisherID)` — 与 Publish 共用管道

### 输出
```json
{ "code": 0, "message": "success", "data": null }
```
