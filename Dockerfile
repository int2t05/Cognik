# OpsMind All-in-One 镜像
# 合并 4 个核心服务：PostgreSQL+pgvector、MinIO、Go 后端、Next.js 前端
#
# 构建前准备（仅需一次）：
#   cd server && python models/rerank/download.py && cd ..
#
# 构建：
#   docker build -t opsmind-allinone .
#
# 运行：
#   docker run -d --name opsmind \
#     -p 8080:8080 -p 3000:3000 \
#     -v opsmind-pg:/var/lib/postgresql \
#     -v opsmind-minio:/data/minio \
#     opsmind-allinone
#
# 首次启动自动执行 initdb + 种子数据，后续启动使用已有数据。

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
# 阶段 3：运行时组装
# ===================================================================
FROM pgvector/pgvector:pg18

LABEL org.opencontainers.image.source="https://github.com/int2t05/OpsMind"
LABEL org.opencontainers.image.description="OpsMind All-in-One"

# ---- 系统依赖 ----
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 python3-pip supervisor curl ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

# ---- Python 依赖 ----
RUN pip3 install --upgrade pip --break-system-packages \
    && pip3 install --no-cache-dir --trusted-host download.pytorch.org \
       torch --index-url https://download.pytorch.org/whl/cpu --break-system-packages \
    && pip3 install --no-cache-dir sentence-transformers --break-system-packages

# ---- MinIO 二进制 ----
RUN curl -fsSL https://dl.min.io/server/minio/release/linux-amd64/minio \
    -o /usr/local/bin/minio && chmod +x /usr/local/bin/minio

# ---- 应用目录 ----
# /data 为 Railway Volume 挂载点，PG 和 MinIO 共用
RUN mkdir -p /app/web /app/migrations /app/models/rerank \
    /var/log/supervisor /data/minio /data/postgresql

WORKDIR /app

# ---- 复制 Go 后端产物 ----
COPY --from=go-builder /build/opsmind-server .
COPY --from=go-builder /build/rerank_server.py .
COPY --from=go-builder /build/migrations/init.sql /app/migrations/
COPY --from=go-builder /build/migrations/seed_essential.sql /app/migrations/

# ---- 复制重排序模型 ----
COPY server/models/rerank/ /app/models/rerank/

# ---- gse 中文词典（BM25 分词运行时必需） ----
COPY --from=go-builder /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/ \
    /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/

# ---- 验证重排序模型 ----
ENV RERANK_MODEL=/app/models/rerank
RUN if [ ! -f /app/models/rerank/config.json ]; then \
    echo "=============================================="; \
    echo "  错误: 重排序模型文件未下载"; \
    echo "  请先执行: cd server && python models/rerank/download.py"; \
    echo "=============================================="; \
    exit 1; \
    fi \
    && python3 -c "\
from sentence_transformers import CrossEncoder; \
CrossEncoder('/app/models/rerank', device='cpu'); \
print('Cross-encoder OK')"

# ---- 复制 Next.js standalone ----
COPY --from=web-builder /app/.next/standalone/ /app/web/
COPY --from=web-builder /app/.next/static/ /app/web/.next/static/
COPY --from=web-builder /app/public/ /app/web/public/

# ===================================================================
# 配置文件（全部用 printf 写入，兼容所有 Docker 版本）
# ===================================================================

# ---- supervisord.conf ----
RUN printf '%s\n' \
    '[supervisord]' \
    'nodaemon=true' \
    'user=root' \
    'logfile=/var/log/supervisor/supervisord.log' \
    'logfile_maxbytes=50MB' \
    'logfile_backups=2' \
    'loglevel=info' \
    'pidfile=/var/run/supervisord.pid' \
    '' \
    '[unix_http_server]' \
    'file=/var/run/supervisor.sock' \
    '' \
    '[supervisorctl]' \
    'serverurl=unix:///var/run/supervisor.sock' \
    '' \
    '[rpcinterface:supervisor]' \
    'supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface' \
    '' \
    '[program:postgres]' \
    'command=/usr/lib/postgresql/18/bin/postgres -D %(ENV_PGDATA)s' \
    'user=postgres' \
    'autostart=true' \
    'autorestart=true' \
    'priority=1' \
    'startsecs=3' \
    'startretries=5' \
    'stopsignal=SIGTERM' \
    'stopwaitsecs=60' \
    'stdout_logfile=/var/log/supervisor/postgres.log' \
    'stderr_logfile=/var/log/supervisor/postgres_err.log' \
    'environment=PGDATA="%(ENV_PGDATA)s"' \
    '' \
    '[program:minio]' \
    'command=/usr/local/bin/minio server /data/minio --console-address ":9001"' \
    'user=root' \
    'autostart=true' \
    'autorestart=true' \
    'priority=2' \
    'startsecs=3' \
    'startretries=5' \
    'stopsignal=SIGTERM' \
    'stopwaitsecs=30' \
    'stdout_logfile=/var/log/supervisor/minio.log' \
    'stderr_logfile=/var/log/supervisor/minio_err.log' \
    '' \
    '[program:opsmind-server]' \
    'command=/app/wait-for-pg.sh /app/opsmind-server' \
    'user=root' \
    'autostart=true' \
    'autorestart=unexpected' \
    'priority=3' \
    'startsecs=5' \
    'startretries=5' \
    'stopsignal=SIGTERM' \
    'stopwaitsecs=30' \
    'stdout_logfile=/var/log/supervisor/opsmind-server.log' \
    'stderr_logfile=/var/log/supervisor/opsmind-server_err.log' \
    '' \
    '[program:opsmind-web]' \
    'command=/app/wait-for-server.sh node /app/web/server.js' \
    'user=root' \
    'autostart=true' \
    'autorestart=unexpected' \
    'priority=4' \
    'startsecs=3' \
    'startretries=5' \
    'stopsignal=SIGTERM' \
    'stopwaitsecs=10' \
    'stdout_logfile=/var/log/supervisor/opsmind-web.log' \
    'stderr_logfile=/var/log/supervisor/opsmind-web_err.log' \
    > /etc/supervisor/supervisord.conf

# ---- wait-for-pg.sh ----
RUN printf '%s\n' \
    '#!/bin/bash' \
    'set -e' \
    'TIMEOUT=${1:-60}' \
    'shift' \
    'for i in $(seq 1 "$TIMEOUT"); do' \
    '  if /usr/lib/postgresql/18/bin/pg_isready -h localhost -q 2>/dev/null; then' \
    '    exec "$@"' \
    '  fi' \
    '  sleep 1' \
    'done' \
    'echo "[wait-for-pg] Timed out after ${TIMEOUT}s"' \
    'exit 1' \
    > /app/wait-for-pg.sh && chmod +x /app/wait-for-pg.sh

# ---- wait-for-server.sh ----
RUN printf '%s\n' \
    '#!/bin/bash' \
    'set -e' \
    'TIMEOUT=${1:-120}' \
    'shift' \
    'for i in $(seq 1 "$TIMEOUT"); do' \
    '  if curl -sf http://localhost:8080/health >/dev/null 2>&1; then' \
    '    exec "$@"' \
    '  fi' \
    '  sleep 1' \
    'done' \
    'echo "[wait-for-server] Timed out after ${TIMEOUT}s"' \
    'exit 1' \
    > /app/wait-for-server.sh && chmod +x /app/wait-for-server.sh

# ---- entrypoint.sh ----
RUN printf '%s\n' \
    '#!/bin/bash' \
    'set -e' \
    '' \
    'PGDATA="${PGDATA:-/data/postgresql/pgdata}"' \
    'PG_BIN="/usr/lib/postgresql/18/bin"' \
    'PG_USER="${POSTGRES_USER:-opsmind}"' \
    'PG_PASS="${POSTGRES_PASSWORD:-opsmind_dev}"' \
    'PG_DB="${POSTGRES_DB:-opsmind}"' \
    '' \
    'log() { echo "[entrypoint] $(date +%H:%M:%S) $*"; }' \
    '' \
    '# ---- init PGDATA ----' \
    'init_pgdata() {' \
    '  if [ -f "${PGDATA}/PG_VERSION" ]; then' \
    '    log "PGDATA already initialized, skip initdb"' \
    '    return 0' \
    '  fi' \
    '  log "First run: initdb..."' \
    '  mkdir -p "$(dirname "$PGDATA")"' \
    '  chown -R postgres:postgres "$(dirname "$PGDATA")"' \
    '  su - postgres -c "${PG_BIN}/initdb -D ${PGDATA} --auth-host=scram-sha-256 --auth-local=peer --encoding=UTF-8 --locale=C"' \
    '  cat >> "${PGDATA}/pg_hba.conf" <<HBA' \
    'host all all 127.0.0.1/32 scram-sha-256' \
    'host all all ::1/128 scram-sha-256' \
    'HBA' \
    '  log "initdb done"' \
    '}' \
    '' \
    '# ---- init database ----' \
    'init_database() {' \
    '  log "Starting temp PostgreSQL..."' \
    '  su - postgres -c "${PG_BIN}/pg_ctl -D ${PGDATA} -l /tmp/pg-init.log start"' \
    '  for i in $(seq 1 30); do' \
    '    su - postgres -c "${PG_BIN}/pg_isready -q" 2>/dev/null && break' \
    '    sleep 1' \
    '  done' \
    '  echo "ALTER USER postgres PASSWORD '"'"'${PG_PASS}'"'"';" | su - postgres -c "${PG_BIN}/psql"' \
    '  su - postgres -c "${PG_BIN}/psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='"'"'${PG_USER}'"'"';\"" | grep -q 1 || echo "CREATE USER ${PG_USER} WITH PASSWORD '"'"'${PG_PASS}'"'"' SUPERUSER;" | su - postgres -c "${PG_BIN}/psql"' \
    '  su - postgres -c "${PG_BIN}/psql -tAc \"SELECT 1 FROM pg_database WHERE datname='"'"'${PG_DB}'"'"';\"" | grep -q 1 || su - postgres -c "${PG_BIN}/psql -c \"CREATE DATABASE ${PG_DB} OWNER ${PG_USER};\""' \
    '  log "Database ready (seed data loads via AutoSeed on server start)"' \
    '  su - postgres -c "${PG_BIN}/pg_ctl -D ${PGDATA} stop"' \
    '}' \
    '' \
    '# ---- main ----' \
    'log "=== OpsMind All-in-One Starting ==="' \
    'init_pgdata' \
    'init_database' \
    'log "Launching supervisord..."' \
    'exec /usr/bin/supervisord -c /etc/supervisor/supervisord.conf -n' \
    > /app/entrypoint.sh && chmod +x /app/entrypoint.sh

# ===================================================================
# 运行时配置
# ===================================================================

ENV TZ=Asia/Shanghai

# 覆盖 pgvector 基础镜像的 PGDATA，指向 Railway Volume 挂载路径
ENV PGDATA=/data/postgresql/pgdata

ENV OPSMIND_DATABASE_HOST=localhost
ENV OPSMIND_MINIO_ENDPOINT=localhost:9000
ENV OPSMIND_RERANK_PYTHON_PATH=python3
ENV OPSMIND_RERANK_SCRIPT_PATH=/app/rerank_server.py
ENV OPSMIND_RERANK_ENABLED=true
ENV NEXT_PUBLIC_API_URL=http://localhost:8080
# PORT 由 Railway 自动注入，不硬编码
ENV NODE_ENV=production

# Railway 通过面板挂载卷，此处不声明 VOLUME
# Railway 自动检测 PORT 变量确定转发端口
EXPOSE 3000
ENTRYPOINT ["/app/entrypoint.sh"]
