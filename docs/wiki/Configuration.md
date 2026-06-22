# ⚙️ 配置指南

OpsMind 使用 `.env` 文件和环境变量进行配置。完整模板见仓库根目录 `.env.example`。

## 快速配置

```bash
# 复制模板
cp .env.example .env

# 必须修改的项：
# 1. JWT 密钥（生产必改）
OPSMIND_JWT_SECRET=change_me_to_random_64_chars   # 建议: openssl rand -hex 32

# 2. LLM API（二选一）
# 方案 A：云端 API
OPSMIND_LLM_BASE_URL=https://api.openai.com/v1
OPSMIND_LLM_API_KEY=sk-xxxxxxxxxxxxxxxx

# 方案 B：本地 llama.cpp（留空 API Key）
OPSMIND_LLM_BASE_URL=http://llama-cpp:8080/v1
OPSMIND_LLM_API_KEY=
```

## 全部配置项

### 数据库

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_DATABASE_HOST` | PostgreSQL 主机 | `localhost` |
| `OPSMIND_DATABASE_PORT` | PostgreSQL 端口 | `5432` |
| `OPSMIND_DATABASE_USER` | 数据库用户 | `opsmind` |
| `OPSMIND_DATABASE_PASSWORD` | 数据库密码 | `opsmind_dev` |
| `OPSMIND_DATABASE_DBNAME` | 数据库名 | `opsmind` |
| `OPSMIND_DATABASE_SSLMODE` | SSL 模式 | `disable` |
| `POSTGRES_DB` | PostgreSQL 容器数据库名 | `opsmind` |
| `POSTGRES_USER` | PostgreSQL 容器用户 | `opsmind` |
| `POSTGRES_PASSWORD` | PostgreSQL 容器密码 | `opsmind_dev` |

> **Docker Compose 注意：** 容器内 OPSMIND_DATABASE_HOST 自动覆写为 `postgres`，本地开发使用 `localhost`。

### MinIO 对象存储

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MINIO_ROOT_USER` | MinIO 管理员用户 | `minioadmin` |
| `MINIO_ROOT_PASSWORD` | MinIO 管理员密码 | `minioadmin` |
| `OPSMIND_MINIO_ENDPOINT` | MinIO 端点 | `localhost:9000` |
| `OPSMIND_MINIO_ACCESS_KEY` | MinIO 访问密钥 | `minioadmin` |
| `OPSMIND_MINIO_SECRET_KEY` | MinIO 秘密密钥 | `minioadmin` |
| `OPSMIND_MINIO_USE_SSL` | 是否使用 SSL | `false` |

### JWT 认证

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_JWT_SECRET` | JWT 签名密钥（生产环境必须修改） | 无，必须设置 |
| `OPSMIND_JWT_ACCESS_EXPIRE` | Access Token 有效期 | `2h` |
| `OPSMIND_JWT_REFRESH_EXPIRE` | Refresh Token 有效期 | `168h`（7 天） |

### LLM（大语言模型）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_LLM_BASE_URL` | LLM API 地址（需含 `/v1` 路径） | `http://llama-cpp:8080/v1` |
| `OPSMIND_LLM_API_KEY` | API 密钥（llama.cpp 留空） | — |
| `OPSMIND_LLM_MODEL` | 模型名称 | `Qwen3-4B-Q4_K_M` |
| `OPSMIND_LLM_MAX_TOKENS` | 最大生成 Token 数 | `8192` |

### Embedding（向量嵌入）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_EMBEDDING_BASE_URL` | Embedding API 地址 | 空（回退到 LLM_BASE_URL） |
| `OPSMIND_EMBEDDING_API_KEY` | API 密钥 | 空（回退到 LLM_API_KEY） |
| `OPSMIND_EMBEDDING_MODEL` | Embedding 模型名称 | `Qwen3-Embedding-0.6B-Q8_0` |
| `OPSMIND_EMBEDDING_DIMENSION` | 向量维度 | `1024` |

### RAG 管道

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_AI_DEFAULT_TOP_K` | 默认检索 Top K | `5` |
| `OPSMIND_AI_CONFIDENCE_THRESHOLD` | 置信度阈值（低于此值引导提交申告） | `0.6` |
| `OPSMIND_AI_MAX_HISTORY_MESSAGES` | 最大会话历史消息数 | `10` |
| `OPSMIND_AI_RAG_QUERY_REWRITE` | 启用查询改写 | `true` |
| `OPSMIND_AI_RAG_MULTI_ROUTE` | 启用多路检索 | `true` |
| `OPSMIND_AI_RAG_HYBRID` | 启用 BM25 混合检索 | `true` |
| `OPSMIND_AI_RAG_RERANK` | 启用重排序 | `true` |

### 重排序

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_RERANK_ENABLED` | 启用 cross-encoder 重排序 | `true` |

### 服务与 CORS

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OPSMIND_SERVER_PORT` | 服务端口 | `8080` |
| `OPSMIND_SERVER_MODE` | 运行模式（`debug` / `release`） | `debug` |
| `OPSMIND_CORS_ALLOW_ORIGINS` | CORS 允许域名（逗号分隔） | `http://localhost:5173,http://localhost:3000` |

### llama.cpp（仅 ai-local profile）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LLAMA_MODELS_DIR` | 模型文件目录 | `./models` |

## 配置热替换

LLM 配置（Base URL、API Key、模型名称等）支持通过后台管理 UI 动态修改，通过 `atomic.Value` 原子操作即时生效：

- 修改后**无需重启服务**
- 不影响正在进行的请求
- 新请求自动使用新配置
- 历史会话不受影响

只有 `.env` 中的基础设施配置（数据库连接、MinIO 端点、JWT 密钥等）修改后需要重启。

## 模型推荐

### 对话模型

| 模型 | 参数量 | 量化 | 适用场景 |
|------|--------|------|----------|
| Qwen3-4B | 4B | Q4_K_M | 通用对话，推荐 |
| Qwen3-8B | 8B | Q4_K_M | 复杂推理，需更多显存 |
| DeepSeek-R1-Distill-Qwen-7B | 7B | Q4_K_M | 推理能力更强 |

### Embedding 模型

| 模型 | 维度 | 适用场景 |
|------|------|----------|
| bge-m3 | 1024 | 多语言，推荐 |
| Qwen3-Embedding-0.6B | 1024 | 轻量级，与 Qwen3 对话模型配套 |
| text-embedding-3-small | 1536 | OpenAI API |

> 选择 Embedding 模型时，维度必须与 `OPSMIND_EMBEDDING_DIMENSION` 配置一致。
