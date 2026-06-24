# Contributing to OpsMind

感谢你对 OpsMind 的关注！无论是 Bug 报告、功能建议还是代码贡献，都非常欢迎。

## 开发环境设置

```bash
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind

# 启动依赖服务
docker compose up -d postgres minio

# 后端
cd server
go mod tidy
go run ./cmd/main.go

# 前端（新终端）
cd web
npm install
npm run dev
```

## 代码规范

- **Go**：遵循标准 Go 风格（`go vet`、`golangci-lint`）
- **TypeScript**：遵循项目 ESLint 配置（`npm run lint`）
- **注释**：中文注释，解释"为什么这样做"而非重复代码逻辑
- **架构**：Handler → Service → Repository 三层分离，不允许跨层调用
- **API**：变更接口时同步更新 `docs/API/` 文档

## 提交规范

使用中文 commit message，格式：`类型: 简短描述`

```
feat: 实现 BM25 混合检索
fix: 修复 pgvector 批量写入事务
docs: 更新 API 文档
test: 添加申告状态机测试
```

## 测试

```bash
# Go 集成测试（需 PostgreSQL + pgvector）
cd server
go test ./tests/... -v -tags=integration -p 1

# 前端 E2E 测试（Playwright）
cd web
npx playwright test
```

提交 PR 前请确保所有测试通过。

## Pull Request 流程

1. Fork 本仓库
2. 创建功能分支（`feat/xxx` 或 `fix/xxx`）
3. 编写代码 + 测试
4. 确保 CI 通过
5. 提交 PR 到 `main` 分支
6. 等待 Code Review

## 问题反馈

- Bug 报告：使用 Bug Report Issue 模板
- 功能建议：使用 Feature Request Issue 模板
- 安全问题：请勿公开 Issue，直接联系维护者
