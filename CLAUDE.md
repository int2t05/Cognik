# CLAUDE.md — OpsMind 项目上下文指令

## 1. 角色声明

你是一名 **Go + Next.js 全栈开发者**，精通以下技术栈：

- **后端：** Go / Gin / GORM / PostgreSQL + pgvector
- **AI/RAG：** 自建 Go RAG 引擎（`server/internal/rag/`）— BM25/向量混合检索、查询改写、重排序 / pgvector 向量存储（HNSW 索引 + halfvec 半精度）
- **LLM / Embedding：** llama.cpp server（OpenAI-compatible）或任意 OpenAI-compatible API（OpenAI / DeepSeek / Moonshot 等）
- **中文分词：** gse（纯 Go，无 CGO），用于 BM25 检索
- **存储：** MinIO（S3-compatible 对象存储）
- **认证：** JWT / bcrypt / RBAC
- **前端：** Next.js / React / TypeScript / Radix UI / SWR / Apple Design System
- **部署：** Docker Compose / Makefile
- **设计系统：** Apple Design（浅色/暗色双主题 CSS 变量）

你在本项目中的职责是按照 `docs/PRD.md`、`docs/TECH.md` 和 `docs/API/` 接口文档，交付和迭代运维数字员工系统。

---

## 2. 关键前置操作

**在做任何涉及 AI/RAG 的开发之前，必须先阅读以下文档：**

- `docs/PRD.md` — 产品需求文档，定义了自建 RAG 引擎、文档上传、统一文章模型等功能需求
- `docs/TECH.md` — 技术架构文档，定义了分层架构、模块接口、数据库设计
- `docs/API/chat.md` — 智能问答 API（SSE 流式 + RAG 管道步骤）
- `docs/API/knowledge.md` — 知识库管理 API（含文档上传/处理状态）
- `docs/API/llm-config.md` — LLM 配置 API（llama.cpp / OpenAI-compatible）

**在修改任何接口或数据模型之前，必须先确认 TECH.md 中的定义是否需要同步更新。**

---

## 3. 项目概览

**OpsMind（运维数字员工系统）** 是面向企业运维场景的 AI 数字员工系统，通过本地化大模型、私有知识库、运维申告门户和后台运营管理能力，辅助或替代人工完成常见咨询、自助处理、申告记录、人工流转和知识库更新。

**核心目标：**
- 提供 RAG 增强的智能问答（自建管道：查询改写→多路检索→混合检索(BM25+向量+RRF)→重排序→LLM 生成）
- SSE 流式输出（真正的 token 级流式，含管道步骤进度）
- 多格式文档上传与异步处理（PDF / DOCX / MD / TXT → 分块 → embedding → pgvector）
- 运维申告全流程管理（状态机：待处理→处理中→需补充信息→已解决/已关闭）
- 知识库统一文章模型（CRUD + 审核发布 + pgvector embedding 写入）
- 用户角色权限管理（RBAC）
- 数据看板与审计日志

**架构风格：** 单体分层架构（Modular Monolith），Handler → Service → Repository 三层分离。RAG 模块（`rag/`）是自包含的领域引擎，不依赖 HTTP 层。

**向量存储：** 全部由 pgvector 承担——知识发布时由 OpsMind 自行完成分块、embedding 生成、向量写入（halfvec 类型，HNSW 索引）。PostgreSQL 统一管理业务数据和向量数据。

**LLM 调用路径：** OpsMind 后端 → `LLMClient` 接口 → llama.cpp server 或 OpenAI-compatible API。支持两种提供商：
- llama.cpp server（本地部署，Docker 可选 profile `ai-local`）
- OpenAI-compatible API（OpenAI / DeepSeek / Moonshot 等）

**Embedding 调用路径：** OpsMind 后端 → `EmbeddingClient` 接口 → 与 LLM 使用同一提供商的 `/v1/embeddings` 端点（模型名称可独立配置）。

---

## 4. 常用命令

### 后端（server/）

```bash
# 进入后端目录
cd server

# 安装依赖
go mod tidy

# 编译
go build ./cmd/...

# 运行（本地开发，依赖 Docker 中的 postgres(pgvector)/minio）
go run ./cmd/main.go

# 静态检查
go vet ./...

# 运行全部测试（不含集成测试）
go test ./tests/config/... -v

# 运行指定模块测试（需 PostgreSQL + pgvector）
go test ./tests/rag/... -v -tags=integration
go test ./tests/model/... -v -tags=integration
go test ./tests/database/... -v -tags=integration
go test ./tests/service/... -v -tags=integration
go test ./tests/adapter/... -v -tags=integration

# 运行全部集成测试（需 PostgreSQL + pgvector + MinIO；-p 1 避免跨包并行共享数据库冲突）
go test ./tests/... -v -tags=integration -p 1

# Lint（如安装了 golangci-lint）
golangci-lint run ./...
```

### 前端（web/）

```bash
# 进入前端目录
cd web

# 安装依赖
npm install

# 启动开发服务器（端口 3000，rewrite 代理到 localhost:8080）
npm run dev

# 构建生产版本
npm run build

# 类型检查 + Lint
npm run lint
```

### Docker Compose

```bash
# 在项目根目录执行

# 一键启动必须服务（opsmind-server, opsmind-web, postgres(pgvector), minio）
docker compose up -d --build

# 启动含 llama.cpp 的完整环境（需要本地模型文件）
docker compose --profile ai-local up -d --build

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f opsmind-server

# 停止全部服务
docker compose down

# 停止并清除数据卷
docker compose down -v
```

### 数据库初始化（手动执行）

```bash
# DDL 增强（HNSW 索引、列注释）
docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/init.sql

# 必要数据（角色 + 用户 + 菜单 + LLM 配置 + 系统配置）
docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_essential.sql
```

---

## 5. 项目结构

| 目录/文件 | 职责 |
| --- | --- |
| `docs/` | 项目文档（PRD、TECH、TODO、API、FLOW） |
| `docs/PRD.md` | 产品需求文档 — 自建 RAG 引擎、文档上传、统一文章模型 |
| `docs/TECH.md` | 技术架构文档 — 模块接口、ADR、数据库 DDL、部署配置 |
| `docs/API/` | API 文档 — 认证/问答/知识库/LLM配置/申告/用户/角色/看板/审计（9 份） |
| `docs/FLOW/` | 业务流程图（7 个模块 + README，含 Mermaid 与详细数据流） |
| `docs/TECH.md §7` | Apple 设计系统（色彩、字体、组件），原 `docs/prompts/ui.md` 已合并 |
| `server/cmd/main.go` | 后端入口，初始化配置、数据库、路由、RAG 模块、调度器 |
| `server/internal/config/` | Viper 配置管理（config.go + config.yaml） |
| `server/internal/middleware/` | Gin 中间件（JWT 认证、RBAC 权限、CORS、请求日志） |
| `server/internal/router/` | 路由注册（router.go + portal.go + admin.go） |
| `server/internal/handler/` | Handler 层 — auth/user/role/chat/ticket/knowledge/llm_config/dashboard/config/audit/message |
| `server/internal/service/` | Service 层 — auth/user/role/chat/ticket/knowledge/llm_config/dashboard/config/message/scheduler |
| `server/internal/repository/` | Repository 层 — user/role/config/ticket/knowledge/chat/audit/message/llm_config |
| `server/internal/model/` | GORM 数据模型 — user/role/ticket/knowledge/chat/audit/system/message/llm_config/enums/common |
| `server/internal/rag/` | RAG 引擎（pipeline / query_rewrite / multi_route / hybrid / bm25 / rerank / document_parser / chunker / embedder / processor） |
| `server/internal/adapter/` | 外部适配层 — LLMClient / EmbeddingClient / VectorStore(pgvector) / StorageClient(MinIO) |
| `server/internal/dto/` | 数据传输对象（request/ + response/） |
| `server/pkg/` | 公共工具包（response / errcode / jwt / hash） |
| `server/migrations/` | 数据库迁移和演示数据 |
| `server/tests/` | 全部测试代码（外部测试包：config/database/model/service/handler/middleware/adapter/rag） |
| `web/src/app/` | Next.js App Router 路由（auth/portal/admin 三组, layouts + pages） |
| `web/src/components/ui/` | Apple Design 原子组件（Button/Input/Dialog/Table 等 14 个） |
| `web/src/components/layout/` | 布局组件（AdminLayout / PortalLayout） |
| `web/src/components/shared/` | 复合组件（StatCard / StatusBadge / ConfirmDialog） |
| `web/src/lib/api/` | API 客户端封装（server-side + client-side, 12 个模块） |
| `web/src/hooks/` | React Hooks（useAuth / useTheme / useToast / useAIConfig / useLoading） |
| `web/src/styles/` | Apple Design Tokens（tokens.css 双主题 + global.css） |
| `docker-compose.yml` | Docker Compose 编排（4 必须服务: opsmind-server/web + postgres(pgvector:pg18) + minio + 1 可选: llama-cpp(profile: ai-local)） |
| `.env` | 环境变量（已 gitignore） |
| `.env.example` | 环境变量模板（提交到版本库） |
| `Makefile` | 构建和开发命令 |

---

## 6. 开发边界

### 始终要做 (Always do)

- **遵循三层架构：** Handler（参数校验、响应格式）→ Service（业务逻辑、事务）→ Repository（数据访问）。不允许跨层调用。RAG 模块（`rag/`）不依赖 Handler/Service/Repository 层。
- **写中文注释：** 每个文件需要文件头注释（说明模块存在的原因），每个关键函数需要函数注释（说明为什么这样实现）。见 §7 注释规范。
- **对齐文档：** 实现任何功能前先查看 `docs/PRD.md` 对应需求、`docs/TECH.md` 对应章节。实现完成后确认 TECH.md 和 API 文档不需要同步更新。
- **统一响应格式：** 所有 API 响应使用 `pkg/response` 封装，格式为 `{"code": 0, "message": "success", "data": {...}}`，错误码定义见 `pkg/errcode`。
- **密码策略校验：** 所有涉及密码创建/修改的场景，必须调用 `pkg/hash.ValidatePassword` 校验正则 `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`。
- **RBAC 权限校验：** 后台管理接口必须经过 JWT 中间件 + RBAC 中间件双重校验。
- **审计日志：** 用户管理、知识管理、申告管理的关键操作必须写入 audit_logs 表。
- **前端遵循 Apple Design：** 使用浅色/暗色双主题 CSS 变量，Action Blue (`#0066cc`) 为唯一品牌色，Inter Variable 字体，无边框扁平卡片，17px body，pill 圆角主按钮。
- **LLM/Embedding 通过适配层访问：** 后端只能通过 `LLMClient` / `EmbeddingClient` 接口调用 LLM 和 Embedding 服务，禁止直接 HTTP 调用。向量存储只能通过 `VectorStore` 接口访问 pgvector。
- **使用中文 git commit message：** 格式为 `类型: 简短描述`，如 `feat: 实现 BM25 混合检索`、`fix: 修复 pgvector 批量写入事务`。
- **测试写在 `tests/` 目录下，使用 `_test` 外部测试包：** 禁止在源码文件中写 `_test.go`，禁止 mock——测试代码要跟实际运行一样，依赖完整模块。
- **消费者接口模式：** Service 层定义它所需依赖的接口（而非引用具体 Repo 类型），遵循 Go "accept interfaces, return structs" 惯例。

### 绝不要做 (Never do)

- **绝不自动执行 `git push`，所有推送操作必须人工确认。**
- **绝不绕过适配层直接调用外部服务：** 禁止在 Service/Handler 中直接调用 LLM HTTP API（OpenAI-compatible）、MinIO API 或 pgvector 原始 SQL，必须通过 `LLMClient` / `EmbeddingClient` / `StorageClient` / `VectorStore` 接口。
- **绝不跳过 RAG 管道降级逻辑：** 管道单步骤失败不应阻塞后续步骤（向量检索和 LLM 生成除外——它们是核心路径，失败需返回错误）。降级矩阵见 `docs/API/chat.md`。
- **绝不在 Repository 层写业务逻辑：** Repository 只负责数据访问，业务规则（如审核人≠创建人、补充信息≤3 次、LLM 配置热替换）必须在 Service 层。RAG 算法逻辑（BM25、RRF、分块）在 `rag/` 包中，不在 Service 层。
- **绝不跳过状态机校验：** 申告状态转换必须在 `TicketService.UpdateStatus` 中严格校验前置状态，禁止直接 `UPDATE status`。知识库文章状态转换同理。
- **绝不硬编码配置值：** LLM Base URL、API Key、模型名称、向量维度、数据库连接、JWT 密钥等必须从配置文件或环境变量读取。LLM 配置修改通过 `atomic.Value` 热替换，无需重启。
- **绝不使用 localhost 访问 Docker 内部服务：** 容器内访问 llama.cpp 使用 `http://llama-cpp:8080/v1`，访问 PostgreSQL 使用 `postgres:5432`，访问 MinIO 使用 `minio:9000`。
- **绝不省略错误处理：** 外部服务调用（LLMClient、EmbeddingClient、VectorStore、StorageClient）必须处理超时、不可达、返回错误三种情况。
- **绝不跳过降级逻辑：** AI 服务不可用时必须返回明确提示（code=20001），RAG 向量检索失败时返回 code=20002，不能静默失败。
- **不要完全覆盖 .gitignore：** 如需添加规则，从文件末尾追加，不要替换已有内容。
- **禁止 mock 测试：** 测试代码要跟实际运行一样，依赖完整的模块。测试目录为 `server/tests/`，使用外部测试包 `_test`。

---

## 7. 注释规范

### 语言和风格

- **所有注释使用中文。**
- 注释必须解释 **为什么这样做**，而不是重复代码逻辑。
- 文件头注释说明模块存在的原因和解决的问题。
- 函数注释说明设计决策、为什么选择这种实现方案。

### 文件头注释示例

```go
// Package service 实现申告业务逻辑。
//
// 申告状态机是本模块的核心设计，采用显式状态转换表而非隐式条件判断，
// 原因是状态转换规则会随业务变化频繁调整，显式表更容易维护和审计。
package service
```

### 函数注释示例

```go
// UpdateStatus 执行申告状态转换。
//
// 为什么用 switch-case 而不是状态转换矩阵：
// MVP 阶段状态数量有限（5 个），switch-case 更直观且易于调试。
// 后续如状态超过 10 个，可重构为状态转换表。
//
// request_info 操作会同步触发站内消息通知（而非异步），
// 原因是消息写入是轻量操作，同步执行可保证事务一致性。
func (s *TicketService) UpdateStatus(id int64, operatorID int64, req *UpdateTicketStatusRequest) error {
    // ...
}
```

### 反面示例（不要这样写）

```go
// 错误：机械重复代码逻辑
// GetUserByID 根据 ID 获取用户
func (r *UserRepo) GetUserByID(id int64) (*User, error) {

// 错误：没有解释为什么
// 密码使用 bcrypt 哈希
func HashPassword(password string) (string, error) {

// 正确：解释为什么选择 bcrypt
// HashPassword 使用 bcrypt 对密码进行单向哈希。
// 选择 bcrypt 而非 argon2 的原因：Go 标准库直接支持，MVP 阶段无需额外依赖。
// cost=10 在 4C8GB 环境下单次哈希约 100ms，满足登录场景性能要求。
func HashPassword(password string) (string, error) {
```

---

## 8. 资源

### 项目文档

| 文档 | 用途 |
| --- | --- |
| `docs/PRD.md` | 产品需求文档 — 自建 RAG 引擎、文档上传解析、统一文章模型、SSE 流式输出 |
| `docs/TECH.md` | 技术架构文档 — 模块接口定义、数据库 DDL、API 设计、部署配置 |
| `docs/TECH.md` §7 | 设计系统附录 — 色彩令牌、字体层级、核心组件规范 |
| `docs/API/README.md` | API 文档索引 — 9 份接口文档，覆盖全部端点 |
| `docs/API/chat.md` | 智能问答 API — SSE 流式 + RAG 管道步骤事件 + 降级规则 |
| `docs/API/knowledge.md` | 知识库管理 API — KB/文章/审核/发布 + 文档上传/状态查询 + KB 删除 |
| `docs/API/llm-config.md` | LLM 配置 API — llama.cpp / OpenAI-compatible 双支持 + 热替换 |
| `docs/API/tickets.md` | 申告管理 API — 门户提交 + 后台处理 + 状态机 |
| `docs/API/users.md` | 用户管理 API — CRUD + 冻结/恢复 |
| `docs/API/roles.md` | 角色与菜单管理 API |
| `docs/API/dashboard.md` | 数据看板 API — 统计 + 趋势 |
| `docs/API/audit-log.md` | 审计日志 + 系统配置 + 站内消息 |
| `docs/FLOW/README.md` | 业务流程图索引 — 7 大模块端到端数据流 |

### 外部依赖文档

| 依赖 | 文档地址 |
| --- | --- |
| Gin | https://gin-gonic.com/docs/ |
| GORM | https://gorm.io/docs/ |
| golang-jwt | https://github.com/golang-jwt/jwt |
| MinIO Go SDK | https://min.io/docs/minio/linux/developers/go/API.html |
| pgvector | https://github.com/pgvector/pgvector |
| pgvector-go | https://github.com/pgvector/pgvector-go |
| gse (中文分词) | https://github.com/go-ego/gse |
| Next.js | https://nextjs.org/docs |
| React | https://react.dev/ |
| Radix UI | https://www.radix-ui.com/ |
| SWR | https://swr.vercel.app/ |
| Inter (字体) | https://fonts.google.com/specimen/Inter |

### 环境变量模板

完整 `.env` 配置见 `docs/TECH.md`，核心变量：

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | PostgreSQL 密码 | opsmind_dev |
| `MINIO_ROOT_USER` | MinIO 管理员用户名 | minioadmin |
| `MINIO_ROOT_PASSWORD` | MinIO 管理员密码 | minioadmin |
| `JWT_SECRET` | JWT 签名密钥 | 需手动设置 |
| `LLM_BASE_URL` | LLM API 地址 | http://llama-cpp:8080/v1 |
| `LLM_API_KEY` | API 密钥（OpenAI 需要；llama.cpp 不需要） | — |
| `LLM_MODEL` | LLM 模型名称 | qwen3-4b |
| `LLM_MAX_TOKENS` | 最大生成 Token 数 | 8192 |
| `EMBEDDING_BASE_URL` | Embedding API 地址（空则回退到 LLM_BASE_URL） | — |
| `EMBEDDING_MODEL` | Embedding 模型名称 | bge-m3 |
| `EMBEDDING_DIMENSION` | 向量维度 | 1024 |
| `AI_CONFIDENCE_THRESHOLD` | 置信度阈值 | 0.6 |
| `AI_DEFAULT_TOP_K` | 默认检索 Top K | 5 |
| `LLAMA_MODELS_DIR` | llama.cpp 模型文件目录 | ./models |

### 预设角色

| 角色 | 说明 | 典型权限 |
| --- | --- | --- |
| 系统管理员 | 系统全局管理 | 全部后台权限、角色权限管理、LLM 配置、系统配置 |
| 运维人员 | 处理申告和回访 | 申告查看/处理、回访记录、知识候选提交 |
| 知识库管理员 | 维护和审核知识 | 知识 CRUD、知识审核、文档上传、知识库配置 |
| 报障人 | 门户端用户 | 智能问答、申告提交、进度查询（仅门户端，无后台权限） |

### 错误码速查

| 错误码 | HTTP 状态 | 说明 |
| --- | --- | --- |
| 0 | 200 | 成功 |
| 10001 | 401 | 未登录或令牌过期 |
| 10002 | 403 | 无权限 |
| 10003 | 400 | 参数校验失败 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（如账号名重复） |
| 10006 | 400 | 用户已被冻结 (ErrAlreadyFrozen) |
| 10007 | 400 | 用户已处于正常状态 (ErrAlreadyActive) |
| 20001 | 503 | AI 服务不可用（LLM/Embedding 调用失败） |
| 20002 | 503 | RAG 服务不可用（pgvector 检索失败） |
| 20003 | 503 | 存储服务不可用（MinIO 操作失败） |
| 99999 | 500 | 未知错误 |
