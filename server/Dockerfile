# OpsMind Server — Go 后端多阶段构建
#
# 阶段 1: 编译 Go 二进制
# 阶段 2: Python slim 运行时（Go 二进制 + 本地 cross-encoder 模型）
#
# 重排序模型使用 MiniLM-L-4-v2（~80MB），
# 需提前下载到 server/models/rerank/ 目录。
# 详见 README.md「重排序模型」章节。
#
# 构建：
#   # 1. 下载模型（仅需一次）
#   cd server && python models/rerank/download.py
#   # 2. 构建镜像
#   docker compose build opsmind-server

# ===== 阶段 1: 编译 Go =====
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /build/opsmind-server ./cmd/main.go

# ===== 阶段 2: Python slim + 本地 reranker =====
FROM python:3.11-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

ENV TZ=Asia/Shanghai

WORKDIR /app

# Python 依赖
# 1. 升级 pip（24.0 的 hash 校验过于严格）
# 2. torch CPU 版从专用源（PyPI 默认是 CUDA 版，3GB+）
# 3. sentence-transformers 从默认 PyPI（torch 已安装，跳过）
RUN pip install --upgrade pip \
    && pip install --no-cache-dir --trusted-host download.pytorch.org \
       torch --index-url https://download.pytorch.org/whl/cpu \
    && pip install --no-cache-dir sentence-transformers

# 复制预下载的模型文件（构建时零网络依赖）
COPY models/rerank/ /app/models/rerank/

# 验证模型可用性。如未下载模型文件，构建会在此失败——
# 请先执行: cd server && python models/rerank/download.py
ENV RERANK_MODEL=/app/models/rerank
RUN if [ ! -f /app/models/rerank/config.json ]; then \
    echo ""; \
    echo "=============================================="; \
    echo "  错误: 重排序模型文件未下载"; \
    echo "  请先执行: cd server && python models/rerank/download.py"; \
    echo "  或设 OPSMIND_RERANK_ENABLED=false 跳过重排序"; \
    echo "=============================================="; \
    echo ""; \
    exit 1; \
    fi \
    && python -c "\
from sentence_transformers import CrossEncoder; \
CrossEncoder('/app/models/rerank', device='cpu'); \
print('模型加载成功')"

# 复制 gse 中文词典（运行时需要，BM25 分词必需）。
# gse 用 runtime.Caller 定位自身源码路径，编译时路径指向 Go module cache，
# 因此运行时必须在同路径存在词典文件。
COPY --from=builder /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/ /go/pkg/mod/github.com/go-ego/gse@v1.0.2/data/

# 复制 Go 二进制和推理脚本
COPY --from=builder /build/opsmind-server .
COPY rerank_server.py .

EXPOSE 8080

ENTRYPOINT ["./opsmind-server"]
