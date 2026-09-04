# Cognos — 本地开发 Makefile
# 用法：make help

.PHONY: help dev dev-server dev-web dev-db dev-ai dev-stop dev-clean build server-build web-build rerank-model

# 默认配置
GO_CMD    := go
NPM_CMD   := npm
DC_CMD    := docker compose -f deploy/docker-compose.yml

help: ## 显示所有命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ============================================
# 一键启动
# ============================================

dev: dev-db ## 一键启动开发环境（DB + 后端 + 前端）
	@echo "==> 启动后端（热重载）..."
	cd server && $(GO_CMD) run ./cmd/main.go &
	@echo "==> 启动前端（热重载）..."
	cd web && $(NPM_CMD) run dev &
	@wait

dev-detached: dev-db ## 后台启动后端 + 前端
	@echo "==> 启动后端（后台）..."
	cd server && $(GO_CMD) run ./cmd/main.go > /dev/null 2>&1 &
	@echo "==> 启动前端（后台）..."
	cd web && $(NPM_CMD) run dev > /dev/null 2>&1 &
	@echo "==> 后端 http://localhost:8080  前端 http://localhost:3000"

dev-server: dev-db ## 仅启动后端
	cd server && $(GO_CMD) run ./cmd/main.go

dev-web: ## 仅启动前端
	cd web && $(NPM_CMD) run dev

# ============================================
# Docker 基础设施
# ============================================

dev-db: ## 启动 PostgreSQL + pgvector
	$(DC_CMD) up -d postgres
	@echo "==> PostgreSQL 就绪 localhost:5432"

dev-ai: dev-db ## 启动 PostgreSQL + llama.cpp（本地 LLM + Embedding）
	$(DC_CMD) --profile ai-local up -d
	@echo "==> LLM localhost:8081  Embedding localhost:8082"

dev-storage: dev-db ## 启动 PostgreSQL + MinIO（可选存储后端）
	$(DC_CMD) --profile storage up -d
	@echo "==> MinIO localhost:9000 (console :9001)"

dev-stop: ## 停止所有 Docker 服务
	$(DC_CMD) down

dev-clean: dev-stop ## 停止并清除所有数据（谨慎！）
	$(DC_CMD) down -v
	@echo "==> 数据已清除"

# ============================================
# 构建
# ============================================

build: server-build web-build ## 编译后端 + 前端

server-build: ## 编译 Go 后端
	cd server && $(GO_CMD) build -o bin/cognos-server ./cmd/main.go

web-build: ## 构建 Next.js 前端
	cd web && $(NPM_CMD) run build

# ============================================
# 依赖安装
# ============================================

install: ## 安装后端 + 前端依赖
	cd server && $(GO_CMD) mod download
	cd web && $(NPM_CMD) ci

rerank-model: ## 下载重排序模型（仅需一次）
	cd server && python3 models/rerank/download.py

# ============================================
# 测试
# ============================================

test: ## 运行后端测试（需要 PostgreSQL + pgvector）
	cd server && $(GO_CMD) test ./test/... -v -tags=integration -p 1

test-unit: ## 运行前端单元测试
	cd web && $(NPM_CMD) test

lint: ## 代码检查
	cd server && $(GO_CMD) vet ./...
	cd web && $(NPM_CMD) run lint

# ============================================
# All-in-One Docker 镜像
# ============================================

docker-allinone: ## 构建 All-in-One Docker 镜像（生产部署）
	docker build -f deploy/allinone/Dockerfile -t cognos-allinone .

docker-server: ## 构建 Go 后端 Docker 镜像
	docker build -f deploy/docker/server/Dockerfile -t cognos-server server/

docker-web: ## 构建 Next.js 前端 Docker 镜像
	docker build -f deploy/docker/web/Dockerfile -t cognos-web web/
