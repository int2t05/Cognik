<p align="center">
  <img src="docs/assets/icon-dark.svg" width="88" height="88" alt="Cognik">
</p>

<h1 align="center">Cognik</h1>

<p align="center">
  自建 <strong>Agentic RAG</strong> 知识库——AI Agent 自动获取、检索、写回知识，无需手动上传文档。<br/>
  不用 LangChain，不连云，数据留在你的服务器。
</p>

<p align="center">
  <a href="https://github.com/int2t05/cognik/stargazers"><img alt="stars" src="https://img.shields.io/github/stars/int2t05/cognik?style=social"></a>
  <a href="https://github.com/int2t05/cognik/releases"><img alt="release" src="https://img.shields.io/github/v/release/int2t05/cognik?color=5b5bd6"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue"></a>
  <a href="https://github.com/int2t05/cognik/commits/main"><img alt="commits" src="https://img.shields.io/github/commit-activity/m/int2t05/cognik?color=5b5bd6"></a>
  <a href="README.md">English</a>
</p>

---

## 为什么选 Cognik

大多数 RAG 知识库是**只读的**——你上传文档，AI 回答。文档过期了，回答就错了。

Cognik 不一样：**Agent 会写回**。它搜索网络、抓取网页、提取知识，自动发布进 RAG 管线。下一轮对话即可检索到新知识。无需手动上传，没有过时文档。

**自迭代知识闭环：**

```
提问 → 检索（BM25 + pgvector + RRF + rerank + CRAG）
     → 不足时：web_search → 抓取 → 写回 → 自动发布进 RAG
     → 下次提问即可检索到新知识
```

全部跑在你的服务器上。PostgreSQL + pgvector + MinIO。除非你主动开启，不调用任何外部 API。

## 特性亮点

- 🔄 **Agent 自迭代闭环** — 搜索→抓取→写入→自动发布进 RAG；语义去重防止重复垃圾
- 🔍 **自建 RAG 引擎** — BM25 + pgvector 混合 → RRF 融合 → cross-encoder 重排 → CRAG 充分性评估（不用 LangChain）
- 🧠 **六级上下文压缩** — Tool Result Budget → Snip → Microcompact → HeadAndTail → 去重 → Autocompact（熔断器保护）
- 💾 **记忆系统** — 会话 + 全局记忆，每轮提取，跨会话 AutoDream 复盘
- 🎫 **工单闭环** — 上传/元数据缺失自动创建复核工单，人工兜底
- 🔐 **私有部署** — JWT + RBAC，Docker Compose，支持本地 llama.cpp 推理

## 快速开始

```bash
git clone https://github.com/int2t05/cognik.git
cd cognik

# 启动 PostgreSQL + pgvector
docker compose -f deploy/docker-compose.yml up -d postgres

# 启动后端 (:8080)
cd server && go run ./cmd/main.go

# 启动前端 (:3000)
cd web && npm install && npm run dev
```

打开 http://localhost:3000 — 默认账号：`admin` / `Admin@123`

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin + GORM |
| 数据库 | PostgreSQL + pgvector（halfvec + HNSW） |
| RAG | 自建 Go 引擎 — BM25（gse）/ 向量（pgvector）/ RRF / cross-encoder rerank / CRAG |
| LLM | 自建 `agent/llm.ChatModel`（net/http，兼容任意 OpenAI API） |
| 前端 | Next.js + React + TypeScript + shadcn/ui + Tailwind v4 |
| 部署 | Docker Compose / All-in-One 镜像 |

## 项目结构

```
server/
├── cmd/main.go              # 入口
├── internal/
│   ├── agent/              # ReAct 循环 + 工具 + 记忆 + 压缩
│   ├── domain/             # chat / knowledge / ticket / user / system
│   ├── rag/                # 自建 RAG 引擎（14 文件）
│   ├── parser/             # 文档解析（MinerU + 本地降级）
│   └── infra/              # adapter / config / database / storage / middleware
web/src/
├── app/                    # Next.js App Router
├── components/             # shadcn/ui
└── lib/api/                # API 客户端
docs/                       # PRD / TECH / ROADMAP / API / FLOW
deploy/                     # Docker Compose + All-in-One
```

## 文档

| 文档 | 用途 |
|------|------|
| [ROADMAP.md](docs/ROADMAP.md) | 产品技术路线图（V1.0 → V2.0） |
| [PRD.md](docs/PRD.md) | 产品需求 |
| [TECH.md](docs/TECH.md) | 技术架构 + ADR |
| [API/](docs/API/README.md) | API 契约（5 模块） |
| [FLOW/](docs/FLOW/README.md) | 业务流程图（mermaid） |

## License

[MIT](LICENSE)
