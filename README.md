<p align="center">
  <img src="docs/assets/icon-dark.svg" width="80" height="80" alt="Cognos">
</p>

<h1 align="center">Cognos</h1>

<p align="center">团队/个人知识库管理平台——AI 驱动的知识沉淀与检索</p>

<p align="center">
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue"></a>
  <a href="https://github.com/int2t05/Cognos"><img alt="platform" src="https://img.shields.io/badge/platform-Go%20%2B%20Next.js-5b5bd6"></a>
</p>

---

## 这是什么

Cognos 是一个私有部署的知识管理平台，帮助团队和个人沉淀知识、检索经验、解决问题。

核心是一个能自主调用工具、多步推理的 Agent——它决定何时检索知识库、何时搜索网络、何时写入新知识。写回即发布进 RAG，下一轮检索可召回，形成**自迭代知识闭环**。从 Agent 循环到检索管道全部自建，数据不出域。

## 特性亮点

- **Agent 自迭代闭环** — 搜索→抓取→写入→自动发布进 RAG，无需人工审核；语义去重防止重复写入
- **自建 RAG 引擎** — BM25 + pgvector 混合检索 + RRF 融合 + cross-encoder rerank + CRAG 充分性评估
- **六级上下文压缩** — Tool Result Budget → Snip → Microcompact → HeadAndTail → 去重 → Autocompact
- **记忆系统** — 会话记忆 + 全局记忆 + ExtractMemories 每轮提取 + AutoDream 跨会话复盘
- **工单闭环** — 上传/元数据补全自动创建复核工单，标记需人工介入
- **私有部署** — PostgreSQL + pgvector + MinIO，全数据自持，支持 llama.cpp 本地推理

## 核心能力

| 模块 | 能力 |
|------|------|
| **Agent 对话** | 自建 ReAct 循环，SubAgent 异步派发（research / coder / deep_research），reasoning + tool_call + tool_result 全事件流式渲染 |
| **知识库** | 文件即真相（Markdown + frontmatter schema），异步索引（chunk→embed→pgvector+BM25），Agent 自迭代闭环（写回即发布进 RAG），kb 工具 6 action |
| **记忆系统** | 会话记忆 + 全局记忆 + 后台复盘（ExtractMemories 每轮提取 + AutoDream 跨会话合并），参考 Claude Code 思想 |
| **上下文压缩** | 六级管线（Tool Result Budget → Snip → Microcompact → HeadAndTail → 去重 → Autocompact），熔断器保护 |
| **检索优化** | Sandwich Reorder + BM25 Enriched + RRF k=30 + Contextual Retrieval + Context Packing + Metadata 预过滤 |
| **深度搜索** | web_search（Exa→Tavily→DuckDuckGo）+ web_fetch（Firecrawl→本地）+ 搜索结果写入知识库闭环 |
| **工单管理** | 完整状态机（待处理→处理中→需补充→已解决/已关闭），7 天自动关闭，CAS 并发控制；上传/元数据补全自动创建复核工单 |
| **权限** | JWT 双令牌 + RBAC，4 个预设角色，菜单动态渲染 |

## 技术栈

- **后端**：Go + Gin + GORM
- **数据库**：PostgreSQL + pgvector（halfvec + HNSW，维度可配）
- **RAG**：自建 Go 引擎——BM25（gse 分词）/ 向量（pgvector）/ RRF 融合 / cross-encoder rerank
- **LLM**：自建 `agent/llm.ChatModel`（net/http 直连 OpenAI 兼容 API，工具描述透明传入）
- **Embedding**：DashScope text-embedding-v2 @ 1536 维（或任意 OpenAI 兼容 embedding 端点）
- **前端**：Next.js + React + TypeScript + shadcn/ui
- **部署**：Docker Compose + All-in-One 镜像

## 快速开始

```bash
# 启动数据库
docker compose -f deploy/docker-compose.yml up -d postgres

# 启动后端
cd server && go run ./cmd/main.go

# 启动前端
cd web && npm run dev
```

默认账号：`admin` / `Admin@123`

## 项目结构

```
server/
├── cmd/main.go              # 入口
├── internal/
│   ├── agent/              # ReAct 循环 + 工具 + 记忆 + 压缩
│   │   ├── llm/            # 自建 ChatModel（net/http 直连 OpenAI 兼容 API）
│   │   ├── tools/          # Agent 内置工具集（kb/memory/bash/grep/...）
│   │   └── store/          # SQLite 对话存储（与业务 PostgreSQL 隔离）
│   ├── domain/             # 业务领域（chat / knowledge / ticket / user / system）
│   ├── rag/                # 自建 RAG 引擎
│   ├── parser/             # 文档解析
│   └── infra/              # 基础设施（adapter / config / database / storage / middleware）
web/src/
├── app/                    # Next.js App Router
├── components/             # shadcn/ui 组件
└── lib/api/                # API 客户端
docs/                        # 正式文档
deploy/                      # Docker 部署
```

## 文档

| 文档 | 用途 |
|------|------|
| [DESIGN.md](docs/DESIGN.md) | Agentic RAG 架构设计 |
| [ROADMAP.md](docs/ROADMAP.md) | 产品技术路线图 |
| [PRD.md](docs/PRD.md) | 产品需求 |
| [TECH.md](docs/TECH.md) | 技术架构 |

## License

MIT
