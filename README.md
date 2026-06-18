# <img src="web/public/icon-64.png" width="28" height="28" alt="OpsMind" style="vertical-align: middle; margin-right: 6px;"> OpsMind — 运维数字员工系统

<p align="center">
  <img src="web/public/icon-96.png" width="96" height="96" alt="OpsMind">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Next.js-000000?logo=nextdotjs" alt="Next.js">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/pgvector-hnsw-336791" alt="pgvector">
  <img src="https://img.shields.io/badge/Docker-blue?logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/Design-Apple-0066cc?logo=apple" alt="Apple Design">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

私有部署的 AI 运维助手 — 自建 Go RAG 引擎，BM25 + 向量混合检索，SSE 流式问答，申告全流程管理，Docker Compose 一键部署。

## 功能特性

| 模块 | 特性 |
|------|------|
| 智能问答 | 自建 RAG 管道（查询改写→多路检索→BM25+向量混合→RRF融合→重排序→LLM生成），SSE 流式输出 |
| 知识库 | 统一文章模型 + 审核→发布工作流 + 自动向量写入 pgvector + PDF/DOCX/MD/TXT 异步处理 |
| 申告管理 | 完整状态机（待处理→处理中→需补充→已解决/已关闭），站内消息通知，自动过期关闭 |
| RBAC 权限 | JWT 双令牌认证 + 角色权限控制 + 菜单动态渲染 + bcrypt 密码策略 |
| LLM 配置 | llama.cpp / OpenAI / DeepSeek 等多提供商支持，`atomic.Value` 热替换无需重启 |
| 数据看板 | 实时统计卡片 + 趋势图，问答量/置信度/申告量 |
| 审计日志 | 敏感操作全量记录，支持按操作类型和操作人筛选 |
| 一键部署 | Docker Compose 编排（PostgreSQL+pgvector / MinIO / Server / Web + 可选 llama.cpp） |

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端框架 | Go / Gin |
| ORM | GORM |
| 数据库 | PostgreSQL + pgvector（halfvec / HNSW 索引） |
| RAG 引擎 | 自建 Go（`rag/` 包）— BM25 + 向量混合检索 + RRF 融合 + 重排序 |
| 中文分词 | gse（纯 Go，无 CGO） |
| LLM / Embedding | llama.cpp / OpenAI / DeepSeek，UI 配置，热切换 |
| 对象存储 | MinIO（S3-compatible） |
| 认证 | JWT + bcrypt |
| 前端 | Next.js / React / TypeScript / Radix UI + Apple Design 双主题 |
| 部署 | Docker Compose + Makefile |

## 快速启动

### 前置条件
- Docker（含 Docker Compose v2）
- 磁盘 ≥ 10 GB，内存 ≥ 8 GB

### 一键部署

```bash
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind
cp .env.example .env
# 编辑 .env：至少配置 JWT_SECRET 和 LLM_BASE_URL
docker compose up -d --build
```

访问地址：

| 地址 | 说明 |
|------|------|
| http://localhost:3000 | 前端 |
| http://localhost:8080 | 后端 API |
| http://localhost:9001 | MinIO 控制台 |

如需本地 AI 模型：

```bash
# 启动含 llama.cpp 的完整环境
docker compose --profile ai-local up -d --build
```

```bash
# 加载演示数据（含预置账号）
docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/001_init.sql
```

预置账号：

| 账号 | 密码 | 角色 |
|------|------|------|
| `admin` | `Admin@123` | 系统管理员 |
| `operator1` | `Operator@123` | 运维人员 |
| `knowledge` | `Knowledge@123` | 知识库管理员 |
| `reporter1` | `Reporter@123` | 报障人 |

## 本地开发

### 依赖服务

```bash
make dev  # 启动 PostgreSQL + MinIO
```

### 后端

```bash
cd server
go mod tidy
go run ./cmd/main.go       # :8080，GORM AutoMigrate
```

### 前端

```bash
cd web
npm install
npm run dev                 # :3000，rewrite 代理 /api → :8080
```

### 运行测试

```bash
# Go 集成测试（需 PostgreSQL + pgvector）
cd server
go test ./tests/... -v -tags=integration -p 1

# API 集成测试（Playwright）
cd test
npm install && npm run test
```

## 项目结构

```
OpsMind/
├── server/                          # Go 后端（分层架构）
│   ├── cmd/main.go                  # 入口
│   ├── internal/
│   │   ├── handler/                 # HTTP Handler
│   │   ├── service/                 # 业务逻辑
│   │   ├── repository/              # 数据访问
│   │   ├── model/                   # GORM 模型
│   │   ├── rag/                     # 自建 RAG 引擎
│   │   ├── adapter/                 # LLM/Embedding/Vector/Storage 适配层
│   │   ├── middleware/              # JWT/RBAC/CORS/Logger
│   │   └── router/                  # Gin 路由注册
│   ├── pkg/                         # 公共工具
│   └── tests/                       # Go 集成测试
│
├── web/                             # Next.js 前端
│   └── src/{app,components/ui,layout,shared,lib/api,hooks,styles}/
│
├── test/                            # Playwright API 测试
├── docs/                            # 文档（PRD/TECH/API/图表）
├── docker-compose.yml
├── Makefile
└── CLAUDE.md
```

## 文档

| 文档 | 说明 |
|------|------|
| [PRD.md](docs/PRD.md) | 产品需求文档 |
| [TECH.md](docs/TECH.md) | 技术架构文档 |
| [API/](docs/API/README.md) | REST API 接口文档 |
| [CLAUDE.md](CLAUDE.md) | 项目上下文指令 |

## 许可证

MIT
