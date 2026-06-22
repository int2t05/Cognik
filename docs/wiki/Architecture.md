# 🏗️ 架构概览

## 设计哲学

OpsMind 采用 **单体分层架构（Modular Monolith）**，而非微服务。选择理由：

- **MVP 阶段复杂度可控** — 所有模块在一个进程中，调试和部署简单
- **模块边界清晰** — Handler → Service → Repository 三层分离，未来可拆分
- **RAG 引擎自包含** — `rag/` 包不依赖 HTTP 层，可独立测试和演进

## 系统架构图

```
客户端 (Next.js App Router)
  │
  ├─ HTTP/REST + SSE ──▶ Go 后端 (Gin, 端口 8080)
  │                        │
  │                        ├─ Middleware 层 — JWT / RBAC / CORS / Logger
  │                        ├─ Handler 层 — 参数校验、响应格式
  │                        ├─ Service 层 — 业务逻辑、事务编排
  │                        ├─ Repository 层 — GORM 数据访问
  │                        ├─ RAG 引擎 (rag/) — 检索管道、文档处理
  │                        └─ Adapter 层 — LLM / Embedding / pgvector / MinIO
  │                             │
  └─────────────────────────────┼────────────────────────
                                ▼
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                  ▼
      pgvector/pg18           MinIO          llama.cpp (可选)
      (业务数据+向量)        (对象存储)      (OpenAI-compat API)
```

## 分层详解

### Middleware 层

请求管道中按序执行的横切关注点：

| 中间件 | 职责 |
|--------|------|
| RequestID | 为每个请求生成唯一追踪 ID |
| Logger | 记录请求方法、路径、耗时、状态码 |
| CORS | 跨域控制（debug 模式自动允许所有来源） |
| JWT Auth | 验证 Bearer Token，解析用户身份注入 Context |
| RBAC | 校验用户权限码是否匹配路由所需权限 |

### Handler 层

- 负责 HTTP 请求解析、参数校验、调用 Service、格式化响应
- 11 个 Handler 模块，每个对应一个业务域
- 共享工具函数：`parsePagination`、`parseID`、`getCurrentUserID`、`handleServiceError`
- 所有响应统一格式：`{"code": 0, "message": "success", "data": {}}`

### Service 层

- 核心业务逻辑所在，编排跨 Repository 的事务操作
- **消费者接口模式** — 每个 Service 定义它所需的依赖接口（Go "accept interfaces, return structs"）
- LLM 配置通过 `atomic.Value` 实现热替换，修改后即时生效无需重启
- `LLMService` 统一编排 RAG 检索 + prompt 构建 + LLM 调用（支持流式/同步两种路径）

### Repository 层

- 纯数据访问，使用 GORM 操作 PostgreSQL
- 不包含任何业务逻辑
- 通用泛型分页函数 `Paginate[T]` 消除各 Repo 重复代码

### RAG 引擎

详见 [🧠 RAG 引擎](RAG-Engine) 专题文档。

### Adapter 层

封装外部协议，通过接口隔离实现细节：

| 接口 | 实现 | 说明 |
|------|------|------|
| `LLMClient` | `OpenAIClient` | ChatCompletion + ChatCompletionStream |
| `EmbeddingClient` | `OpenAIEmbeddingClient` | `/v1/embeddings` 端点 |
| `VectorStore` | `PgvectorStore` | pgvector 批量写入 / 余弦相似度搜索 / 删除 |
| `StorageClient` | `MinIOClient` | 对象上传 / 下载 / presigned URL / 删除 |

### 跨层调用规则

```
✅ 允许：
  Handler → Service → Repository
  Service → RAG Engine
  Service → Adapter
  RAG Engine → Adapter

❌ 禁止：
  Handler → Repository（必须经过 Service）
  RAG Engine → Handler / Service（RAG 引擎不依赖 HTTP 层）
  Service → 直接 HTTP 调用外部服务（必须经过 Adapter）
```

## 项目结构

```
OpsMind/
├── server/                  # Go 后端
│   ├── cmd/main.go          # 入口：DI 装配 → 路由注册 → 调度器 → 优雅关闭
│   ├── internal/
│   │   ├── config/          # Viper 配置加载
│   │   ├── database/        # GORM 连接 + AutoMigrate
│   │   ├── middleware/       # JWT / RBAC / CORS / Logger / RequestID
│   │   ├── router/          # 路由注册（public / portal / admin 三组）
│   │   ├── handler/         # HTTP Handler（11 个模块）
│   │   ├── service/         # 业务逻辑 + 事务编排 + 调度器
│   │   ├── repository/      # 数据访问（含泛型分页）
│   │   ├── model/           # GORM 数据模型
│   │   ├── rag/             # RAG 引擎（自包含，不依赖 HTTP 层）
│   │   ├── adapter/         # 外部适配层（LLM / Embedding / pgvector / MinIO）
│   │   └── dto/             # 请求/响应 DTO
│   ├── pkg/                 # 公共工具包（errcode / jwt / hash / response）
│   ├── migrations/          # 数据库迁移 + 种子数据
│   └── tests/               # 集成测试（外部测试包 _test）
├── web/                     # Next.js 前端
│   └── src/
│       ├── app/             # App Router（auth / portal / admin 三组布局）
│       ├── components/
│       │   ├── ui/          # Apple Design 原子组件（14 个）
│       │   ├── layout/      # 布局组件（AdminLayout / PortalLayout）
│       │   └── shared/      # 复合组件（StatCard / StatusBadge / ConfirmDialog）
│       ├── lib/             # 工具函数 + API 客户端封装（12 个模块）
│       ├── hooks/           # React Hooks（useAuth / useTheme / useToast 等）
│       └── styles/          # Apple Design Tokens（浅色/暗色双主题）
├── docs/                    # 项目文档（PRD / TECH / API / 架构图）
├── docker-compose.yml       # Docker Compose 编排
├── Makefile                 # 开发命令入口
└── .env.example             # 环境变量模板
```

## 路由设计

API 按认证要求分为三组：

| 路由组 | 前缀 | 认证 | 说明 |
|--------|------|------|------|
| Public | `/api/v1/auth` | 无 | 登录、刷新令牌 |
| Portal | `/api/v1/portal` | JWT | 智能问答（SSE 流式）、申告提交、进度查询、站内消息 |
| Admin | `/api/v1/admin` | JWT + RBAC | 申告处理、知识库管理、LLM 配置、用户/角色、看板、审计 |

## 关键设计决策（ADR）

### 为什么用 pgvector 而非专用向量数据库？

PostgreSQL 已存储业务数据，引入 Milvus/Qdrant 等专用向量库会增加运维复杂度。pgvector 的 HNSW 索引 + halfvec 半精度在 10 万级向量场景下查询 < 50ms，满足 MVP 需要。

### 为什么自建 RAG 而非用 LangChain？

LangChain 的抽象层次过高，调试困难且引入 Python 依赖。自建 Go 管道每个步骤都是明确的函数调用，状态可观测、可审计。

### 为什么用 gse 而非 jieba？

gse 是纯 Go 实现，无需 CGO，交叉编译和 Docker 构建零额外依赖。jieba 需要 C++ 扩展，在 Go 中集成复杂。

### 为什么用 SSE 而非 WebSocket？

智能问答是单向流式输出（服务端 → 客户端）。SSE 基于 HTTP 协议，实现简单，浏览器原生支持 `EventSource`，更适合 LLM token 级别的流式场景。
