<p align="center">
  <img src="docs/assets/icon-dark.svg" width="80" height="80" alt="Cognos">
</p>

<h1 align="center">Cognos</h1>

<p align="center">A private-deploy knowledge management platform — AI-driven knowledge capture and retrieval for teams and individuals</p>

<p align="center">
  <a href="README.zh-CN.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue"></a>
  <a href="https://github.com/int2t05/Cognos"><img alt="platform" src="https://img.shields.io/badge/platform-Go%20%2B%20Next.js-5b5bd6"></a>
</p>

---

## What is this

Cognos is a privately-deployed knowledge management platform that helps teams and individuals capture knowledge, retrieve experience, and solve problems.

At its core is an Agent that autonomously calls tools and reasons in multiple steps — it decides when to retrieve the knowledge base, when to search the web, and when to write new knowledge. Writes are published straight into the RAG pipeline and become retrievable in the next round, forming a **self-iterating knowledge loop**. The entire chain, from the Agent loop to the retrieval pipeline, is built in-house; data never leaves your domain.

## Highlights

- **Self-iterating Agent loop** — search → fetch → write → auto-publish into RAG, no manual review; semantic de-duplication prevents duplicate writes
- **Self-built RAG engine** — BM25 + pgvector hybrid retrieval + RRF fusion + cross-encoder rerank + CRAG sufficiency assessment
- **Six-stage context compression** — Tool Result Budget → Snip → Microcompact → HeadAndTail → De-dup → Autocompact
- **Memory system** — session memory + global memory + per-turn ExtractMemories + AutoDream cross-session review
- **Ticket workflow** — uploads / metadata completion auto-create review tickets, flagging cases that need human attention
- **Private deployment** — PostgreSQL + pgvector + MinIO, all data self-held, supports local llama.cpp inference

## Core capabilities

| Module | Capabilities |
|------|------|
| **Agent chat** | Self-built ReAct loop, async SubAgent dispatch (research / coder / deep_research), full streaming of reasoning + tool_call + tool_result events |
| **Knowledge base** | File as source of truth (Markdown + frontmatter schema), async indexing (chunk → embed → pgvector + BM25), self-iterating loop (write-back publishes into RAG), 6 kb tool actions |
| **Memory system** | Session memory + global memory + background review (ExtractMemories per-turn + AutoDream cross-session merge), inspired by Claude Code |
| **Context compression** | Six-stage pipeline (Tool Result Budget → Snip → Microcompact → HeadAndTail → De-dup → Autocompact) with circuit-breaker protection |
| **Retrieval optimization** | Sandwich Reorder + BM25 Enriched + RRF k=30 + Contextual Retrieval + Context Packing + Metadata pre-filtering |
| **Deep search** | web_search (Exa → Tavily → DuckDuckGo) + web_fetch (Firecrawl → local) + search-result-to-knowledge-base loop |
| **Ticket management** | Full state machine (Pending → In Progress → Needs Info → Resolved / Closed), 7-day auto-close, CAS concurrency control; uploads / metadata completion auto-create review tickets |
| **Access control** | JWT dual-token + RBAC, 4 preset roles, dynamic menu rendering |

## Tech stack

- **Backend**: Go + Gin + GORM
- **Database**: PostgreSQL + pgvector (halfvec + HNSW, configurable dimension)
- **RAG**: self-built Go engine — BM25 (gse tokenizer) / vector (pgvector) / RRF fusion / cross-encoder rerank
- **LLM**: self-built `agent/llm.ChatModel` (net/http direct to OpenAI-compatible API, tool descriptions passed through transparently)
- **Embedding**: Qwen3-Embedding-0.6B @ 1024 dims or DashScope text-embedding-v2 @ 1536 dims (configurable; any OpenAI-compatible endpoint)
- **Frontend**: Next.js + React + TypeScript + shadcn/ui
- **Deployment**: Docker Compose + All-in-One image

## Quick start

```bash
# Start the database
docker compose -f deploy/docker-compose.yml up -d postgres

# Start the backend
cd server && go run ./cmd/main.go

# Start the frontend
cd web && npm run dev
```

Default account: `admin` / `Admin@123`

## Project structure

```
server/
├── cmd/main.go              # entry point
├── internal/
│   ├── agent/              # ReAct loop + tools + memory + compression
│   │   ├── llm/            # self-built ChatModel (net/http to OpenAI-compatible API)
│   │   ├── tools/          # built-in agent toolset (kb/memory/bash/grep/...)
│   │   └── store/          # SQLite conversation store (isolated from business PostgreSQL)
│   ├── domain/             # business domains (chat / knowledge / ticket / user / system)
│   ├── rag/                # self-built RAG engine
│   ├── parser/             # document parsing
│   └── infra/              # infrastructure (adapter / config / database / storage / middleware)
web/src/
├── app/                    # Next.js App Router
├── components/             # shadcn/ui components
└── lib/api/                # API clients
docs/                       # formal docs
deploy/                     # Docker deployment
```

## Documentation

| Doc | Purpose |
|------|------|
| [ROADMAP.md](docs/ROADMAP.md) | Product & tech roadmap |
| [PRD.md](docs/PRD.md) | Product requirements |
| [TECH.md](docs/TECH.md) | Technical architecture |

## License

MIT
