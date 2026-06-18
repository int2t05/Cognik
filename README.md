# <img src="web/public/icon-32.png" width="28" height="28" alt="OpsMind" style="vertical-align: middle; margin-right: 6px;"> OpsMind — 运维数字员工系统

<p align="center">
  <img src="web/public/icon-96.png" width="96" height="96" alt="OpsMind">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Next.js-15-000000?logo=nextdotjs" alt="Next.js">
  <img src="https://img.shields.io/badge/PostgreSQL-18-4169E1?logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/pgvector-hnsw-336791" alt="pgvector">
  <img src="https://img.shields.io/badge/Docker-blue?logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/Design-Apple-0066cc?logo=apple" alt="Apple Design">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

**私有部署的 AI 运维助手** — 自建 Go RAG 引擎，BM25 + 向量混合检索，SSE 流式问答，申告全流程管理，一体 Docker Compose 部署。

## 为什么选择 OpsMind

大多数 AI 运维方案要么把数据交给云端，要么拼凑多个开源组件却无法协同。OpsMind 提供了一条不同的路径：

- **数据不出机房** — 本地 LLM + 本地 Embedding + 本地向量库，所有数据完全私有
- **自建 RAG，不依赖 LangChain** — 纯 Go 实现的完整检索增强管道，零外部 RAG 框架依赖
- **开箱即用** — `make up` 一条命令启动 4 个服务，5 分钟内开始使用
- **生产就绪** — 208 条 TODO 已在 [docs/TODO.md](docs/TODO.md) 中按优先级审计追踪

## 功能特性

| 模块 | 特性 |
|------|------|
| 🤖 **智能问答** | 自建 RAG 管道（查询改写→多路检索→BM25+向量混合→RRF融合→重排序→LLM生成），SSE 流式输出 |
| 📚 **知识库** | 统一文章模型 + 审核→发布工作流 + 自动向量写入 pgvector + PDF/DOCX/MD/TXT 异步处理 |
| 🎫 **申告管理** | 完整状态机（待处理→处理中→需补充→已解决/已关闭），站内消息通知，自动过期关闭 |
| 👥 **RBAC 权限** | JWT 双令牌认证 + 角色权限控制 + 菜单动态渲染 + bcrypt 密码策略 |
| ⚙️ **LLM 配置** | llama.cpp / OpenAI / DeepSeek 等多提供商支持，`atomic.Value` 热替换无需重启 |
| 📊 **数据看板** | 实时统计卡片 + 趋势图，问答量/置信度/申告量一目了然 |
| 📝 **审计日志** | 敏感操作全量记录，支持按操作类型和操作人筛选 |
| 🐳 **一键部署** | Docker Compose 编排（PostgreSQL+pgvector / MinIO / Server / Web + 可选 llama.cpp） |

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端框架 | Go 1.22+ / Gin 1.9+ | REST API + SSE 流式，分层架构 |
| ORM | GORM v1.25+ | PostgreSQL 数据访问 + AutoMigrate |
| 数据库 | PostgreSQL 18 + pgvector | 业务数据 + halfvec 向量 / HNSW 索引 |
| RAG 引擎 | 自建 Go（`rag/` 包） | BM25 + 向量混合检索 + RRF 融合 + 重排序 |
| 中文分词 | gse（纯 Go） | BM25 分词，无 CGO，跨平台 |
| LLM / Embedding | llama.cpp / OpenAI / DeepSeek | UI 配置，热切换，独立 URL 和 API Key |
| 对象存储 | MinIO | S3-compatible，文档上传 + 异步解析 |
| 认证 | JWT（golang-jwt v5）+ bcrypt | access_token 2h + refresh_token 7d |
| 前端 | Next.js 15 (React 19) / TypeScript | App Router + Radix UI + Apple Design 双主题 |
| 部署 | Docker Compose + Makefile | 4 必须服务 + 1 可选（llama.cpp），支持 profile |

## 快速启动

### 前置条件
- Docker Desktop 4.x+（含 Docker Compose v2）
- 磁盘 ≥ 10 GB，内存 ≥ 8 GB

### 一键部署

```bash
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind
cp .env.example .env
# 编辑 .env：至少配置 JWT_SECRET 和 LLM_BASE_URL
make up
```

等待约 1 分钟：
| 地址 | 说明 |
|------|------|
| http://localhost:3000 | 前端（Next.js） |
| http://localhost:8080 | 后端 API |
| http://localhost:9001 | MinIO 控制台 |

LLM/Embedding 服务是可选的——基础功能不依赖 AI 模型。如需本地 AI：

```bash
# 下载模型（首次约 3.2GB，国内走 ModelScope 阿里 CDN）
pip install modelscope
python -c "from modelscope import snapshot_download; snapshot_download('Qwen/Qwen3-4B-GGUF', allow_patterns='*Q4_K_M*', local_dir='./models')"
python -c "from modelscope import snapshot_download; snapshot_download('Qwen/Qwen3-Embedding-0.6B-GGUF', allow_patterns='*Q8_0*', local_dir='./models')"

# 启动含 AI 的完整环境
docker compose --profile ai-local up -d --build
```

> `docker compose --profile ai-local` 首次启动也会自动下载（默认走 ModelScope）。

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

### 重排序模型（可选，提升检索精度 ~5%）

Docker 镜像内置轻量 cross-encoder 模型用于 RAG 重排序。构建前需先下载模型到本地：

```bash
# 安装 huggingface_hub（仅需一次）
pip install huggingface_hub

# 从 hf-mirror.com 镜像下载（~50MB，国内速度稳定）
cd server
python models/rerank/download.py
```

模型文件会随 `docker compose build` 直接 COPY 进镜像，构建过程无需网络下载。

| 模型 | 大小 | 说明 |
|------|------|------|
| `MiniLM-L-2-v2` | ~17MB | 最小，2 层 transformer |
| `MiniLM-L-4-v2` | ~50MB | **默认**，4 层，体积与效果平衡 |
| `MiniLM-L-6-v2` | ~80MB | 6 层，效果更好 |
| `MiniLM-L-12-v2` | ~120MB | 12 层，效果最好 |

> 切换模型：设置环境变量 `RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-6-v2 python models/rerank/download.py`
>
> 不下载模型也可构建——RAG 管道会自动降级跳过重排序步骤。

## 本地开发

### 依赖服务（Docker）

```bash
make dev  # 启动 PostgreSQL + MinIO
```

### 后端

```bash
cd server
go mod tidy
go run ./cmd/main.go       # :8080，GORM AutoMigrate
make seed                   # 加载演示数据
```

### 前端

```bash
cd web
npm install
npm run dev                 # :3000，rewrite 代理 /api → :8080
```

### 运行测试

```bash
# Go 测试（12 包，需 PostgreSQL + pgvector）
cd server
go test ./tests/... -v -tags=integration -p 1

# API 集成测试（Playwright，129 个）
cd test
npm install && npm run test
```

## 项目结构

```
OpsMind/
├── server/                          # Go 后端（分层架构）
│   ├── cmd/main.go                  # 入口
│   ├── internal/
│   │   ├── handler/                 # HTTP Handler（10 模块）
│   │   ├── service/                 # 业务逻辑（12 服务）
│   │   ├── repository/              # 数据访问（9 Repo）
│   │   ├── model/                   # GORM 模型
│   │   ├── rag/                     # 自建 RAG 引擎（Pipeline/BM25/Hybrid/Processor）
│   │   ├── adapter/                 # LLM/Embedding/Vector/Storage 适配层
│   │   ├── middleware/              # JWT/RBAC/CORS/Logger
│   │   └── router/                  # Gin 路由注册
│   ├── pkg/                         # 公共工具（errcode/jwt/hash/response）
│   └── tests/                       # Go 集成测试（12 包）
│
├── web/                             # Next.js 前端
│   └── src/{app,components/ui,layout,shared,lib/api,hooks,styles}/
│
├── test/                            # Playwright API 测试（129 个）
├── docs/                            # 文档（PRD/TECH/API/TODO）
├── docker-compose.yml
├── Makefile
└── CLAUDE.md
```

## 改进路线

从 76 个源文件审计出 208 条改进项，详见 [docs/TODO.md](docs/TODO.md)。概要：

| 优先级 | 数量 | 聚焦领域 |
|--------|------|----------|
| 🔴 P0 | 58 | 并发安全、事务原子性、静默错误、配置安全 |
| 🟡 P1 | 107 | 架构对齐、context 传播、RAG 管道优化、API 规范 |
| 🟢 P2 | 43 | 代码风格、单元测试、工具函数提取 |

## 文档

| 文档 | 说明 |
|------|------|
| [PRD.md](docs/PRD.md) | 产品需求文档 |
| [TECH.md](docs/TECH.md) | 技术架构文档 |
| [API/](docs/API/README.md) | REST API 接口文档（9 组） |
| [TODO.md](docs/TODO.md) | 代码改进清单（208 条） |
| [CLAUDE.md](CLAUDE.md) | 项目上下文指令 |

## 许可证

MIT — 可自由使用、修改和分发。
