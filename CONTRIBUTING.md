# Contributing to Cognik

Thanks for your interest in Cognik! Bug reports, feature suggestions, and code contributions are all welcome.

[简体中文](CONTRIBUTING.zh-CN.md)

## Development setup

```bash
git clone https://github.com/int2t05/Cognik.git
cd Cognik

# Start dependency services
docker compose up -d postgres minio

# Backend
cd server
go mod tidy
go run ./cmd/main.go

# Frontend (new terminal)
cd web
npm install
npm run dev
```

Default account: `admin` / `Admin@123`

## Code conventions

- **Go**: follow standard Go style (`go vet`, `golangci-lint`)
- **TypeScript**: follow the project ESLint config (`npm run lint`)
- **Comments**: comments in Chinese, explaining *why*, not restating code logic
- **Architecture**: Handler → Service → Repository three-layer separation; no cross-layer calls
- **API**: when changing an interface, keep `docs/API/` in sync
- **Docs**: after code changes, update the corresponding `docs/` to keep doc/impl consistent

## Commit conventions

Commit messages may be written in **Chinese or English**, in the format: `type: short description`

```
feat: implement BM25 hybrid retrieval
fix: fix pgvector batch write transaction
docs: update API docs
test: add ticket state machine tests
```

## Tests

```bash
# Go integration tests (requires PostgreSQL + pgvector)
cd server
go test ./test/... -v -tags=integration -p 1

# Frontend E2E tests (Playwright)
cd web
npx playwright test
```

Make sure all tests pass before submitting a PR.

## Pull request flow

1. Fork this repository
2. Create a feature branch (`feat/xxx` or `fix/xxx`)
3. Write code + tests
4. Ensure CI passes
5. Submit a PR to `main`
6. Wait for code review

## Feedback

- Bug reports: use the Bug Report issue template
- Feature suggestions: use the Feature Request issue template
- Security issues: do **not** open a public issue; contact the maintainers directly
