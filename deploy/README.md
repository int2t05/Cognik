# Cognik 部署

> 本地优先开发方案。Docker 仅用于基础设施（PostgreSQL + pgvector），后端和前端在宿主机直接运行。

## 目录结构

```
deploy/
├── docker-compose.yml       # 开发环境 Docker Compose（PostgreSQL + 可选 MinIO / llama.cpp）
├── docker/                  # 独立服务 Dockerfile（Docker Compose 全量构建用）
│   ├── server/
│   │   ├── Dockerfile        # Go 后端多阶段构建
│   │   └── .dockerignore
│   └── web/
│       └── Dockerfile        # Next.js standalone 多阶段构建
├── allinone/                 # All-in-One 单容器镜像（生产部署）
│   ├── Dockerfile
│   ├── supervisord.conf
│   └── entrypoint.sh
└── README.md
```

## 本地开发

### 前置条件

- Go 1.26+
- Node.js 22+
- Docker / Docker Compose
- Python 3（重排序模型下载用）

### 一键启动

```bash
# 1. 复制环境变量
cp .env.example .env

# 2. 安装依赖（首次）
make install

# 3. 下载重排序模型（首次，~80MB）
make rerank-model

# 4. 启动 PostgreSQL
make dev-db

# 5. 一键启动后端 + 前端
make dev
```

访问地址：
- 前端：http://localhost:3000
- 后端 API：http://localhost:8080

### 分终端启动

```bash
# 终端 1：PostgreSQL
make dev-db

# 终端 2：后端（热重载）
make dev-server

# 终端 3：前端（热重载）
make dev-web
```

### 本地 LLM（可选）

```bash
# 启动 PostgreSQL + llama.cpp（LLM + Embedding）
make dev-ai

# .env 中配置
# COGNIK_LLM_BASE_URL=http://localhost:8081/v1
# COGNIK_EMBEDDING_BASE_URL=http://localhost:8082/v1
```

### MinIO 对象存储（可选）

```bash
# 启动 PostgreSQL + MinIO
make dev-storage

# .env 中配置
# COGNIK_STORAGE_DRIVER=minio
```

### 停止 / 清理

```bash
make dev-stop      # 停止 Docker 服务
make dev-clean     # 停止并清除数据卷（谨慎）
```

## 生产部署

### All-in-One 单容器

```bash
# 1. 下载重排序模型
cd server && python models/rerank/download.py

# 2. 构建镜像（从项目根目录）
docker build -f deploy/allinone/Dockerfile -t cognik-allinone .

# 3. 运行
docker run -d --name cognik \
  -p 3000:3000 \
  -e JWT_SECRET=your_secret \
  -v cognik-data:/data \
  cognik-allinone
```

All-in-One 镜像内含：
- PostgreSQL 18 + pgvector
- MinIO
- Go 后端
- Next.js 前端
- rerank_server.py（cross-encoder 重排序）

首次启动自动 initdb + 建库，种子数据由 Go AutoSeed 加载。

### Docker Compose 全量

```bash
# 从项目根目录
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml --profile ai-local up -d
docker compose -f deploy/docker-compose.yml --profile storage --profile ai-local up -d
```

## 环境变量

见 [.env.example](../.env.example)，核心配置：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COGNIK_DATABASE_HOST` | localhost | PostgreSQL 地址 |
| `COGNIK_STORAGE_DRIVER` | local | 存储后端（local / minio） |
| `COGNIK_LLM_BASE_URL` | http://llama-cpp:8081/v1 | LLM API 地址 |
| `COGNIK_EMBEDDING_BASE_URL` | http://llama-cpp-emb:8082/v1 | Embedding API 地址 |
| `COGNIK_JWT_SECRET` | — | JWT 签名密钥（必须设置） |
