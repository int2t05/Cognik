# 🚀 快速开始

本指南帮助你从零开始启动 OpsMind。预计耗时：**10 分钟**。

## 前置要求

| 依赖 | 最低版本 | 说明 |
|------|----------|------|
| Docker + Docker Compose | Docker 24+ | 必须，用于运行全部服务 |
| Go | 1.26+ | 仅本地开发需要 |
| Node.js | 20+ | 仅本地开发需要 |
| Git | 2.40+ | 克隆仓库 |

> **硬件建议：** 4 核 CPU + 16 GB 内存（含 LLM 推理）；仅 API 模式可降至 2 核 4 GB。

## 方式一：Docker Compose（推荐）

这是最快的方式，一条命令启动全部服务。

```bash
# 1. 克隆仓库
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind

# 2. 复制环境变量模板
cp .env.example .env

# 3. 修改 .env 中的关键配置
#    - OPSMIND_JWT_SECRET：必须改为随机字符串（推荐 openssl rand -hex 32）
#    - OPSMIND_LLM_BASE_URL：LLM API 地址（见下方说明）

# 4. 一键启动（4 个必须服务）
docker compose up -d --build
```

启动后访问：
- **前端：** http://localhost:3000
- **后端 API：** http://localhost:8080
- **MinIO 控制台：** http://localhost:9001

### LLM 配置说明

OpsMind 支持两种 LLM 模式：

**模式 A：使用云端 API（最简单）**

编辑 `.env`，填入你的 API 凭据：

```bash
OPSMIND_LLM_BASE_URL=https://api.openai.com/v1   # 或 DeepSeek / Moonshot 等
OPSMIND_LLM_API_KEY=sk-xxxxxxxxxxxxxxxx
OPSMIND_LLM_MODEL=gpt-4o
OPSMIND_EMBEDDING_BASE_URL=https://api.openai.com/v1
OPSMIND_EMBEDDING_API_KEY=sk-xxxxxxxxxxxxxxxx
OPSMIND_EMBEDDING_MODEL=text-embedding-3-small
OPSMIND_EMBEDDING_DIMENSION=1536
```

**模式 B：使用本地 llama.cpp（离线，无需 API Key）**

```bash
# 启动含 llama.cpp 的完整环境
docker compose --profile ai-local up -d --build

# .env 保持默认即可（已指向 llama-cpp 容器）
# OPSMIND_LLM_BASE_URL=http://llama-cpp:8080/v1
# OPSMIND_LLM_API_KEY=
```

> 本地模型文件需放入 `./models` 目录。推荐使用 Qwen3-4B（对话）+ bge-m3 或 Qwen3-Embedding（embedding）。

### 验证安装

```bash
# 检查服务状态
docker compose ps

# 预期输出：4 个服务均为 Up 状态
# NAME              STATUS
# opsmind-server    Up
# opsmind-web       Up
# opsmind-postgres  Up (healthy)
# opsmind-minio     Up
```

访问 http://localhost:3000，应能看到登录页面。使用默认管理员账号登录：

| 用户名 | 密码 |
|--------|------|
| `admin` | `Admin@123456` |

> ⚠️ 生产环境请立即修改默认密码！

## 方式二：本地开发模式

适合需要修改源码的开发者。

```bash
# 1. 启动依赖服务（PostgreSQL + MinIO）
docker compose up -d postgres minio

# 2. 启动后端（新终端）
cd server
cp config/config.example.yaml config/config.yaml  # 如需要
go mod tidy
go run ./cmd/main.go

# 3. 启动前端（新终端）
cd web
npm install
npm run dev
```

前端开发服务器默认运行在 http://localhost:3000，API 请求自动代理到 `localhost:8080`。

## 下一步

- 阅读 [🏗️ 架构概览](Architecture) 理解系统设计
- 查看 [⚙️ 配置指南](Configuration) 了解所有配置项
- 在后台管理界面中上传你的第一份知识文档
- 试试智能问答功能
