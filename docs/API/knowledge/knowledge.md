# 知识库管理接口

> **Base URL:** `/api/v1/admin` + `/api/v1/portal` | **Auth:** Admin: JWT + RBAC, Portal: JWT only

## 知识文章状态

**审核状态机**（`status` 字段，人工操作流转）：

```
草稿(1) → 待审核(2) → 已通过(3) → 已发布(4) → 已停用(0)
            ↓
          已驳回(5)
```

| 编号 | 枚举值                   | 含义   |
| ---- | ------------------------ | ------ |
| 0    | `ArticleStatusDisabled`  | 已停用 |
| 1    | `ArticleStatusDraft`     | 草稿   |
| 2    | `ArticleStatusReviewing` | 待审核 |
| 3    | `ArticleStatusApproved`  | 已通过 |
| 4    | `ArticleStatusPublished` | 已发布 |
| 5    | `ArticleStatusRejected`  | 已驳回 |

合法转换：

- `Draft → Reviewing`：提交审核（`POST /articles/:id/submit-review`）
- `Reviewing → Approved` / `Rejected`：审核（`POST /articles/:id/review`）
- `Approved → Published`：发布（`POST /articles/:id/publish`）
- `Published → Disabled`：停用（`POST /articles/:id/disable`）
- `Disabled → Published`：启用（`POST /articles/:id/enable`，内部重跑分块→embedding→pgvector）

其他转换（含 `Draft → Disabled` 等）一律拒绝，返回 `code=10003`。

**文档处理状态机**（`process_status` 字段，独立于 `status`，仅 `source_type=upload` 时相关）：

```
pending → parsing → chunking → embedding → indexing → completed
   ↓
failed → retry → pending
```

| 取值        | 含义                                       |
| ----------- | ------------------------------------------ |
| `pending`   | 已创建文档，尚未开始处理                   |
| `parsing`   | 解析文档（PDF/DOCX/MD/TXT）                |
| `chunking`  | 文本分块                                   |
| `embedding` | 调用 Embedding API 生成向量                |
| `indexing`  | 写入 pgvector                              |
| `completed` | 处理完成，可参与 RAG 检索                  |
| `failed`    | 处理失败，`process_error` 记录原因，可重试 |
| `disabled`  | 文章被停用，不再参与 RAG 检索              |

> **架构约定**：两个状态机互不污染。`status` 仅承载人工审核流转，`process_status` 仅承载文档处理进度。前端展示"已驳回"的文章不应再显示"chunking"。

## 知识库 CRUD

### 0. 门户端知识库列表

> 门户端接口（无需 admin 权限），用于问答页面选择目标知识库。

```http
GET /api/v1/portal/knowledge-bases
Authorization: Bearer <token>
```

**响应：**

```json
{ "code": 0, "message": "success", "data": [{ "id": 1, "name": "IT 知识 FAQ", "description": "常见的 IT 问题和解决方案" }] }
```

> 仅返回 `id`、`name`、`description`，不暴露 embedding 配置等管理字段。

### 1. 知识库列表

```http
GET /api/v1/admin/knowledge-bases
Authorization: Bearer <token>
```

**响应：**

```json
{
    "code": 0,
    "message": "success",
    "data": [
        {
            "id": 1,
            "name": "技术知识库",
            "description": "技术标准操作流程",
            "embedding_model": "Qwen3-Embedding-0.6B",
            "vector_dimension": 1536,
            "article_count": 120,
            "created_by": 1,
            "created_at": "2026-06-11 19:27:43",
            "updated_at": "2026-06-11 19:27:43"
        }
    ]
}
```

| 字段             | 类型   | 说明                      |
| ---------------- | ------ | ------------------------- |
| id               | int64  | 知识库 ID                 |
| name             | string | 知识库名称                |
| description      | string | 描述                      |
| embedding_model  | string | 使用的 embedding 模型名称 |
| vector_dimension | int    | 向量维度                  |
| article_count    | int    | 文章数量                  |
| created_by       | int64  | 创建者用户 ID              |
| created_at       | string | 创建时间（`YYYY-MM-DD HH:MM:SS` 格式） |
| updated_at       | string | 更新时间（`YYYY-MM-DD HH:MM:SS` 格式） |

### 2. 创建知识库

```http
POST /api/v1/admin/knowledge-bases
Authorization: Bearer <token>
```

**请求体：**

```json
{
    "name": "网络知识 FAQ",
    "description": "网络相关的技术知识",
    "embedding_model": "Qwen3-Embedding-0.6B",
    "vector_dimension": 1536
}
```

| 字段             | 类型   | 必填 | 说明                                     |
| ---------------- | ------ | ---- | ---------------------------------------- |
| name             | string | ✓    | 知识库名称                               |
| description      | string |      | 描述                                     |
| embedding_model  | string | ✓    | embedding 模型名称 |
| vector_dimension | int    | ✓    | 向量维度           |

> 知识库创建仅写入 PostgreSQL 记录，向量存储通过 pgvector 表 `knowledge_chunks` 管理。

**响应：**

```json
{ "code": 0, "message": "success", "data": { "id": 2 } }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                             |
| ------ | --------- | -------------------------------- |
| 10003  | 400       | 参数校验失败（名称/模型必填）     |
| 10005  | 409       | 同名知识库已存在                 |

### 3. 更新知识库

```http
PUT /api/v1/admin/knowledge-bases/:id
Authorization: Bearer <token>
```

**请求体：**

```json
{ "name": "网络技术知识库", "description": "更新后的描述" }
```

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10003  | 400       | 参数校验失败             |
| 10004  | 404       | 知识库不存在             |

---

## 知识文章 CRUD

### 4. 文章列表

```http
GET /api/v1/admin/knowledge-bases/:kb_id/articles?page=1&page_size=10&status=-1
Authorization: Bearer <token>
```

**查询参数：**

| 参数          | 类型   | 默认 | 说明                                                  |
| ------------- | ------ | ---- | ----------------------------------------------------- |
| page          | int    | 1    | 页码                                                  |
| page_size     | int    | 10   | 每页条数（最大 100）                                  |
| status        | int    | -1   | 按状态筛选（-1=全部）                                 |
| source_type   | int    | 0    | 按来源筛选（0=全部, 1=手动, 2=上传）                  |
| process_status | string | —    | 按处理状态筛选（pending/parsing/chunking/embedding/indexing/completed/failed/disabled） |

**响应：**

```json
{
    "code": 0,
    "message": "success",
    "data": [
        {
            "id": 1,
            "kb_id": 1,
            "kb_name": "IT 知识 FAQ",
            "title": "如何重置 VPN 密码？",
            "content": "请登录 VPN 自助服务平台...",
            "source_type": 1,
            "source_type_text": "手动输入",
            "category": "网络与VPN",
            "tags": ["VPN", "密码", "自助"],
            "status": 4,
            "status_text": "已发布",
            "word_count": 156,
            "chunk_count": 2,
            "file_type": "pdf",
            "process_status": "completed",
            "process_error": "",
            "created_by": 1,
            "created_by_name": "admin",
            "created_at": "2026-06-11T19:27:43Z",
            "updated_at": "2026-06-11T19:27:43Z"
        },
        {
            "id": 2,
            "kb_id": 1,
            "kb_name": "IT 知识 FAQ",
            "title": "账号冻结处理流程.pdf",
            "content": "一、账号冻结场景说明...（解析后的文本全文）",
            "source_type": 2,
            "source_type_text": "文档上传",
            "category": "",
            "tags": [],
            "status": 1,
            "status_text": "草稿",
            "word_count": 3200,
            "chunk_count": 0,
            "file_type": "pdf",
            "process_status": "parsing",
            "process_error": "",
            "created_by": 1,
            "created_by_name": "admin",
            "created_at": "2026-06-11T20:00:00Z",
            "updated_at": "2026-06-11T20:00:00Z"
        }
    ],
    "total": 5,
    "page": 1,
    "page_size": 10
}
```

| 字段           | 类型   | 说明                                                              |
| -------------- | ------ | ----------------------------------------------------------------- |
| title          | string | 文章标题（手动输入时填写，文档上传时取文件名）                    |
| content        | string | 文章正文全文                                                      |
| source_type    | int    | 1=手动输入, 2=文档上传                                            |
| word_count     | int    | 正文字数                                                          |
| chunk_count    | int    | 分块数（发布后填充）                                              |
| file_type      | string | 文档类型：pdf/docx/md/txt，仅 source_type=2                       |
| process_status | string | 文档处理状态：pending/parsing/chunking/embedding/indexing/completed/failed/disabled |
| process_error  | string | 处理失败原因（成功时为空字符串）                                   |

### 5. 创建文章

```http
POST /api/v1/admin/knowledge-bases/:kb_id/articles
Authorization: Bearer <token>
```

**手动输入模式：**

```json
{
    "title": "VPN 连接超时怎么办？",
    "content": "1. 检查网络连接\n2. 尝试备用线路 vpn2.company.com\n3. 联系 IT 服务台（分机 8888）",
    "source_type": 1,
    "article_type": "procedure",
    "tags": ["VPN", "连接", "超时"]
}
```

| 字段          | 类型     | 必填 | 说明                                                  |
| ------------- | -------- | ---- | ----------------------------------------------------- |
| title         | string   | ✓    | 文章标题                                              |
| content       | string   | ✓    | 文章正文（Markdown 格式，不含 frontmatter）           |
| source_type   | int      |      | 1=手动输入（默认）, 2=文档上传, 3=深度研究             |
| article_type  | string   |      | 文章类型：guide/reference/procedure/analysis/note/faq/snippet；留空则发布时 LLM 补全 |
| tags          | string[] |      | 标签列表                                              |

> 创建后状态初始为「草稿(1)」。article_type 留空时，发布时由 LLM 推断补全并自动创建【元数据复核】工单。

**响应：**

```json
{ "code": 0, "message": "success", "data": { "id": 3 } }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                           |
| ------ | --------- | ------------------------------ |
| 10003  | 400       | 参数校验失败（标题/内容必填）   |
| 10004  | 404       | 知识库不存在                   |

### 6. 更新文章

```http
PUT /api/v1/admin/articles/:id
Authorization: Bearer <token>
```

**请求体：**

```json
{
    "title": "VPN 连接超时问题处理（修订版）",
    "content": "1. 检查本地网络连接\n2. 尝试备用线路...",
    "category": "网络与VPN",
    "tags": ["VPN", "超时"]
}
```

> 仅草稿(1)和驳回(5)状态可编辑。已发布文章修改后状态自动回退为草稿。

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10003  | 400       | 参数校验失败             |
| 10004  | 404       | 文章不存在               |

### 7. 文章详情

```http
GET /api/v1/admin/articles/:id
Authorization: Bearer <token>
```

**响应：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "id": 1,
        "kb_id": 1,
        "kb_name": "IT 知识 FAQ",
        "title": "VPN 连接超时怎么办？",
        "content": "1. 检查本地网络连接\n2. 尝试备用线路 vpn2.company.com\n3. 联系 IT 服务台（分机 8888）",
        "source_type": 1,
        "source_type_text": "手动输入",
        "category": "网络与VPN",
        "tags": ["VPN", "连接", "超时"],
        "status": 4,
        "status_text": "已发布",
        "word_count": 87,
        "chunk_count": 1,
        "file_type": "",
        "minio_path": "",
        "process_status": "completed",
        "process_error": "",
        "chunks": [
            {
                "id": 1,
                "kb_id": 1,
                "content": "VPN 连接超时怎么办？1. 检查本地网络连接...",
                "chunk_index": 0,
                "embedding_model": "Qwen3-Embedding-0.6B",
                "vector_dimension": 1536,
                "created_at": "2026-06-11T20:30:00Z"
            }
        ],
        "created_by": 1,
        "created_by_name": "admin",
        "reviewed_by": null,
        "published_by": 1,
        "published_by_name": "admin",
        "created_at": "2026-06-11T19:27:43Z",
        "updated_at": "2026-06-11T20:30:00Z"
    }
}
```

| 字段                      | 类型         | 说明                                |
| ------------------------- | ------------ | ----------------------------------- |
| chunks                    | array        | 文章的分块列表（含向量相关信息）    |
| chunks[].kb_id            | int64        | 知识库 ID（冗余字段，加速检索过滤） |
| chunks[].chunk_index      | int          | 分块序号（从 0 开始）               |
| chunks[].embedding_model  | string       | embedding 模型名称                  |
| chunks[].vector_dimension | int          | 向量维度                            |
| minio_path                | string | MinIO 对象路径（仅文档上传，无文档时为空字符串） |
| process_status            | string | 文档处理状态                                           |
| process_error             | string | 处理失败原因（成功时为空字符串）                       |

> chunks 含 `kb_id`/`chunk_index` 字段。embedding 向量不通过 JSON 返回（过大），仅在数据库中存储。

---

## 文档上传与处理

### 8. 上传文档

```http
POST /api/v1/admin/knowledge-bases/:kb_id/documents/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

**请求：** multipart/form-data，字段名 `files`，支持多文件同时上传。

| 约束           | 值                 |
| -------------- | ------------------ |
| 支持格式       | PDF、DOCX、MD、TXT |
| 单文件最大     | 50 MB              |
| 单次最多文件数 | 10 个              |

**响应 (200)：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "documents": [
            {
                "article_id": 101,
                "file_name": "账号管理FAQ.pdf",
                "file_size": 524288,
                "file_type": "pdf",
                "process_status": "pending"
            },
            {
                "article_id": 102,
                "file_name": "网络故障排查.docx",
                "file_size": 131072,
                "file_type": "docx",
                "process_status": "pending"
            }
        ]
    }
}
```

| 字段           | 类型   | 说明                               |
| -------------- | ------ | ---------------------------------- |
| article_id     | int64  | 自动创建的文章 ID                  |
| file_name      | string | 原始文件名                         |
| file_size      | int64  | 文件大小（字节）                   |
| file_type      | string | 文件类型（pdf/docx/md/txt）        |
| process_status | string | 初始状态为 `pending`，后台异步处理 |

**错误响应示例：**

```json
{ "code": 10003, "message": "不支持的文件格式: exe（仅支持 PDF、DOCX、MD、TXT）" }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                                           |
| ------ | --------- | ---------------------------------------------- |
| 10003  | 400       | 参数校验失败（文件格式不支持、文件过大超过 50MB） |
| 10004  | 404       | 知识库不存在                                   |
| 20003  | 503       | MinIO 存储服务不可用                           |

**处理流程：**

```
上传 → 存储至 MinIO → 创建 article 记录(source_type=upload)
  → 自动创建【文档复核】工单（source=3，关联 article_id + kb_id）
  → 发布时分块/embedding（上传时仅创建草稿，处理推迟到 Publish）
  失败 → process_status = "failed"，记录 process_error
```

> 上传后自动创建 `source=3` 知识库复核工单，标记需人工复核解析内容完整性。

### 9. 查询文档处理状态

```http
GET /api/v1/admin/knowledge-bases/:kb_id/documents/:id/status
Authorization: Bearer <token>
```

**响应：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "article_id": 101,
        "file_name": "账号管理FAQ.pdf",
        "process_status": "embedding",
        "process_error": ""
    }
}
```

| process_status | 说明                         |
| -------------- | ---------------------------- |
| `pending`      | 等待处理（在队列中）         |
| `parsing`      | 正在解析文档文本             |
| `chunking`     | 正在分块                     |
| `embedding`    | 正在生成向量                 |
| `indexing`     | 正在写入 pgvector            |
| `completed`    | 处理完成                     |
| `failed`       | 处理失败（见 process_error） |
| `disabled`     | 文档已停用                   |

### 10. 重试失败的文档

```http
POST /api/v1/admin/knowledge-bases/:kb_id/documents/:id/retry
Authorization: Bearer <token>
```

> 仅 `process_status=failed` 时可重试。重置状态为 `pending`，重新加入处理队列。

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10003  | 400       | 当前状态不为 `failed`    |
| 10004  | 404       | 文档不存在               |

---

## 审核流程

### 11. 提交审核

```http
POST /api/v1/admin/articles/:id/submit-review
Authorization: Bearer <token>
```

> 状态：草稿(1) → 待审核(2)

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10003  | 400       | 当前状态不为草稿(1)      |
| 10004  | 404       | 文章不存在               |

### 12. 审核操作

```http
POST /api/v1/admin/articles/:id/review
Authorization: Bearer <token>
```

**请求体：**

```json
{ "approved": true, "review_comment": "内容准确，通过审核" }
```

> `approved=true` → 审核通过(3)，否则 → 审核驳回(5)
>
> **业务规则：** 审核人不能是文章创建人。驳回时 `review_comment` 必填。

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                           |
| ------ | --------- | ------------------------------ |
| 10003  | 400       | 参数校验失败、状态不允许、驳回时 `review_comment` 必填 |
| 10004  | 404       | 文章不存在                     |

---

## 发布与停用

### 13. 发布

```http
POST /api/v1/admin/articles/:id/publish
Authorization: Bearer <token>
```

> 状态：审核通过(3) → 已发布(4)
>
> **内部逻辑：**
>
> 1. 解析 frontmatter → 若 `article_type` 缺失/非法，LLM 推断 type/tags（MetadataCompleter）
> 2. type 缺失触发补全时，自动创建【元数据复核】工单（source=3）
> 3. 生成含 frontmatter 的 .md 文件写入存储
> 4. 剥离 frontmatter → 对正文（# 标题 + body）分块（Chunker）
> 5. 批量调用 Embedding API 生成向量（Embedder，1536 维）
> 6. 将分块和向量写入 `knowledge_chunks` 表（VectorStore.BatchInsert）
> 7. 重建该知识库的 BM25 索引 + INDEX.md 页目录（onKBChanged 回调，per-kbID 锁）
> 8. 记录审计日志

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                           |
| ------ | --------- | ------------------------------ |
| 10003  | 400       | 仅审核通过的文章可发布           |
| 10004  | 404       | 文章不存在                     |
| 20001  | 503       | AI 服务不可用（Embedding 调用失败） |

**发布失败处理：**

如果 embedding 生成或向量写入失败，文章状态保持为「审核通过(3)」，`process_status` 设为 `failed`，`process_error` 记录失败原因。可通过「重试」按钮重新发布。

### 14. 停用

```http
POST /api/v1/admin/articles/:id/disable
Authorization: Bearer <token>
```

> 状态：已发布(4) → 已停用(0)
>
> **前置校验**：当前状态必须为 `Published(4)`，否则返回 `code=10003`。
>
> **内部逻辑：**
>
> 1. 从 `knowledge_chunks` 删除该文章的所有向量分块（VectorStore.DeleteByArticle）
> 2. 重建该知识库的 BM25 索引（onKBChanged 回调 → BuildIndex）
> 3. 记录审计日志
>
> 停用后文章不再参与 RAG 检索。

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10003  | 400       | 当前状态不为已发布(4)     |
| 10004  | 404       | 文章不存在               |

### 15. 启用文章

```http
POST /api/v1/admin/articles/:id/enable
Authorization: Bearer <token>
```

> 状态：已停用(0) → 已发布(4)
>
> **前置校验**：当前状态必须为 `Disabled(0)`，否则返回 `code=10003`。
>
> **内部逻辑**：复用发布管道（`Publish` 同款路径）：
>
> 1. Chunker.Split → 文本分块
> 2. Embedder.Embed → 生成向量
> 3. VectorStore.BatchInsert → 写入新向量（先写后删，失败时旧向量可恢复）
> 4. 更新状态为 `Published(4)` 并记录 `published_by`
>
> 启用是发布管道的完整重跑（不停用时向量已删除，必须重建才能再次被 RAG 检索到）。

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                           |
| ------ | --------- | ------------------------------ |
| 10003  | 400       | 当前状态不为已停用(0)           |
| 10004  | 404       | 文章不存在                     |
| 20001  | 503       | AI 服务不可用（Embedding 调用失败） |

## 知识库删除

### 16. 删除知识库

```http
DELETE /api/v1/admin/knowledge-bases/:id
Authorization: Bearer <token>
```

> 删除知识库及其下所有文章、分块向量、MinIO 文档文件。不可逆操作。
>
> **内部逻辑：**
>
> 1. 删除 `knowledge_chunks` 中该 kb 的所有向量（VectorStore.DeleteByKB）
> 2. 删除 MinIO 中 `kb_{id}/` 前缀的所有文档文件
> 3. 删除 `knowledge_articles` 记录
> 4. 删除 `knowledge_bases` 记录
> 5. 失效 BM25 缓存
> 6. 记录审计日志

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明                     |
| ------ | --------- | ------------------------ |
| 10004  | 404       | 知识库不存在             |

---

## 文章删除

### 17. 删除文章

```http
DELETE /api/v1/admin/articles/:id
Authorization: Bearer <token>
```

> 删除文章及其分块向量、存储文件。不可逆操作。
>
> **内部逻辑：**
>
> 1. 查询文章（不存在返回 `code=10004`）
> 2. 异步清理存储中的关联文件（`cleanupArticleFiles` goroutine）
> 3. 事务删除 `knowledge_chunks`（按 `article_id`）与 `knowledge_articles` 记录

**响应：**

```json
{ "code": 0, "message": "success", "data": null }
```

**错误码：**

| 错误码 | HTTP 状态 | 说明       |
| ------ | --------- | ---------- |
| 10004  | 404       | 文章不存在 |

---

## 通用文件服务

> 文章内嵌图片/附件的上传与访问，存储于 `article-assets` 目录，供 Markdown 编辑器内嵌引用；不绑定知识库、不触发解析流水线。

### 18. 上传文件

```http
POST /api/v1/admin/files/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

**请求：** multipart/form-data，字段名 `file`（单文件）。

| 约束       | 值                     |
| ---------- | ---------------------- |
| 字段名     | `file`（单文件）       |
| 单文件最大 | 50 MB                  |
| 文件类型   | 不限（保留原始扩展名） |

**响应 (200)：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "url": "/api/v1/admin/files/article-assets/550e8400-e29b-41d4-a716-446655440000.png",
        "filename": "550e8400-e29b-41d4-a716-446655440000.png"
    }
}
```

| 字段     | 类型   | 说明                                           |
| -------- | ------ | ---------------------------------------------- |
| url      | string | 访问 URL（`/api/v1/admin/files/article-assets/` + 存储文件名） |
| filename | string | 存储文件名（UUID + 原始扩展名）                |

> 文件名以 UUID 重命名存储，避免冲突与路径穿越；原始扩展名保留用于 `Content-Type` 判定。

**错误码：**

| 错误码 | HTTP 状态 | 说明                          |
| ------ | --------- | ----------------------------- |
| 10003  | 400       | 未选择文件 / 文件大小超过限制 |
| 99999  | 500       | 存储未初始化或文件上传失败    |

### 19. 获取文章资源文件

```http
GET /api/v1/admin/files/article-assets/:filename
Authorization: Bearer <token>
```

> `filename` 为上传时返回的 `filename`（UUID + 扩展名）。
>
> 返回文件二进制内容：Local 存储模式直接 `ServeFile`（`Content-Type` 由 `http.ServeFile` 自动判定）；MinIO 模式 `302` 重定向到预签名 URL。
>
> **安全校验：** 文件名不允许包含 `/` 或 `\`，否则返回 `code=10003`。

**响应：** 文件二进制流，或 `302` 重定向到 MinIO 预签名 URL。

**错误码：**

| 错误码 | HTTP 状态 | 说明                       |
| ------ | --------- | -------------------------- |
| 10003  | 400       | 文件名非法（含路径分隔符） |
| 99999  | 500       | 存储未初始化或获取文件失败 |

---

## 公共与配置接口

### 20. 上传配置

```http
GET /api/v1/config/upload
Authorization: Bearer <token>
```

> 供前端上传前校验文件类型与大小（对应 [上传文档](#8-上传文档)）。仅需 JWT 认证，无需 RBAC 权限码。

**响应：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "max_upload_size_kb": 51200,
        "max_upload_size": 52428800,
        "allowed_types": ["bmp", "docx", "gif", "jpg", "md", "pdf", "png", "pptx", "txt", "webp", "xlsx"],
        "max_files": 10
    }
}
```

| 字段               | 类型     | 说明                       |
| ------------------ | -------- | -------------------------- |
| max_upload_size_kb | int      | 上传大小上限（KB）         |
| max_upload_size    | int64    | 上传大小上限（字节）       |
| allowed_types      | string[] | 允许的文件类型列表         |
| max_files          | int      | 单次上传文件数上限         |

### 21. 公开图片访问

```http
GET /api/v1/public/images/:filename
```

> 公开端点（无需认证）。供 `<img>` 标签直接引用——img 标签无法携带 Authorization header。
>
> 服务解析器从文档提取的图片（全局 `image/` 目录，内容寻址命名，与文章解耦）。

**路径参数：**

| 参数     | 类型   | 说明                       |
| -------- | ------ | -------------------------- |
| filename | string | 图片文件名（不含路径分隔符） |

**响应：**

- Local 存储：直接返回图片文件，Content-Type 按扩展名推断。
- MinIO 存储：302 重定向到预签名 URL。

**错误码：**

| 错误码 | HTTP 状态 | 说明                           |
| ------ | --------- | ------------------------------ |
| 10003  | 400       | 无效的文件名（含路径分隔符）   |
| 20003  | 503       | 存储服务不可用（MinIO 操作失败） |
