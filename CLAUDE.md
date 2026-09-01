# CLAUDE.md — OpsMind Project Context

## 1. Role

You are a senior Go + Next.js full-stack engineer on OpsMind, bound to the actual stack: Gin / GORM / PostgreSQL + pgvector (halfvec + HNSW) / MinIO / a self-built Go RAG engine (`server/internal/rag/`) / gse Chinese tokenizer / Next.js / React / TypeScript / Radix UI / SWR / Docker Compose.

Deliver and iterate the ops digital employee system per `docs/PRD.md`, `docs/TECH.md`, and `docs/API/`.

## 2. Project

OpsMind is a private-deploy AI ops digital employee system for enterprise IT operations.

- **RAG-enhanced Q&A** — self-built pipeline: query rewrite → multi-route → hybrid (BM25 + vector + RRF) → rerank → LLM generation, with token-level SSE streaming and pipeline-step progress events.
- **Ticket workflow** — full state machine (待处理 → 处理中 → 需补充信息 → 已解决 / 已关闭), auto-close after 7 days, CAS-based concurrency guard.
- **Knowledge base** — unified article model (CRUD + review + publish), document upload (PDF/DOCX/MD/TXT) with async processing (parse → chunk → embed → pgvector), BM25 index rebuild on KB change.
- **RBAC** — JWT dual-token + permission codes, 4 preset roles, dynamic menu rendering.

Architecture: modular monolith, Handler → Service → Repository. RAG engine (`rag/`) is a self-contained domain engine with no HTTP-layer dependency. pgvector holds all vector data; PostgreSQL unifies business and vector storage. LLM/Embedding calls go through adapter interfaces to llama.cpp server or any OpenAI-compatible API.

## 3. Stack

- **Backend:** Go + Gin + GORM
- **Database:** PostgreSQL + pgvector (halfvec + HNSW index)
- **Object storage:** MinIO (S3-compatible)
- **RAG:** self-built Go engine — BM25 (gse tokenizer) / vector (pgvector) / RRF fusion / cross-encoder rerank (`rerank_server.py`)
- **LLM/Embedding:** llama.cpp server or OpenAI-compatible API, via adapter interface
- **Frontend:** Next.js + React + TypeScript + Radix UI + SWR + Tailwind CSS
- **Design system:** Apple Design (light/dark dual-theme CSS variables in `web/src/app/globals.css`)
- **Deployment:** Docker Compose — 4 required (opsmind-server, opsmind-web, postgres, minio) + 2 optional (llama-cpp, llama-cpp-emb under `ai-local` profile)

## 4. Structure

```
server/
├── cmd/main.go              # entry: config → DB → RAG → Service → Handler → Router → Scheduler
├── internal/
│   ├── adapter/             # LLMClient / EmbeddingClient / VectorStore(pgvector) / StorageClient(MinIO)
│   ├── cache/               # in-memory cache
│   ├── config/              # Viper config
│   ├── database/            # AutoMigrate + connection management
│   ├── dto/                 # request/ + response/
│   ├── handler/             # 11 Handlers (auth/user/role/chat/ticket/knowledge/llm_config/dashboard/config/audit/message)
│   ├── log/                 # structured logging
│   ├── middleware/          # JWT / RBAC / CORS / Logger
│   ├── model/               # GORM models + enums
│   ├── rag/                 # self-built RAG engine (pipeline/query_rewrite/multi_route/hybrid/bm25/rerank/chunker/embedder/retriever/processor/types)
│   ├── repository/          # 11 Repositories
│   ├── router/              # route registration + safeHandler
│   └── service/             # 12 Services + scheduler + tx_manager + generation_hub
├── pkg/                     # jwt / hash / crypto / response / errcode
├── migrations/              # DDL + seed data
├── models/                  # rerank model files
├── test/                    # external test packages (config/database/model/service/handler/middleware/adapter/rag/repository/router/e2e/integration)
└── rerank_server.py         # Python cross-encoder rerank service

web/src/
├── app/                     # Next.js App Router + globals.css (Apple Design Tokens)
├── components/              # ui/ + layout/ + shared/ + chat/
├── contexts/                # ChatStreamProvider
├── hooks/                   # 11 custom Hooks
├── lib/api/                 # 11 API client modules
└── __tests__/               # frontend unit tests

docs/                        # PRD / TECH / API / FLOW / TODO / audit
docker-compose.yml
```

## 5. Commands

```bash
# Backend (cd server)
go build ./cmd/...
go run ./cmd/main.go
go vet ./...
go test ./test/... -v -tags=integration -p 1   # requires PostgreSQL + pgvector + MinIO

# Frontend (cd web)
npm run dev      # port 3000, rewrite proxies to localhost:8080
npm run build
npm run lint

# Docker (project root)
docker compose up -d --build
docker compose --profile ai-local up -d --build   # with local llama.cpp

# DB seed (tables auto-created via GORM AutoMigrate on server startup)
docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_essential.sql
```

## 6. Project Conventions

- **Three-layer architecture:** Handler (param validation, response format) → Service (business logic, transactions) → Repository (data access). No cross-layer calls. RAG (`rag/`) does not depend on Handler/Service/Repository.
- **External calls via adapters only:** LLM, Embedding, VectorStore, StorageClient are accessed exclusively through interfaces in `server/internal/adapter/`. No direct HTTP calls to LLM/MinIO/pgvector from Service/Handler.
- **Chinese comments:** file-header comment per file (why the module exists), function comment per key function (why, not what).
- **Tests:** in `server/test/` using external test packages (`_test`); no mocks — tests run against real PostgreSQL + pgvector + MinIO. Use `-p 1` for integration tests to avoid cross-package DB contention.
- **Unified response envelope:** `{"code": 0, "message": "success", "data": {...}}` via `pkg/response`; error codes in `pkg/errcode`.
- **RBAC:** admin endpoints require JWT middleware + RBAC permission-code middleware.
- **Password validation:** `pkg/hash.ValidatePassword` enforces `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`.
- **Audit logs:** user/knowledge/ticket management operations write to `audit_logs`.
- **LLM config hot-swap:** `atomic.Value` in `LLMConfigManager` — config changes take effect without restart.
- **Apple Design:** light/dark dual-theme CSS variables; Action Blue `#006cc`; Inter Variable font; borderless flat cards; 17px body; pill-radius primary buttons. Tokens in `web/src/app/globals.css`.

## 7. Project Boundaries

**Always:**
- Read `docs/PRD.md` + `docs/TECH.md` before changing any interface or data model; confirm whether TECH.md needs sync after implementation.
- Handle external-service failures (timeout, unreachable, error return) for all LLMClient / EmbeddingClient / VectorStore / StorageClient calls.
- Enforce state machines in Service (ticket status transitions, knowledge article status transitions) — never `UPDATE status` directly.
- Use Docker-internal hostnames: `postgres:5432`, `minio:9000`, `http://llama-cpp:8080/v1` — never `localhost` for inter-container calls.

**Never:**
- Bypass the adapter layer to call LLM / MinIO / pgvector directly.
- Put business logic in Repository (audit rules, state-machine transitions, LLM hot-swap belong in Service).
- Hardcode config values (LLM Base URL, API Key, model names, vector dimensions, DB connection, JWT secret) — read from config/env.
- Skip RAG pipeline degradation logic (single-step failure must not block later steps; vector search and LLM generation are core-path — their failure returns an error, code 20002/20001).
- Silently swallow AI service failures (must return code 20001/20002/20003 with a clear message).
- Write mock tests.
- Auto-`git push` — every push requires human confirmation.

## 8. Formal Docs

| Doc | Purpose |
|-----|---------|
| `docs/PRD.md` | Product requirements — RAG engine, document upload, unified article model, SSE streaming |
| `docs/TECH.md` | Technical architecture — module interfaces, DDL, ADR, deployment config, design system appendix |
| `docs/API/README.md` | API docs index — 9 endpoint docs covering all routes |
| `docs/FLOW/README.md` | Business flow diagrams — 7 modules end-to-end data flow |
| `docs/TODO.md` | Improvement backlog and product roadmap (single source of truth) |
