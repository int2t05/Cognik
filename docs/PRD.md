# OpsMind — 产品需求文档

关联文档：[TECH](TECH.md) · [API 文档](API/README.md) · [架构图](diagrams/)

## 1. 产品概述

OpsMind 是面向企业运维场景的 AI 数字员工系统，通过本地化大模型、私有知识库、运维申告门户和后台运营管理，辅助或替代人工完成常见咨询、自助处理、申告记录、人工流转和知识库更新。

核心能力：

- **RAG 增强智能问答** — 自建管道（查询改写→多路检索→BM25+向量混合→重排序→LLM生成），SSE 流式输出
- **多格式文档上传** — PDF/DOCX/MD/TXT 异步解析→分块→embedding→pgvector
- **运维申告全流程** — 状态机管理（待处理→处理中→需补充→已解决/已关闭）
- **统一知识文章模型** — 手动创建 + 文档上传，审核→发布→pgvector 向量写入
- **RBAC 权限管控** — 4 个预设角色，JWT 认证，菜单动态渲染
- **数据看板与审计** — 统计卡片 + 趋势图 + 操作日志

## 2. 架构定位

本地优先，兼顾云端。LLM 和 Embedding 支持本地 llama.cpp server 和远程 OpenAI-compatible API 两种模式，通过后台管理 UI 配置切换。向量存储统一使用 PostgreSQL + pgvector，与业务数据同库管理。

架构风格：单体分层架构（Modular Monolith）— Handler → Service → Repository 三层分离，RAG 引擎（`rag/` 包）为自包含领域模块。

## 3. 核心功能

### 3.1 智能问答

```
用户问题
  → 查询改写 (LLM 消除指代歧义)
  → 多路检索 (LLM 生成子查询)
  → 向量检索 (pgvector cosine 相似度)
  → BM25 检索 (Go 原生实现，中文分词)
  → RRF 融合 (Reciprocal Rank Fusion, k=60)
  → 重排序 (cross-encoder 重新评分)
  → LLM 生成答案 (SSE 流式)
```

- 每个步骤可独立开关（`rag_options`）
- 单步骤失败自动降级（除向量检索和 LLM 生成外）
- 置信度低于阈值时引导提交申告
- 支持用户反馈（已解决/未解决）

### 3.2 知识库管理

- **知识库 CRUD** — 创建/编辑/删除知识库，关联 LLM 配置
- **文章生命周期** — 草稿→提交审核→审核（不能是创建人）→发布→停用/恢复
- **发布时自动向量化** — 分块→embedding→pgvector 写入
- **停用时清理向量** — 从 pgvector 删除，不再参与检索
- **文档上传** — 支持 PDF/DOCX/MD/TXT，后台异步处理

### 3.3 申告管理

状态机：`待处理(1) → 处理中(2) → 已解决(4) / 需补充信息(3) → 已关闭(5)`

- 门户端提交申告（含紧急程度、影响范围、受影响系统）
- 后台处理（开始处理 / 索要补充 / 解决 / 关闭）
- 处理记录、站内消息通知、知识库候选生成
- 自动关闭：持续 7 天未操作的申告自动标记为已关闭

### 3.4 用户与权限

4 个预设角色：系统管理员 / 运维人员 / 知识库管理员 / 报障人

- JWT 认证（access_token + refresh_token）
- RBAC 权限中间件，菜单根据角色动态渲染
- 密码策略：8-32 位，含大小写字母和数字

### 3.5 数据看板

实时统计：今日申告 / 待处理 / 处理中 / 已解决 / 今日问答 / 平均置信度 / 知识条目数
趋势图：按日/周粒度展示申告和问答趋势

### 3.6 LLM 配置管理

- 支持两种方案：llama.cpp 本地部署和 OpenAI-compatible API
- LLM 和 Embedding 各自独立 Base URL，可指向不同服务
- 热替换：配置修改后通过 `atomic.Value` 即时生效，无需重启
- 测试连接：验证 Base URL 可达性和模型可用性

## 4. 部署架构

```
必须服务（4 个）:
  opsmind-server  — Go 后端 (Gin)
  opsmind-web     — Next.js 前端
  postgres        — PostgreSQL + pgvector（业务数据 + 向量存储）
  minio           — MinIO 对象存储（文档 + 附件）

可选服务:
  llama-cpp       — llama.cpp server（profile: ai-local）
```

`docker compose up -d --build` 一键启动。

## 5. 技术选型

| 层级 | 技术 |
|------|------|
| 后端语言 | Go |
| HTTP 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL + pgvector（HNSW 索引 + halfvec 半精度） |
| 中文分词 | gse（纯 Go，无 CGO） |
| 对象存储 | MinIO（S3-compatible） |
| 认证 | JWT + bcrypt |
| 前端框架 | Next.js / React / TypeScript |
| UI 组件 | Radix UI + 自定义 Apple Design System（浅色/暗色双主题） |
| 状态管理 | React Context + SWR |
| 部署 | Docker Compose |

## 6. API 设计

所有 API 采用统一响应格式 `{"code": 0, "message": "success", "data": {}}`，分页响应附加 `total/page/page_size`。

路由分为三组：
- `/api/v1/auth` — 公开（登录/刷新令牌）
- `/api/v1/portal` — 门户端（JWT），含智能问答 SSE 流式
- `/api/v1/admin` — 后台管理（JWT + RBAC）

详细 API 文档见 [docs/API/](API/README.md)。
