# OpsMind Server — Go 后端多阶段构建
#
# 阶段 1: 编译 Go 二进制
# 阶段 2: Python slim 运行时（Go 二进制 + 轻量 cross-encoder 重排序）
#
# 重排序模型：ms-marco-MiniLM-L-4-v2（~50MB，vs bge-reranker-base 1.1GB）
# 镜像总大小约 450MB（vs 旧方案 2.5GB）
#
# 构建方式：
#   docker build -t opsmind-server -f server/Dockerfile server/
# 或在项目根目录通过 docker compose 一键构建：
#   docker compose build opsmind-server

# ===== 阶段 1: 编译 Go =====
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# 利用 Docker 层缓存：先复制依赖文件
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

# 复制源码并编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /build/opsmind-server ./cmd/main.go

# ===== 阶段 2: Python slim + 轻量 reranker =====
FROM python:3.11-slim-bookworm

# 系统依赖：CA 证书 + 时区
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

ENV TZ=Asia/Shanghai

WORKDIR /app

# Python 依赖：torch 2.6+ 默认 PyPI 即 CPU-only（~200MB）+ sentence-transformers
# 不再使用 PyTorch 专用 CPU 索引（hash 不稳定导致构建中断）
RUN pip install --no-cache-dir --retries 10 \
    torch sentence-transformers

# 预下载 rerank 模型到镜像（编译期完成，启动时无需网络）
# ms-marco-MiniLM-L-4-v2：4 层 transformer ~50MB，最小可用的 cross-encoder
# 环境变量 RERANK_MODEL 可在运行时覆盖为其他模型（如 bge-reranker-v2-m3）
ENV RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-4-v2
RUN python -c "\
from sentence_transformers import CrossEncoder; \
import torch; \
CrossEncoder('cross-encoder/ms-marco-MiniLM-L-4-v2', device='cpu')"

# 从构建阶段复制 Go 二进制
COPY --from=builder /build/opsmind-server .

# 复制 Python 重排序推理脚本
COPY rerank_server.py .

EXPOSE 8080

ENTRYPOINT ["./opsmind-server"]
