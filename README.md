<p align="center">
  <img src="docs/assets/icon-dark.svg" width="88" height="88" alt="Cognik">
</p>

<h1 align="center">Cognik</h1>

<p align="center">
  Self-hosted <strong>Agentic RAG</strong> knowledge base — an AI agent that captures, retrieves, and writes back knowledge automatically.<br/>
  No LangChain. No cloud. Your data stays on your server.
</p>

<p align="center">
  <a href="https://github.com/int2t05/cognik/stargazers"><img alt="stars" src="https://img.shields.io/github/stars/int2t05/cognik?style=social"></a>
  <a href="https://github.com/int2t05/cognik/releases"><img alt="release" src="https://img.shields.io/github/v/release/int2t05/cognik?color=5b5bd6"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue"></a>
  <a href="https://github.com/int2t05/cognik/commits/main"><img alt="commits" src="https://img.shields.io/github/commit-activity/m/int2t05/cognik?color=5b5bd6"></a>
  <a href="README.zh-CN.md">简体中文</a>
</p>

---

## Why Cognik

Most RAG knowledge bases are **read-only** — you upload docs, the AI answers. When the docs are outdated, answers are wrong.

Cognik is different: **the agent writes back**. It searches the web, fetches pages, extracts knowledge, and publishes it straight into the RAG pipeline — automatically. Next round, that knowledge is retrievable. No manual uploads, no stale docs.

**Self-iterating knowledge loop:**

```
ask → retrieve (BM25 + pgvector + RRF + rerank + CRAG)
    → if gap: web_search → fetch → write back → auto-publish into RAG
    → next question retrieves the new knowledge
```

Everything runs on your server. PostgreSQL + pgvector + MinIO. No external API calls unless you enable them.

![cognik.png](https://s3.bmp.ovh/2026/09/06/039tVckl.png)

## Highlights

- 🔄 **Self-iterating Agent loop** — search → fetch → write → auto-publish into RAG; semantic de-duplication prevents garbage
- 🔍 **From-scratch RAG engine** — BM25 + pgvector hybrid → RRF fusion → cross-encoder rerank → CRAG sufficiency assessment (no LangChain)
- 🧠 **Six-stage context compression** — Tool Result Budget → Snip → Microcompact → HeadAndTail → De-dup → Autocompact (circuit-breaker protected)
- 💾 **Memory system** — session + global memory, per-turn extraction, cross-session AutoDream review
- 🎫 **Ticket workflow** — uploads and metadata gaps auto-create review tickets for human-in-the-loop
- 🔐 **Private deployment** — JWT + RBAC, Docker Compose, supports local llama.cpp inference

## Quick start

```bash
git clone https://github.com/int2t05/cognik.git
cd cognik

# Start PostgreSQL + pgvector
docker compose -f deploy/docker-compose.yml up -d postgres

# Start backend (:8080)
cd server && go run ./cmd/main.go

# Start frontend (:3000)
cd web && npm install && npm run dev
```

Open http://localhost:3000 — default account: `admin` / `Admin@123`

## Tech stack

| Layer | Technology |
|-------|-----------|
| Backend | Go + Gin + GORM |
| Database | PostgreSQL + pgvector (halfvec + HNSW) |
| RAG | Self-built Go engine — BM25 (gse) / vector (pgvector) / RRF / cross-encoder rerank / CRAG |
| LLM | Self-built `agent/llm.ChatModel` (net/http, any OpenAI-compatible API) |
| Frontend | Next.js + React + TypeScript + shadcn/ui + Tailwind v4 |
| Deploy | Docker Compose / All-in-One image |

## Project structure

```
server/
├── cmd/main.go              # entry
├── internal/
│   ├── agent/              # ReAct loop + tools + memory + compression
│   ├── domain/             # chat / knowledge / ticket / user / system
│   ├── rag/                # self-built RAG engine (14 files)
│   ├── parser/             # document parsing (MinerU + local fallback)
│   └── infra/              # adapter / config / database / storage / middleware
web/src/
├── app/                    # Next.js App Router
├── components/             # shadcn/ui
└── lib/api/                # API clients
docs/                       # PRD / TECH / ROADMAP / API / FLOW
deploy/                     # Docker Compose + All-in-One
```

## Documentation

| Doc | Purpose |
|------|---------|
| [ROADMAP.md](docs/ROADMAP.md) | Product & tech roadmap (V1.0 → V2.0) |
| [PRD.md](docs/PRD.md) | Product requirements |
| [TECH.md](docs/TECH.md) | Technical architecture & ADR |
| [API/](docs/API/README.md) | API contracts (5 modules) |
| [FLOW/](docs/FLOW/README.md) | Business flow diagrams (mermaid) |

## License

[MIT](LICENSE)
