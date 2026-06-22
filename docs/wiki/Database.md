# 🗄️ 数据库设计

OpsMind 使用 **PostgreSQL + pgvector** 统一管理业务数据和向量数据，无需额外的向量数据库。

## 设计原则

- **一库两用** — PostgreSQL 同时存储关系数据和向量数据（通过 pgvector 扩展）
- **halfvec 半精度** — 向量使用 `halfvec` 类型（IEEE 754 float16），节省 50% 存储空间
- **HNSW 索引** — 使用 `halfvec_cosine_ops` 算子，10K 分块下查询 < 50ms
- **GORM AutoMigrate** — 表结构通过 GORM 模型自动创建和迁移

## 核心表一览

| 表名 | 说明 | 关键字段 |
|------|------|----------|
| `users` | 用户 | bcrypt 密码哈希、状态（正常/冻结）、角色关联 |
| `roles` | 角色 | 名称、描述、内置标记 |
| `user_roles` | 用户-角色关联 | user_id + role_id 联合唯一 |
| `role_menus` | 角色-菜单关联 | 角色可访问的菜单项和权限码 |
| `knowledge_bases` | 知识库 | 名称、描述、关联 llm_config |
| `knowledge_articles` | 知识文章 | 标题、内容、status（审核状态机）、source_type、process_status |
| `knowledge_chunks` | 文章分块 | chunk 索引、文本内容、halfvec embedding |
| `chat_sessions` | 问答会话 | user_id、kb_id、标题 |
| `chat_messages` | 问答消息 | session_id、角色（user/assistant）、内容、置信度、来源引用 |
| `tickets` | 申告 | 标题、描述、状态、紧急程度、影响范围、处理人 |
| `ticket_records` | 申告操作记录 | ticket_id、操作类型、操作人、备注 |
| `llm_configs` | LLM 配置 | Base URL、API Key、模型名、是否默认 |
| `audit_logs` | 审计日志 | 操作人、操作类型、目标资源、变更前后数据 |
| `system_configs` | 系统配置 | 键值对，用于存储系统级参数 |
| `messages` | 站内消息 | 接收人、标题、内容、已读状态 |

## 关键模型定义

### 用户与角色

```
users ──N:M── user_roles ──N:M── roles
                                    │
                              role_menus (权限码列表)
```

- 一个用户可以有多个角色，权限为所有角色权限的并集
- `roles.builtin = true` 的角色不可删除（预设角色）
- 密码使用 bcrypt 存储，cost = 10

### 知识库与文章

```
knowledge_bases ──1:N── knowledge_articles ──1:N── knowledge_chunks
      │                                                │
      └── llm_config_id (关联 LLM 配置)                 └── embedding (halfvec)
```

- 文章有两个独立状态机：
  - `status`（审核状态）：`draft → reviewing → approved → published → disabled`
  - `process_status`（处理状态）：`pending → parsing → chunking → embedding → indexing → completed`
- `source_type` 区分手动创建（`manual`）和文档上传（`upload`）

### 申告状态机

```
tickets.status:
  1 (pending)   — 待处理
  2 (processing) — 处理中
  3 (need_info)  — 需补充信息
  4 (resolved)   — 已解决
  5 (closed)     — 已关闭
```

- 状态转换通过 CAS（Compare-And-Swap）更新：`WHERE id=? AND status=?`
- 每次状态变更自动创建 `ticket_records` 记录

### 问答会话

```
chat_sessions ──1:N── chat_messages
```

- 消息角色：`user` / `assistant`
- 助理消息含 `confidence`（置信度）、`sources`（引用来源 JSON）
- 会话关联知识库，检索时限定在该 KB 范围内

## pgvector 配置

### 向量列定义

```sql
-- halfvec 类型，维度由 EMBEDDING_DIMENSION 配置决定（默认 1024）
embedding halfvec(1024)
```

### HNSW 索引

```sql
CREATE INDEX idx_chunks_embedding ON knowledge_chunks
  USING hnsw (embedding halfvec_cosine_ops)
  WITH (m = 16, ef_construction = 200);
```

### 核心查询

```sql
-- 余弦相似度搜索
SELECT c.id, c.content, c.embedding <=> $1::halfvec AS distance
FROM knowledge_chunks c
WHERE c.kb_id = $2
ORDER BY c.embedding <=> $1::halfvec
LIMIT $3;
```

`<=>` 运算符计算 cosine distance（距离越小越相似）。

### 分块参数

- `chunk_size = 1000` 字符
- `chunk_overlap = 200` 字符
- 使用 RecursiveCharacterTextSplitter 算法

## 迁移

数据库表结构通过 GORM AutoMigrate 自动创建：

```go
// server/internal/database/database.go
db.AutoMigrate(
    &model.User{},
    &model.Role{},
    &model.UserRole{},
    &model.RoleMenu{},
    &model.KnowledgeBase{},
    &model.KnowledgeArticle{},
    &model.KnowledgeChunk{},
    // ...
)
```

pgvector 扩展需手动启用：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

Docker 镜像 `pgvector/pgvector:pg18` 已预装 pgvector 扩展，无需额外配置。

## 种子数据

后端启动时自动执行种子数据填充（通过 `server/migrations/` 中的脚本），包括：

- 4 个预设角色及其权限码
- 默认管理员账号（`admin` / `Admin@123456`）
- 默认 LLM 配置（读取 `.env` 中的值）
- 示例知识库

种子数据仅在对应表为空时插入，不会覆盖已有数据。
