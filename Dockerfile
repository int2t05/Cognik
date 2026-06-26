# OpsMind All-in-One 镜像
# 合并：PostgreSQL+pgvector、MinIO、Go 后端、Next.js 前端
#
# 构建：
#   cd server && python models/rerank/download.py   # 下载重排序模型（仅需一次）
#   docker build -t opsmind-allinone .
#
# 运行：
#   docker run -d --name opsmind \
#     -p 3000:3000 -e JWT_SECRET=xxx \
#     -v opsmind-data:/data \
#     opsmind-allinone
#
# 首次启动自动 initdb + 建库，种子数据由 Go AutoSeed 加载。

# ===================================================================
# 阶段 1：编译 Go 后端
# ===================================================================
FROM golang:1.26-alpine AS go-builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY server/go.mod server/go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /build/opsmind-server ./cmd/main.go

# ===================================================================
# 阶段 2：构建 Next.js 前端
# ===================================================================
FROM node:22-alpine AS web-builder
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ===================================================================
# 阶段 3：运行时
# ===================================================================
FROM pgvector/pgvector:pg18

# 系统依赖（含 Node.js 22 用于运行 Next.js standalone）
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates gnupg tzdata \
    python3 python3-pip supervisor \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" > /etc/apt/sources.list.d/nodesource.list \
    && apt-get update && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Python 依赖（torch CPU + sentence-transformers）
RUN pip3 install --upgrade pip --break-system-packages \
    && pip3 install --no-cache-dir --trusted-host download.pytorch.org \
       torch --index-url https://download.pytorch.org/whl/cpu --break-system-packages \
    && pip3 install --no-cache-dir sentence-transformers --break-system-packages

# MinIO 二进制
RUN curl -fsSL https://dl.min.io/server/minio/release/linux-amd64/minio \
    -o /usr/local/bin/minio && chmod +x /usr/local/bin/minio

# 目录结构
RUN mkdir -p /app/web /app/migrations /app/models/rerank \
    /var/log/supervisor /data/minio /data/postgresql
WORKDIR /app

# ---- 复制产物 ----
COPY --from=go-builder /build/opsmind-server .
COPY --from=go-builder /build/rerank_server.py .
COPY --from=go-builder /build/migrations/ /app/migrations/
COPY server/models/rerank/ /app/models/rerank/

# gse 中文词典（BM25 分词运行时必需）
COPY --from=go-builder /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/ \
    /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/

# 重排序模型验证（缺失则构建失败）
RUN if [ ! -f /app/models/rerank/config.json ]; then \
    echo "错误: 重排序模型未下载，请先执行: cd server && python models/rerank/download.py"; exit 1; fi \
    && python3 -c "from sentence_transformers import CrossEncoder; CrossEncoder('/app/models/rerank', device='cpu'); print('Cross-encoder OK')"

# Next.js standalone
COPY --from=web-builder /app/.next/standalone/ /app/web/
COPY --from=web-builder /app/.next/static/ /app/web/.next/static/
COPY --from=web-builder /app/public/ /app/web/public/

# ---- 配置与脚本 ----
COPY docker/allinone/supervisord.conf /etc/supervisor/supervisord.conf
COPY docker/allinone/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# ---- 运行时 ----
ENV TZ=Asia/Shanghai
ENV PGDATA=/data/postgresql/pgdata
ENV OPSMIND_DATABASE_HOST=localhost
ENV OPSMIND_DATABASE_PASSWORD=opsmind_dev
ENV OPSMIND_MINIO_ENDPOINT=localhost:9000
ENV OPSMIND_RERANK_PYTHON_PATH=python3
ENV OPSMIND_RERANK_SCRIPT_PATH=/app/rerank_server.py
ENV OPSMIND_RERANK_ENABLED=true
ENV NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NODE_ENV=production

EXPOSE 3000
ENTRYPOINT ["/app/entrypoint.sh"]
