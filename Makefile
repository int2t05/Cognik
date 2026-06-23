# OpsMind Makefile — 构建和开发命令
#
# 使用方式：
#   make dev          本地开发启动（仅启动依赖服务）
#   make build        构建全部 Docker 镜像
#   make up           一键启动全部服务（含健康检查等待）
#   make up-ai        启动含 llama.cpp 的完整 AI 环境
#   make down         停止全部服务
#   make down-v       停止并清除数据卷
#   make restart      重启全部服务
#   make logs         查看全部服务日志
#   make status       查看服务运行状态
#   make ps           查看服务运行状态（别名）
#   make test         运行全部非集成测试
#   make test-integration 运行集成测试（需 PostgreSQL opsmind_test 库）
#   make db-init      执行 DDL 增强脚本（HNSW 索引、列注释）
#   make db-seed      加载最小测试数据（角色 + 用户）
#   make shell-db     进入 PostgreSQL 交互终端
#   make model-download 下载 llama.cpp GGUF 模型文件
#   make clean        清理构建产物和运行时数据

.PHONY: help dev build up up-ai down down-v restart logs status ps test test-integration db-init db-seed shell-db model-download clean

# 默认目标
help:
	@echo "OpsMind 构建和开发命令"
	@echo ""
	@echo "  启动与部署："
	@echo "    make dev             本地开发启动（仅启动依赖服务）"
	@echo "    make build           构建全部 Docker 镜像"
	@echo "    make up              一键启动全部服务（含健康检查等待）"
	@echo "    make up-ai           启动含 llama.cpp 的完整 AI 环境"
	@echo "    make down            停止全部服务"
	@echo "    make down-v          停止并清除数据卷"
	@echo "    make restart         重启全部服务"
	@echo ""
	@echo "  运维与监控："
	@echo "    make logs            查看全部服务日志（Ctrl+C 退出）"
	@echo "    make status          查看服务运行状态"
	@echo "    make ps              查看服务运行状态（别名）"
	@echo "    make shell-db        进入 PostgreSQL 交互终端"
	@echo ""
	@echo "  数据库："
	@echo "    make db-init          执行 DDL 增强脚本"
	@echo "    make db-seed          加载角色和用户"
	@echo ""
	@echo "  测试："
	@echo "    make test            运行全部非集成测试"
	@echo "    make test-integration 运行集成测试（需 docker compose up -d postgres）"
	@echo ""
	@echo "  AI 模型："
	@echo "    make model-download  下载 llama.cpp GGUF 模型文件"
	@echo ""
	@echo "  其他："
	@echo "    make clean           清理构建产物和运行时数据"

# ===== 本地开发 =====

# 启动本地开发所需的依赖服务（PostgreSQL + MinIO）
dev:
	@echo "=== 启动依赖服务 ==="
	docker compose up -d postgres minio
	@echo ""
	@echo "等待 PostgreSQL 就绪..."
	@timeout=30; while [ $$timeout -gt 0 ]; do \
		if docker compose exec -T postgres pg_isready -U opsmind >/dev/null 2>&1; then \
			echo "PostgreSQL 已就绪 ✓"; \
			break; \
		fi; \
		sleep 2; \
		timeout=$$((timeout - 2)); \
	done; \
	if [ $$timeout -le 0 ]; then echo "PostgreSQL 启动超时！"; exit 1; fi
	@echo "MinIO 已启动（API: :9000, Web: :9001）"
	@echo ""
	@echo "依赖服务已就绪，接下来手动启动："
	@echo "  cd server && go run ./cmd/main.go"
	@echo "  cd web && npm run dev"

# ===== Docker 构建和启动 =====

# 构建全部镜像
build:
	docker compose build

# 一键启动全部服务（4 个必须服务，含健康检查等待）
up:
	docker compose up -d --build --wait
	@echo ""
	@echo "============================================"
	@echo "  OpsMind 服务已启动（全部 healthy）"
	@echo "============================================"
	@echo "  前端:     http://localhost:3000"
	@echo "  后端 API: http://localhost:8080"
	@echo "  数据库:   postgres:5432"
	@echo "  MinIO:    http://localhost:9001 (Console)"
	@echo ""
	@echo "  种子数据已自动加载（首次启动）"
	@echo "  如需启动 llama.cpp: make up-ai"
	@echo "============================================"

# 启动含 llama.cpp 的完整 AI 环境（需要先下载模型: make model-download）
up-ai:
	docker compose --profile ai-local up -d --build --wait
	@echo ""
	@echo "============================================"
	@echo "  OpsMind + llama.cpp 已启动（全部 healthy）"
	@echo "============================================"
	@echo "  前端:       http://localhost:3000"
	@echo "  后端 API:   http://localhost:8080"
	@echo "  LLM API:    http://llama-cpp:8081/v1 (localhost:8081/v1)"
	@echo "  Embedding:  http://llama-cpp-emb:8082/v1 (localhost:8082/v1)"
	@echo "============================================"

# 停止全部服务
down:
	docker compose $(if $(filter ai-local,$(PROFILE)),--profile ai-local,) down

# 停止并清除数据卷
down-v:
	docker compose down -v

# 重启全部服务
restart:
	docker compose restart
	@echo "服务已重启"

# ===== 运维与监控 =====

# 查看全部服务日志（Ctrl+C 退出）
logs:
	docker compose logs -f

# 查看服务运行状态
status:
	docker compose ps

# 查看服务运行状态（别名）
ps: status

# 进入 PostgreSQL 交互终端
shell-db:
	docker compose exec postgres psql -U opsmind -d opsmind

# ===== 测试 =====

# 运行全部非集成测试
test:
	cd server && go test ./tests/pkg/... ./tests/middleware/... ./tests/router/... ./tests/config/... ./tests/adapter/... -v

# 运行集成测试（需要 PostgreSQL opsmind_test 库；-p 1 避免跨包并行共享数据库冲突）
test-integration:
	cd server && go test ./tests/... -tags=integration -v -p 1

# ===== 数据库 =====

# 执行 DDL 增强脚本（pgvector 扩展 + HNSW 索引 + 列注释）
db-init:
	docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/init.sql

# 加载必要种子数据（角色 + 用户 + 菜单 + LLM 配置 + 系统配置）
db-seed:
	docker compose exec -T postgres psql -U opsmind -d opsmind < server/migrations/seed_essential.sql

# ===== 模型下载 =====

# 下载 llama.cpp GGUF 模型文件（对话模型 + Embedding 模型）
#
# llama.cpp 需要 .gguf 格式的模型文件，而非原始 HuggingFace 模型。
# 最便捷的下载方式是通过 huggingface-cli（需 Python 3.8+）。
#
# 没有 huggingface-cli？手动从 HuggingFace 下载 .gguf 文件到 ./models/ 即可。
model-download:
	@echo "=== 下载 llama.cpp GGUF 模型文件 ==="
	@echo ""
	@mkdir -p models
	@if command -v huggingface-cli >/dev/null 2>&1; then \
		echo "使用 huggingface-cli 下载..."; \
		echo ""; \
		echo "下载对话模型 Qwen3-4B-Instruct Q4_K_M (~2.5 GB)..."; \
		huggingface-cli download bartowski/Qwen3-4B-Instruct-2507-GGUF \
			--include "*Q4_K_M*" --local-dir ./models/ || \
			echo "下载失败，请手动下载 .gguf 文件放到 ./models/"; \
		echo ""; \
		echo "下载 Embedding 模型 Qwen3-Embedding-0.6B Q8_0 (~0.6 GB)..."; \
		huggingface-cli download bartowski/Qwen3-Embedding-0.6B-GGUF \
			--include "*Q8_0*" --local-dir ./models/ || \
			echo "下载失败，请手动下载 .gguf 文件放到 ./models/"; \
	else \
		echo "未安装 huggingface-cli。"; \
		echo ""; \
		echo "安装方式："; \
		echo "  pip install huggingface_hub"; \
		echo ""; \
		echo "然后重新运行: make model-download"; \
	fi
	@echo ""
	@echo "模型文件位于 ./models/ 目录"
	@echo "现在可以运行: make up-ai"

# ===== 清理 =====

# 清理构建产物和运行时数据
clean:
	docker compose down -v
	docker builder prune -f
	rm -rf server/bin/
	rm -rf server/*.exe
	rm -rf web/dist/
	rm -rf web/node_modules/.vite/
	@echo "清理完成"
