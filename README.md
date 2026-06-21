<p align="center">
  <img src="web/public/icon-96.png" width="96" height="96" alt="OpsMind">
</p>

<h1 align="center">OpsMind</h1>

<p align="center"><strong>私有部署的 AI 运维数字员工</strong> — 让每家企业拥有自己的智能运维助手</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Next.js-16-000000?logo=nextdotjs" alt="Next.js">
  <img src="https://img.shields.io/badge/PostgreSQL-pgvector-4169E1?logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/Docker-blue?logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

---

## 为什么是 OpsMind？

企业运维团队每天面对大量重复性咨询：密码重置、权限申请、系统报障、操作指引。这些工作消耗运维人员 40% 以上的时间，却无法沉淀为可复用的知识。

OpsMind 是一个**完全私有化部署**的 AI 运维助手，核心思路不是"接一个大模型 API"，而是：

1. **自建 RAG 引擎** — BM25 + 向量混合检索 + RRF 融合 + 重排序，全过程可控、可审计
2. **知识资产化** — 每次问答、每条申告处理记录都可转化为知识库文章，经审核后发布
3. **私有数据不出域** — 全部数据存储在自有的 PostgreSQL + pgvector 中，支持本地 llama.cpp

不是另一个 ChatGPT 套壳。是一个从检索管道到业务流程都自建的运维数字员工系统。

## 核心能力

<table>
<tr>
<td width="50%">

### 智能问答（RAG）
自建 7 步检索管道，SSE 流式输出：
```
查询改写 → 多路检索 → 向量检索
→ BM25 检索 → RRF 融合
→ 重排序 → LLM 生成
```
- 中文分词（gse，纯 Go）
- 单步可独立开关，失败自动降级
- 置信度不足时引导提交申告

</td>
<td width="50%">

### 知识库管理
统一文章模型 + 审核发布工作流：
- 手动创建 / 文档上传两种录入方式
- 草稿 → 审核 → 发布 → 停用
- 发布时自动分块→embedding→pgvector
- 停用时自动清理向量，不再参与检索
- 支持 PDF / DOCX / MD / TXT

</td>
</tr>
<tr>
<td width="50%">

### 申告全流程管理
完整状态机驱动的工单流转：
```
待处理 → 处理中 → 已解决
              ↘ 需补充信息 → 已关闭
```
- 站内消息实时通知
- 处理记录全量留存
- 7 天无操作自动关闭
- 支持从申告生成知识候选

</td>
<td width="50%">

### 权限与数据看板
- JWT 双令牌 + bcrypt 密码策略
- RBAC 角色权限（4 个预设角色）
- 菜单根据权限动态渲染
- 实时统计卡片 + 30 日趋势图
- 敏感操作全量审计日志

</td>
</tr>
</table>

## 技术架构

```
客户端 (Next.js App Router)
  │
  ├─ HTTP/REST + SSE ──▶ Go 后端 (Gin, :8080)
  │                        │
  │   ┌────────────────────┼────────────────────┐
  │   │                    │                    │
  │   ▼                    ▼                    ▼
  │  Handler              Service           Repository
  │  (参数校验)          (业务逻辑)         (GORM 数据访问)
  │   │                    │                    │
  │   └────────────────────┼────────────────────┘
  │                        │
  │           ┌────────────┼────────────┐
  │           ▼            ▼            ▼
  │       RAG 引擎     Adapter       Middleware
  │     (检索管道)   (LLM/Embed/   (JWT/RBAC)
  │                   pgvector/MinIO)
  │
  ▼
PostgreSQL+pgvector    MinIO        llama.cpp (可选)
 (业务+向量数据)     (对象存储)    (本地 AI 模型)
```

**RAG 完全自建** — 不依赖 LangChain 或 LlamaIndex。BM25 算法用纯 Go 实现，向量检索走 pgvector 的 HNSW 索引 + halfvec 半精度，重排序用本地 cross-encoder 模型。

## 快速启动

### 前置条件

- Docker（含 Docker Compose v2）
- Python 3.10+（仅本地开发需要，用于 cross-encoder 重排序子进程）
- 8 GB 内存，10 GB 磁盘

### 一键部署

```bash
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind
cp .env.example .env
# 编辑 .env：至少设置 JWT_SECRET 和 LLM_BASE_URL
docker compose up -d --build
```

启动后访问：

| 服务 | 地址 |
|------|------|
| 前端 | http://localhost:3000 |
| 后端 API | http://localhost:8080 |
| MinIO 控制台 | http://localhost:9001 |

### 使用本地 AI 模型

```bash
# 启动含 llama.cpp 的完整环境（需提前放置 GGUF 模型文件）
docker compose --profile ai-local up -d --build
```

### 加载演示数据

```bash
docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/001_init.sql
```

预置账号：

| 账号 | 密码 | 角色 |
|------|------|------|
| `admin` | `Admin@123` | 系统管理员 |
| `operator1` | `Operator@123` | 运维人员 |
| `knowledge` | `Knowledge@123` | 知识库管理员 |
| `reporter1` | `Reporter@123` | 报障人 |

## LLM 配置

OpsMind 不绑定任何特定 AI 服务。在后台「LLM 配置」页面可自由切换：

| 提供商 | 说明 |
|--------|------|
| llama.cpp | 本地部署，数据不出域 |
| OpenAI | 兼容 `/v1/chat/completions` 和 `/v1/embeddings` |
| DeepSeek | 同上，OpenAI-compatible API |
| 其他兼容服务 | 任何实现 OpenAI API 格式的服务均可 |

LLM 和 Embedding 的 Base URL 可独立配置（如 LLM 用 DeepSeek，Embedding 用本地 bge-m3），配置修改后热替换生效，无需重启。

## 项目结构

```
OpsMind/
├── server/                       # Go 后端
│   ├── cmd/main.go               # 入口
│   ├── internal/
│   │   ├── handler/              # HTTP Handler（11 个 API 域）
│   │   ├── service/              # 业务逻辑 + 状态机
│   │   ├── repository/           # GORM 数据访问
│   │   ├── model/                # 数据模型 + 枚举
│   │   ├── rag/                  # 自建 RAG 引擎（12 个模块）
│   │   ├── adapter/              # LLM / Embedding / pgvector / MinIO
│   │   ├── middleware/           # JWT / RBAC / CORS / Logger
│   │   └── router/               # Gin 路由
│   ├── pkg/                      # 公共工具（response / errcode / jwt / hash）
│   ├── migrations/               # 数据库迁移 + 种子数据
│   └── tests/                    # Go 集成测试
│
├── web/                          # Next.js 前端
│   ├── src/app/                  # App Router（portal / admin / login）
│   ├── src/components/ui/        # Apple Design 原子组件
│   ├── src/components/layout/    # PortalLayout / AdminLayout
│   ├── src/components/chat/      # 问答组件（ChatInput / Message / Pipeline）
│   ├── src/lib/api/              # API 客户端（12 个模块）
│   └── src/hooks/                # React Hooks
│
├── docs/                         # 项目文档
│   ├── PRD.md                    # 产品需求文档
│   ├── TECH.md                   # 技术架构文档
│   ├── API/                      # REST API 接口文档（9 份）
│   └── diagrams/                 # Mermaid 架构与业务流程图
│
├── docker-compose.yml            # Docker Compose 编排
├── Makefile                      # 构建与开发命令
└── CLAUDE.md                     # 项目上下文指令
```

## 本地开发

### 启动依赖服务

```bash
make dev   # 启动 PostgreSQL + MinIO
```

### 后端

```bash
cd server

# 安装 rerank 依赖（cross-encoder 重排序子进程，BAAI/bge-reranker-base）
pip install torch sentence-transformers

go mod tidy
go run ./cmd/main.go     # :8080，GORM AutoMigrate
```

### 前端

```bash
cd web
npm install
npm run dev               # :3000，代理 /api → :8080
```

### 运行测试

```bash
# Go 集成测试（需 PostgreSQL + pgvector）
cd server
go test ./tests/... -v -tags=integration -p 1

# 前端 E2E 测试（Playwright）
cd web
npx playwright test
```

## 文档

| 文档 | 说明 |
|------|------|
| [PRD](docs/PRD.md) | 产品需求 — 功能定义、业务规则 |
| [TECH](docs/TECH.md) | 技术架构 — 分层设计、数据库 DDL、ADR |
| [API](docs/API/README.md) | REST API — 9 份接口文档，覆盖全部端点 |
| [Diagrams](docs/diagrams/README.md) | 架构与业务流程图（Mermaid） |

## 贡献

欢迎提交 Issue 和 Pull Request。在提交 PR 之前：

1. 确保通过现有测试（`go test ./tests/... -v -tags=integration` 和 `npx playwright test`）
2. 遵循项目现有的代码风格和注释规范
3. 涉及 API 变更时同步更新 `docs/API/` 文档

## 许可证

[MIT](LICENSE)
