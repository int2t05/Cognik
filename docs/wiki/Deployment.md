# 🐳 部署指南

## Docker Compose 部署（推荐）

### 架构

```
docker compose up -d --build
        │
        ├── opsmind-server (Go, :8080)
        ├── opsmind-web    (Next.js, :3000)
        ├── postgres       (pgvector/pg18, :5432)
        ├── minio          (S3-compatible, :9000/:9001)
        └── llama-cpp      (可选, --profile ai-local, :8081)
```

### 1. 准备环境

```bash
# 确保 Docker 24+ 和 Docker Compose 已安装
docker --version
docker compose version

# 克隆仓库
git clone https://github.com/int2t05/OpsMind.git
cd OpsMind
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

**必须修改的项：**

```bash
# JWT 密钥（生成随机字符串）
OPSMIND_JWT_SECRET=$(openssl rand -hex 32)

# LLM 配置 — 选择适合你的方案
```

### 3. 启动服务

**方案 A：使用云端 API（简单，无需 GPU）**

```bash
# 编辑 .env 填入 API 凭据后启动
docker compose up -d --build
```

**方案 B：使用本地 llama.cpp（离线，需要 GPU 或足够内存）**

```bash
# 1. 下载模型文件到 ./models 目录
mkdir -p models
# 将 GGUF 格式的对话模型和 embedding 模型放入 models/

# 2. 启动完整环境
docker compose --profile ai-local up -d --build
```

### 4. 验证

```bash
docker compose ps

# 预期输出全部 Up：
# NAME              STATUS
# opsmind-server    Up
# opsmind-web       Up
# opsmind-postgres  Up (healthy)
# opsmind-minio     Up
```

访问 http://localhost:3000 查看前端。

### 5. 管理命令

```bash
# 查看日志
docker compose logs -f opsmind-server
docker compose logs -f opsmind-web

# 重启单个服务
docker compose restart opsmind-server

# 停止全部服务
docker compose down

# 停止并清除数据（⚠️ 不可恢复）
docker compose down -v
```

## 生产环境部署

### 安全加固清单

- [ ] 修改 `OPSMIND_JWT_SECRET` 为高强度随机字符串（64 字符以上）
- [ ] 修改 `POSTGRES_PASSWORD` 和 `OPSMIND_DATABASE_PASSWORD`
- [ ] 修改 `MINIO_ROOT_PASSWORD` 和 `OPSMIND_MINIO_SECRET_KEY`
- [ ] 设置 `OPSMIND_SERVER_MODE=release`
- [ ] 配置 `OPSMIND_CORS_ALLOW_ORIGINS` 为生产域名
- [ ] 使用 nginx / Caddy 作为反向代理，配置 HTTPS
- [ ] 配置 PostgreSQL 和 MinIO 的数据持久化卷
- [ ] 设置防火墙规则，仅暴露 80/443 端口
- [ ] 配置数据库定期备份
- [ ] 修改默认管理员密码

### 推荐的反向代理配置（nginx）

```nginx
server {
    listen 443 ssl http2;
    server_name opsmind.example.com;

    ssl_certificate     /etc/ssl/opmind.pem;
    ssl_certificate_key /etc/ssl/opmind.key;

    # 前端
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 后端 API（含 SSE 流式，需关闭缓冲）
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 支持：关闭代理缓冲
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 300s;
        chunked_transfer_encoding on;
    }
}
```

> **重要：** `/api/` 路径必须关闭代理缓冲（`proxy_buffering off`），否则 SSE 流式输出会被缓冲而非实时推送到客户端。

### 数据库备份

```bash
# PostgreSQL 备份
docker compose exec postgres pg_dump -U opsmind opsmind > backup_$(date +%Y%m%d).sql

# MinIO 数据备份（直接备份数据卷）
docker compose stop minio
tar -czf minio_backup_$(date +%Y%m%d).tar.gz /var/lib/docker/volumes/opsmind_minio_data/
docker compose start minio
```

### 资源建议

| 规模 | CPU | 内存 | 磁盘 | 说明 |
|------|-----|------|------|------|
| 最小（API 模式） | 2 核 | 4 GB | 20 GB | 不含本地 LLM |
| 标准（本地 LLM） | 4 核 | 16 GB | 50 GB | 含 Qwen3-4B 对话 + 0.6B embedding |
| 生产（本地 LLM） | 8 核 | 32 GB | 100 GB+ | 含更大模型 + 数据增长 |

## Makefile 命令

```bash
make dev       # 启动开发依赖（postgres + minio）
make build     # 构建全部 Docker 镜像
make test      # 运行全部测试
make migrate   # 运行数据库迁移
make seed      # 加载演示数据
```
