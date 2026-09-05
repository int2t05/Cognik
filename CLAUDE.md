# CLAUDE.md — Cognos Project Context

> Project-specific conventions only. Engineering principles (Purity, Ask-first, Push-back, Simplicity, etc.) are in `~/.claude/CLAUDE.md` and are not repeated here.

## 1. Role

You are a senior Go + Next.js full-stack engineer on Cognos, bound to the actual stack: Gin / GORM / PostgreSQL + pgvector (halfvec + HNSW) / shadcn/ui (Radix + Tailwind v4) / a self-built Go RAG engine (`server/internal/rag/`) / gse Chinese tokenizer / Next.js / React / TypeScript / SWR / Docker Compose.

## 2. Project

Cognos — a private-deploy AI knowledge management platform system for enterprise knowledge management.

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
- **Frontend:** Next.js + React + TypeScript + shadcn/ui (Radix + Tailwind v4) + SWR
- **Design system:** Linear/Vercel 专业工具风格（靛蓝 #5b5bd6 + zinc 灰阶，13px body，中性小圆角，亮暗双主题 CSS 变量 in `web/src/app/globals.css`）
- **Deployment:** Local-first dev (Makefile + Docker for PostgreSQL only) / `deploy/` for Docker Compose + All-in-One image

## 4. Structure

```
server/
├── cmd/main.go              # entry: config → DB → RAG → domain → router → runtime
├── internal/
│   ├── domain/              # 业务领域（每领域 handler + service + repository 三文件）
│   │   ├── chat/           # 聊天/AI 问答（chat + llm + llm_config）
│   │   ├── knowledge/      # 知识库
│   │   ├── system/         # 系统管理（audit + config + dashboard + message）
│   │   ├── ticket/         # 工单
│   │   └── user/           # 用户/权限（auth + role + user）
│   ├── infra/              # 基础设施
│   │   ├── adapter/        # LLMClient / EmbeddingClient / VectorStore / Reranker
│   │   ├── cache/          # 用户状态缓存
│   │   ├── config/         # Viper 配置
│   │   ├── database/       # AutoMigrate + 连接管理
│   │   ├── log/            # 结构化日志
│   │   ├── middleware/     # JWT / RBAC / CORS / Logger
│   │   ├── runtime/        # scheduler / tx_manager / generation_hub
│   │   └── storage/        # StorageClient 接口 + MinIO / Local 双实现（目录式）
│   ├── rag/                # 自建 RAG 引擎（pipeline/bm25/hybrid/rerank/chunker/embedder/processor）
│   ├── parser/             # 文档解析（parser.go + mineru/ + local/）
│   ├── router/             # 路由注册 + safeHandler
│   └── shared/             # 共享类型和工具
│       ├── dto/            # request/ + response/
│       ├── handler/        # 共享 handler 工具
│       ├── model/          # GORM models + enums
│       └── pkg/            # jwt / hash / crypto / response / errcode
├── migrations/              # DDL + seed data
├── models/                  # rerank model files
├── test/                    # 外部测试包（domain/infra/rag/shared/router/e2e/integration）
└── rerank_server.py         # Python cross-encoder rerank service

web/src/
├── app/                     # Next.js App Router + globals.css (Design Tokens)
├── components/              # ui/ + layout/ + shared/ + chat/
├── contexts/                # ChatStreamProvider
├── hooks/                   # 11 custom Hooks
├── lib/api/                 # 11 API client modules
└── __tests__/               # frontend unit tests

docs/                        # formal docs — see §8
deploy/                      # Docker 部署（docker-compose.yml + allinone/）
Makefile                     # 本地开发命令入口
```

## 5. Commands

```bash
# 一键开发（Docker DB + Go 后端 + Next.js 前端）
make dev          # 前台启动（3 个进程）
make dev-detached # 后台启动

# 分终端启动
make dev-db       # 仅 PostgreSQL + pgvector
make dev-server   # 仅后端（热重载）
make dev-web      # 仅前端（热重载）

# 本地 AI（可选）
make dev-ai       # PostgreSQL + llama.cpp (LLM + Embedding)
make dev-storage  # PostgreSQL + MinIO

# 构建 / 测试
make build        # 编译后端 + 前端
make test         # 后端集成测试
make lint         # 代码检查

# Docker 基础设施
make dev-stop     # 停止 Docker 服务
make dev-clean    # 停止并清除数据卷

# All-in-One 镜像
make docker-allinone  # 构建单容器生产镜像

# 手动命令
cd server && go run ./cmd/main.go          # 后端 :8080
cd web && npm run dev                      # 前端 :3000
cd server && go test ./test/... -v -tags=integration -p 1
```

## 6. Project Conventions

- **Three-layer architecture:** Handler (param validation, response format) → Service (business logic, transactions) → Repository (data access). No cross-layer calls. RAG (`rag/`) does not depend on Handler/Service/Repository.
- **External calls via adapters only:** LLM, Embedding, VectorStore are accessed exclusively through interfaces in `server/internal/infra/adapter/`. StorageClient is in `server/internal/infra/storage/`. No direct HTTP calls to LLM/MinIO/pgvector from Service/Handler.
- **Comment specification:** comments in concise Chinese, focusing on functionality; file-header comment required per file; function comment required per key function / method; no restating code logic — focus on functional explanation with a short Chinese example.
- **Tests:** in `server/test/` using external test packages (`_test`); no mocks — tests run against real PostgreSQL + pgvector + MinIO. Use `-p 1` for integration tests to avoid cross-package DB contention.
- **Unified response envelope:** `{"code": 0, "message": "success", "data": {...}}` via `internal/pkg/response`; error codes in `internal/pkg/errcode`.
- **RBAC:** admin endpoints require JWT middleware + RBAC permission-code middleware.
- **Password validation:** `internal/pkg/hash.ValidatePassword` enforces `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,32}$`.
- **Audit logs:** user/knowledge/ticket management operations write to `audit_logs`.
- **LLM config hot-swap:** `atomic.Value` in `LLMConfigManager` — config changes take effect without restart.
- **Design system:** Linear/Vercel 专业工具风格；靛蓝 `#5b5bd6`；system-ui 字体；中性小圆角；13px body。Tokens in `web/src/app/globals.css`。
- **Markdown 渲染安全:** `web/src/components/shared/Markdown.tsx` 是聊天/工单/文章共用的渲染入口，渲染 LLM/用户上传内容。`rehype-raw` 必须配 `rehype-sanitize`（`rehype-raw` 之后、`rehype-katex`/`rehype-highlight` 之前），schema 扩 `math` + `unwrapDisallowed` —— 防未知 HTML 标签崩页 + XSS。
- **前端 i18n:** `next-intl`（cookie 策略，无 `[locale]` URL 前缀）。消息文件 `web/messages/{zh,en}.json`，根 `useTranslations()` + 全路径键（如 `t('status.ticket.pending')`）。locale 解析：cookie → `Accept-Language` → 回退中文；`<html lang>` 服务端渲染，无水合闪烁。客户端错误用 `ApiError.messageKey`（全路径）+ `translateError(err, t, fallback)`；后端 `json.message` 原样透传（后端 i18n 超范围）。模块级 label 常量改为函数接收 `t`（如 `ticketStatusOptions(t)`），或改用 `labelKey` 在组件内翻译。

## 7. Project Boundaries

**Always:**

- Read `docs/PRD.md` + `docs/TECH.md` before changing any interface or data model; confirm whether TECH.md needs sync after implementation.
- Handle external-service failures (timeout, unreachable, error return) for all LLMClient / EmbeddingClient / VectorStore / StorageClient calls. StorageClient is in `server/internal/infra/storage/`.
- Enforce state machines in Service (ticket status transitions, knowledge article status transitions) — never `UPDATE status` directly.
- Use Docker-internal hostnames: `postgres:5432`, `minio:9000`, `http://llama-cpp:8080/v1` — never `localhost` for inter-container calls.

**Never:**

- Bypass the adapter layer to call LLM / MinIO / pgvector directly.
- Put business logic in Repository (audit rules, state-machine transitions, LLM hot-swap belong in Service).
- Hardcode config values (LLM Base URL, API Key, model names, vector dimensions, DB connection, JWT secret) — read from config/env.
- Skip RAG pipeline degradation logic (single-step failure must not block later steps; vector search and LLM generation are core-path — their failure returns an error, code 20002/20001).
- Silently swallow AI service failures (must return code 20001/20002/20003 with a clear message).
- Write mock tests.
- Auto-`git commit` / `git push` — only on user's explicit instruction.

## 8. Formal Docs

Project-level source of truth on `main` branch. Changes require audit.

| Doc                     | Purpose                                                |
| ----------------------- | ------------------------------------------------------ |
| `docs/ROADMAP.md`     | 产品技术路线图 — 战略方向、里程碑、技术决策记录       |
| `docs/PRD.md`         | 产品需求 — RAG 引擎、文档上传、统一文章模型、SSE 流式 |
| `docs/TECH.md`        | 技术架构 — 模块接口、DDL、ADR、部署配置、设计系统附录 |
| `docs/API/README.md`  | API 文档索引 — 9 份端点文档覆盖全部路由               |
| `docs/FLOW/README.md` | 业务流程图 — 7 大模块端到端数据流                     |
| `docs/TODO.md`        | 代码级改进清单与优先级                                 |

---

## Core Discipline (highest authority — overrides on conflict)

### Artifacts require user consent

All artifacts (docs / files / research / audit) — generation, creation, path, structure — require explicit user consent. Research / audit tasks: confirm artifact form and location before producing; never create unsolicited.

- "可以 / 都行" is not consent — keep asking until ~95% clear, one question at a time.
- Artifacts are fixed: do not create / rename artifact files at will; merge research into existing files, do not create dated files.

### No silent gap-filling

When requirements are unclear or multi-solution, stop and ask. Do not guess.

### Docs first

Spec before code; plan before implementation.

### Simplicity

Abstractions must earn their complexity. Product-level docs define "what + acceptance criteria" only; interactions / selections / thresholds sink to `docs/vX.Y/PRD.md` and `TECH.md`.

### Evidence over assertion

Claims carry citations. Negative claims: falsify via GitHub. If unfalsifiable, mark `UNVERIFIED:`. Distinguish fact from inference.

### Design must be grounded

Design phase (DESIGN / TECH and intermediates): do not design blind. Clone comparable open-source repos to `reference/` (git-ignored) for local research. Annotate each design decision with its source (repo / file / pattern).

### Chinese-first

Formal docs and communication in Chinese; prefer mermaid diagrams. (Project-specific CLAUDE.md content is in English for template consistency; identifiers remain in original.)

### Git discipline

- **Branch:** `main` holds formal-version files; develop on branches; every commit is auditable.
- **One block per commit:** commits are scoped to one feature / function; related changes go in one commit, not split into fragments. Multiple rounds of the same feature merge into the existing commit (`--amend` or as user directs), not new commits per round. Different features / functions are separate commits.
- **No auto commit / push:** commit and push are triggered by the user manually. After completing work, report status and diff only; wait for explicit instruction.
- **Research artifacts are user-specified:** the output file, path, and structure form of research tasks are not self-determined — confirm with the user first, do not create research files unsolicited.

## Artifact Paths (fixed — do not create / rename at will)

| Phase        | Artifact                                            | Path                                                             |
| ------------ | --------------------------------------------------- | ---------------------------------------------------------------- |
| Research     | Interview / market / competitor / strategy          | `docs/research/{interview,market,competitor,strategy}.md`      |
| Planning     | Global view (authoritative)                         | `docs/ROADMAP.md`                                              |
| Planning     | Project icon                                        | `docs/assets/icon-dark.svg`                                    |
| Requirements | Project-level PRD (concise)                         | `docs/PRD.md`                                                  |
| Architecture | Project-level TECH (concise)                        | `docs/TECH.md`                                                 |
| API          | API contracts                                       | `docs/API/*.md`                                                |
| Plan         | Project-level plan (concise)                        | `docs/PLAN.md`                                                 |
| Plan         | Version-level plan (ticket breakdown)               | `docs/vX.Y/plan.md`                                            |
| Flow         | Business flows (mermaid + data flow)                | `docs/FLOW/*.md`                                               |
| Review       | TODO (code ↔ TODO.md bidirectional check)          | `docs/TODO.md`                                                 |
| Load test    | Capacity report                                     | `docs/CAPACITY.md`                                             |
| Security     | Security findings                                   | `docs/security-report.md`                                      |
| Incident     | Blameless postmortem                                | `docs/postmortem/YYYY-MM-DD-<slug>.md`                         |
| Audit        | Doc audit (git-ignored, local snapshot)             | `docs/audit/`                                                  |

- Project-level (concise, goes to `main`) vs. version-level (detailed, goes to `docs/vX.Y/`). Single-version projects fall back to project-level only.
- `docs/audit/` is git-ignored (see `.gitignore`), does not enter version control.
- Design references: clone comparable open-source repos to `reference/` (git-ignored), use for local design grounding, does not enter version control.

## Artifact Purity (audit requirements)

- **Provenance residue:** no version / date / event labels in body text (e.g. "v2 新增", "已观测", "保留"); evolution belongs in git log, body states current facts only. Group by dimension, not by add-batch.
- **Decorative redundancy:** every diagram / comment carries routing / decision / timing / structure / why information; if deleting it loses nothing for the reader, it is decorative. Benchmark / stars are background, not selling points.
- **Dead content:** no dead directives (claiming "统一 / 必须" but body doesn't follow), no dead links, no dead fields (no consumers).
- **Doc/impl gap:** validation items ↔ actions correspond; artifact paths ↔ manifests synced; cross-instance shared structures use consistent wording; numbering continuous without gaps.
- **Language / platform residue:** body language consistent (identifiers in original excepted); platform / tool implementation details do not enter general rules.
