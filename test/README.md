# OpsMind 本地开发指南

## 环境要求

| 工具 | 版本 | 用途 |
|---|---|---|
| Go | 1.26+ | 后端编译运行 |
| Node.js | 20+ | 前端开发 |
| Docker Desktop | 8GB+ 内存 | PostgreSQL + llama.cpp |
| Python | 3.10+ | 重排序子进程（可选） |

## 快速启动

### 1. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，确认以下配置：
- PostgreSQL 连接（默认 localhost:5432，用户 opsmind，密码 opsmind_dev）
- LLM/Embedding 端点（本地 llama.cpp 或云端 API）
- 文件存储驱动（`OPSMIND_STORAGE_DRIVER=local` 默认本地存储）
- MinerU API Key（可选，不配置则降级到本地解析）

### 2. 启动 Docker 基础设施

```bash
# 方式一：Makefile（推荐）
make dev-ai    # 启动 PostgreSQL + llama.cpp（LLM + Embedding）

# 方式二：手动 docker compose
docker compose -f deploy/docker-compose.yml up -d postgres
docker compose -f deploy/docker-compose.yml --profile ai-local up -d llama-cpp llama-cpp-emb
```

**Docker 内存要求**：llama.cpp 加载 Qwen3-4B 需约 7GB 内存，Docker Desktop 至少分配 8GB（Settings → Resources → Memory）。

**模型文件**：`models/` 目录下需有 `Qwen3-4B-Q4_K_M.gguf` 和 `Qwen3-Embedding-0.6B-Q8_0.gguf`。首次使用需下载。

### 3. 启动后端 + 前端

```bash
# 方式一：Makefile 一键启动
make dev       # 后端（go run）+ 前端（npm run dev），热重载

# 方式二：分别启动
make dev-server  # 仅后端，http://localhost:8080
make dev-web     # 仅前端，http://localhost:3000
```

### 4. 验证服务

| 服务 | 地址 | 健康检查 |
|---|---|---|
| PostgreSQL | localhost:5432 | `docker compose -f deploy/docker-compose.yml ps` |
| LLM (llama.cpp) | http://localhost:8081/health | `curl http://localhost:8081/health` |
| Embedding | http://localhost:8082/health | `curl http://localhost:8082/health` |
| 后端 API | http://localhost:8080/health | `curl http://localhost:8080/health` |
| 前端 | http://localhost:3000 | 浏览器访问，重定向到 /login |

## 测试账号

种子数据在后端启动时自动加载（`AutoSeed`），无需手动执行 SQL。

| 用户名 | 密码 | 角色 | 权限 |
|---|---|---|---|
| `admin` | `Admin@123` | 系统管理员 | 全部权限（用户/工单/知识库/看板/审计/配置） |
| `operator1` | `Operator@123` | 运维人员 | 工单处理、知识库读写 |
| `operator2` | `Operator@123` | 运维人员 | 同上 |
| `knowledge` | `Knowledge@123` | 知识库管理员 | 知识库管理/审核 |
| `reporter1` | `Reporter@123` | 报障人 | 门户端（提交申告、智能问答） |
| `reporter2` | `Reporter@123` | 报障人 | 同上（无需首次改密） |

**首次登录**：除 `reporter2` 外，所有用户首次登录后需修改密码。

## 登录流程

1. 浏览器访问 `http://localhost:3000`
2. 自动重定向到 `/login`
3. 输入用户名和密码（如 `admin` / `Admin@123`）
4. 登录成功后：
   - 管理员角色 → 重定向到 `/admin/dashboard`
   - 普通用户 → 重定向到 `/portal/chat`

## 常用 Make 命令

```bash
make help          # 显示所有命令
make dev           # 一键启动（DB + 后端 + 前端）
make dev-ai        # 启动 DB + llama.cpp
make dev-db        # 仅启动 PostgreSQL
make dev-server    # 仅启动后端
make dev-web       # 仅启动前端
make build         # 编译后端 + 前端
make test          # 运行后端测试（需 PostgreSQL）
make test-unit     # 运行前端单元测试
make lint          # 代码检查（go vet + eslint）
make dev-stop      # 停止 Docker 服务
make dev-clean     # 停止并清除所有数据
```

## 文件存储

默认使用本地文件系统存储（`OPSMIND_STORAGE_DRIVER=local`），文档存储在 `data/storage/` 目录。

切换到 MinIO：
```bash
# .env 中设置
OPSMIND_STORAGE_DRIVER=minio
# 启动 MinIO 容器
docker compose -f deploy/docker-compose.yml --profile storage up -d minio
```

## 文档解析

默认使用本地纯 Go 库解析（PDF/DOCX/XLSX/PPTX/TXT/MD）。

配置 MinerU 云端高精度解析（可选）：
```bash
# .env 中设置
MINERU_API_KEY=sk-your-mineru-api-key
```

无 API Key 时自动降级到本地解析。旧版格式（DOC/XLS/PPT）需 Python `markitdown` 库。

## 端口分配

| 服务 | 端口 |
|---|---|
| 前端 (Next.js) | 3000 |
| 后端 (Go/Gin) | 8080 |
| LLM (llama.cpp) | 8081 |
| Embedding (llama.cpp) | 8082 |
| PostgreSQL | 5432 |
| MinIO API | 9000 |
| MinIO Console | 9001 |

## 故障排查

### LLM 容器反复重启
- 检查 Docker Desktop 内存是否 ≥8GB
- 检查 `models/` 目录是否有模型文件
- 查看日志：`docker logs opsmind-llama-cpp`

### 后端启动报数据库连接失败
- 确认 PostgreSQL 容器已启动：`docker compose -f deploy/docker-compose.yml ps`
- 确认 `.env` 中数据库配置正确

### 前端 API 请求 404
- 确认后端已启动：`curl http://localhost:8080/health`
- Next.js rewrite 代理 `/api/*` 到 `localhost:8080`

### 种子数据未加载
- AutoSeed 仅在 `roles` 表为空时执行
- 手动加载：`docker compose -f deploy/docker-compose.yml exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_essential.sql`
- 清空重建：`make dev-clean && make dev-ai && make dev`
