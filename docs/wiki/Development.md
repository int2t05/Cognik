# 💻 开发指南

## 环境搭建

### 前置要求

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.26+ | 后端语言 |
| Node.js | 20+ | 前端运行时 |
| Docker | 24+ | 运行 PostgreSQL + MinIO 依赖 |
| Git | 2.40+ | 版本控制 |

### 本地开发环境

```bash
# 1. 克隆仓库
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind

# 2. 启动依赖服务
docker compose up -d postgres minio

# 3. 配置环境变量
cp .env.example .env
# 编辑 .env，将 localhost 保持为数据库和 MinIO 的主机地址

# 4. 启动后端（新终端）
cd server
go mod tidy
go run ./cmd/main.go
# 后端运行在 http://localhost:8080

# 5. 启动前端（新终端）
cd web
npm install
npm run dev
# 前端运行在 http://localhost:3000
```

前端开发服务器自动将 API 请求代理到 `localhost:8080`（配置在 `web/next.config.js`）。

## 项目架构

详见图 [🏗️ 架构概览](Architecture)。核心原则：

### 三层架构

```
Handler → Service → Repository
  ↑         ↑          ↑
  参数校验   业务逻辑    数据访问
```

- **不允许跨层调用** — Handler 不能直接调用 Repository
- **Repository 不含业务逻辑** — 只做纯数据访问
- **Service 定义消费者接口** — 遵循 "accept interfaces, return structs"

### RAG 引擎独立性

`server/internal/rag/` 是自包含的领域引擎：
- 不依赖 HTTP 层（Handler/Service/Repository）
- 只依赖 Adapter 接口（LLMClient、EmbeddingClient、VectorStore）
- 可独立单元测试，不启动 HTTP 服务

### Adapter 抽象

所有外部服务访问必须通过 Adapter 接口：
- `LLMClient` — LLM 调用
- `EmbeddingClient` — Embedding 生成
- `VectorStore` — pgvector 操作
- `StorageClient` — MinIO 操作

## 代码规范

### Go 后端

```bash
# 静态检查
go vet ./...

# Lint（如安装了 golangci-lint）
golangci-lint run ./...

# 格式化
go fmt ./...
```

- 遵循标准 Go 代码风格
- 文件头注释：说明模块存在的原因
- 函数注释：说明为什么这样实现，而非重复代码逻辑
- 所有注释使用中文

### TypeScript 前端

```bash
# Lint
cd web
npm run lint

# 类型检查
npx tsc --noEmit
```

- 遵循项目 ESLint 配置
- React 组件使用 TypeScript
- UI 组件遵循 Apple Design System 约束

### Git 提交规范

中文提交信息，格式：`类型: 简短描述`

```
feat: 实现 BM25 混合检索
fix: 修复 pgvector 批量写入事务
docs: 更新 API 文档
test: 添加申告状态机测试
refactor: 抽取通用分页函数 Paginate[T]
chore: 更新 Go 依赖版本
```

## 测试

### Go 测试

全部测试位于 `server/tests/` 目录，使用外部测试包 `_test`。

```bash
cd server

# 运行全部测试（需 PostgreSQL + pgvector + MinIO）
go test ./tests/... -v -tags=integration -p 1

# 运行指定模块测试
go test ./tests/rag/... -v -tags=integration       # RAG 引擎
go test ./tests/model/... -v -tags=integration     # 数据模型
go test ./tests/database/... -v -tags=integration  # 数据库
go test ./tests/service/... -v -tags=integration   # 业务逻辑
go test ./tests/handler/... -v -tags=integration   # HTTP Handler
go test ./tests/middleware/... -v -tags=integration # 中间件
go test ./tests/adapter/... -v -tags=integration   # 适配层
```

> `-p 1` 标志确保串行执行，避免跨包并行测试共享数据库冲突。

### 前端 E2E 测试

```bash
cd web
npx playwright test
```

## 新增功能开发流程

1. **阅读文档** — `docs/PRD.md` 对应需求 → `docs/TECH.md` 对应章节 → `docs/API/` 对应接口
2. **确定影响范围** — 是否需要新增 Handler / Service / Repository？是否修改 RAG 管道？
3. **编写代码** — 遵守三层架构、通过 Adapter 访问外部服务
4. **编写测试** — 在 `server/tests/` 下创建测试文件（外部测试包 `_test`）
5. **更新文档** — 同步更新 `docs/API/` 对应接口文档
6. **提交前检查** — `go vet`、`npm run lint`、全部测试通过

## 接口开发约定

### 统一响应格式

所有 Handler 必须使用 `pkg/response` 封装响应：

```go
// 成功
response.Success(c, data)

// 分页成功
response.SuccessWithPage(c, list, total, page, pageSize)

// 业务错误
response.Error(c, errcode.InvalidParam)

// 带自定义消息的错误
response.ErrorWithMsg(c, errcode.InvalidParam, "用户名不能为空")
```

### 错误码

所有业务错误使用 `pkg/errcode` 中定义的错误码，禁止硬编码错误信息。新增错误码需同步更新 `docs/API/README.md` 中的错误码表。

## 常用命令速查

```bash
# 后端
cd server
go mod tidy                  # 安装/更新依赖
go build ./cmd/...           # 编译
go run ./cmd/main.go         # 运行
go vet ./...                 # 静态检查
go test ./tests/... -v -tags=integration -p 1  # 全部测试

# 前端
cd web
npm install                  # 安装依赖
npm run dev                  # 启动开发服务器
npm run build                # 构建生产版本
npm run lint                 # Lint + 类型检查

# Docker
docker compose up -d --build              # 启动全部
docker compose --profile ai-local up -d   # 启动含 llama.cpp
docker compose ps                          # 查看状态
docker compose logs -f opsmind-server      # 查看日志
docker compose down                        # 停止
```
