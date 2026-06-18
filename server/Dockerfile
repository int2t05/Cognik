# OpsMind Server — Go 后端多阶段构建
#
# 阶段 1: 编译 Go 二进制
# 阶段 2: Alpine 运行时（纯 Go，无 Python 重排序依赖）
#
# 重排序说明：cross-encoder (bge-reranker-base) 约 1.1GB，
# 通过 Python 子进程推理。Docker 镜像不内置此依赖，
# 运行时若 reranker 不可用，RAG 管道自动降级跳过重排序步骤。
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

# ===== 阶段 2: Alpine 运行时（纯 Go 二进制）=====
FROM alpine:3.21

# 运行时依赖：CA 证书（HTTPS 调用 LLM API）+ 时区数据
RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制 Go 二进制
COPY --from=builder /build/opsmind-server .

# 暴露 8080 端口
EXPOSE 8080

# 运行服务
ENTRYPOINT ["./opsmind-server"]
