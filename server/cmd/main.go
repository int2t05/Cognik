// Package main 是 OpsMind 后端服务的入口。
//
// 负责初始化配置、数据库连接、路由注册和 HTTP 服务启动。
// 采用单体分层架构（Handler→Service→Repository），所有模块在同一进程内运行。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"opsmind/internal/adapter"
	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	opslog "opsmind/internal/log"
	"opsmind/internal/rag"
	"opsmind/internal/repository"
	"opsmind/internal/router"
	"opsmind/internal/service"
	"strconv"
)

// envInt 读取整数环境变量，失败或未设置时返回默认值。
func envInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return def
}

// envDuration 读取 time.Duration 环境变量（分钟），未设置时返回默认值。
func envDuration(key string, def time.Duration) time.Duration {
	if s := os.Getenv(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return time.Duration(v) * time.Minute
		}
	}
	return def
}

func main() {
	slog.Info("OpsMind 服务启动中...")

	// 1. 加载配置
	// TODO(cmd/main): 将 main 中的初始化流程拆成 wireApp()/runServer() 两层。
	// 现在 main 同时负责配置、DB、Adapter、Service、Handler、Scheduler 和 HTTP 生命周期，
	// 后续新增依赖时会继续膨胀，不利于集成测试单独复用应用装配逻辑。
	cfg, err := config.Load("")
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	// 初始化日志：JSON 格式输出到 stdout + 旋转日志文件。
	// OPSMIND_LOG_DIR 控制日志目录（见 docker-compose.yml + .env.example）。
	logDir := os.Getenv("OPSMIND_LOG_DIR")
	if logDir == "" {
		logDir = filepath.Join("..", "logs") // 本地开发 → 项目根目录 logs/
	}
	logCleanup, err := opslog.Init(logDir)
	if err != nil {
		slog.Warn("日志文件输出不可用，仅输出到控制台", "dir", logDir, "error", err)
	} else {
		defer logCleanup()
	}

	// 生产模式下 JWT 密钥必须非空，否则拒绝启动。
	if cfg.JWT.Secret == "" {
		if cfg.Server.Mode == "release" {
			slog.Error("JWT 密钥为空，生产模式不允许启动，请设置环境变量 OPSMIND_JWT_SECRET")
			os.Exit(1)
		}
		slog.Warn("JWT 密钥为空，JWT 认证功能不可用（仅调试模式允许）")
	}

	// 2. 初始化数据库连接
	// TODO(cmd/main): 数据库连接池参数目前写死在 database.Init 内部。
	// 应将 MaxOpenConns/MaxIdleConns/ConnMaxLifetime 放入配置，避免生产环境只能改代码调参。
	db, err := database.Init(cfg.Database)
	if err != nil {
		slog.Error("数据库连接失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库连接成功")

	// 3. 自动迁移
	// TODO(cmd/main): 生产环境应改为显式迁移命令或启动前迁移任务。
	// AutoMigrate 适合开发环境，但在生产库上自动变更表结构缺少审批、回滚和审计。
	if err := database.AutoMigrate(db); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库迁移完成")

	// 4. 初始化 Adapter 层（LLMClient / EmbeddingClient / VectorStore）
	// TODO(cmd/main): LLM/Embedding 超时时间应来自配置，并区分 query rewrite、rerank、最终生成等场景。
	// 当前 60s/30s 写死会让短请求和长生成共享同一超时策略。
	llmTimeout := 60 * time.Second
	llmClient, err := adapter.NewOpenAIClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, llmTimeout)
	if err != nil {
		slog.Error("创建 LLM 客户端失败", "error", err)
		os.Exit(1)
	}

	// Embedding 优先使用独立 Base URL 和 API Key，空时回退到 LLM 配置
	embedBaseURL := cfg.Embedding.BaseURL
	if embedBaseURL == "" {
		embedBaseURL = cfg.LLM.BaseURL
	}
	embedAPIKey := cfg.Embedding.APIKey
	if embedAPIKey == "" {
		embedAPIKey = cfg.LLM.APIKey
	}
	embeddingClient := adapter.NewOpenAIEmbeddingClient(embedBaseURL, embedAPIKey, cfg.Embedding.Model, 30*time.Second)
	slog.Info("LLM/Embedding 客户端已初始化",
		"llm_base_url", cfg.LLM.BaseURL,
		"embedding_base_url", embedBaseURL,
		"llm_model", cfg.LLM.Model,
		"embedding_model", cfg.Embedding.Model)

	// Cross-encoder 重排序子进程（可选，配置禁用时跳过）
	var reranker adapter.Reranker
	if cfg.Rerank.Enabled && cfg.Rerank.PythonPath != "" && cfg.Rerank.ScriptPath != "" {
		reranker = adapter.NewSubprocessReranker(cfg.Rerank.PythonPath, cfg.Rerank.ScriptPath)
		if reranker != nil {
			defer func() {
				if r, ok := reranker.(*adapter.SubprocessReranker); ok {
					r.Close()
				}
			}()
			slog.Info("Cross-encoder 重排序已启用", "python", cfg.Rerank.PythonPath, "script", cfg.Rerank.ScriptPath)
		}
	} else {
		slog.Info("Cross-encoder 重排序已禁用，将降级跳过")
	}

	// pgvector 向量存储
	// TODO(cmd/main): VectorStore 初始化失败后仅 warn，但后续 KnowledgeService/ChatService 仍可启动。
	// 应提供健康状态并让依赖向量核心路径的接口返回明确 20002，而不是在 nil store 处退化为未知错误。
	vectorStore, err := adapter.NewPgvectorStore(db)
	if err != nil {
		slog.Warn("pgvector 连接失败，向量检索功能不可用", "error", err)
	} else {
		slog.Info("pgvector VectorStore 已连接")
	}

	// MinIO 对象存储
	// TODO(cmd/main): MinIO 初始化失败后 storageClient 为 nil，传给 KnowledgeService/Processor 可能导致 panic。
	// 应在初始化失败时将 storageClient 设为降级实现（如本地文件存储），而非 nil。
	var storageClient adapter.StorageClient
	minioEndpoint := cfg.MinIO.Endpoint
	if minioEndpoint == "" {
		slog.Warn("MinIO 未配置，文档上传将使用降级模式（纯文本）")
	} else {
		minioClient, err := minio.New(minioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
			Secure: cfg.MinIO.UseSSL,
		})
		if err != nil {
			slog.Error("MinIO 客户端创建失败，文档上传将降级", "error", err)
		} else if mc, err := adapter.NewMinIOClient(minioClient, "opsmind-attachments", "opsmind-documents"); err != nil {
			slog.Error("MinIO bucket 初始化失败，文档上传将降级", "error", err)
		} else {
			storageClient = mc
			slog.Info("MinIO 对象存储已连接", "endpoint", minioEndpoint)
		}
	}

	// 5. 初始化 Repository 层
	configRepo := repository.NewConfigRepo(db)
	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	ticketRepo := repository.NewTicketRepo(db)
	knowledgeRepo := repository.NewKnowledgeRepo(db)
	chatRepo := repository.NewChatRepo(db)
	messageRepo := repository.NewMessageRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	dashboardRepo := repository.NewDashboardRepo(db)

	// 6. 初始化 Service 层
	txManager := service.NewGormTxManager(db)
	menuRepo := repository.NewMenuRepo(db)
	authService := service.NewAuthService(userRepo, menuRepo, db, cfg.JWT)
	userService := service.NewUserService(userRepo, auditRepo, db)
	roleService := service.NewRoleService(roleRepo, menuRepo, auditRepo, db)
	messageService := service.NewMessageService(messageRepo)
	ticketService := service.NewTicketService(ticketRepo, txManager, messageService, nil)
	dashboardService := service.NewDashboardService(dashboardRepo)
	configService := service.NewConfigService(configRepo, auditRepo)

	// LLM 配置管理
	// TODO(cmd/main): 启动时应把 config.yaml/env 的 LLM 配置同步为默认 LLMConfig 或作为 fallback 注入 Manager。
	// 当前 ChatService 可能从 LLMConfigManager 读取到 nil，然后使用 model="default"，与 cfg.LLM.Model 不一致。
	llmConfigRepo := repository.NewLlmConfigRepo(db)
	llmConfigSvc, err := service.NewLLMConfigService(llmConfigRepo)
	if err != nil {
		slog.Error("创建 LLM 配置服务失败", "error", err)
		os.Exit(1)
	}
	slog.Info("LLM 配置服务已初始化")

	// RAG 引擎组件
	embedder := rag.NewEmbedder(embeddingClient, 20)
	docParser := rag.NewDocParser()
	chunker := rag.NewChunker(1000, 200)

	// 向量检索器（包装 Embedder + pgvector）
	vectorRetriever := rag.NewVectorRetriever(embedder, vectorStore)

	// BM25 混合检索器（中文分词 + 倒排索引，懒加载 + TTL）
	segmenter := rag.NewGseSegmenter()
	bm25TTL := envDuration("OPSMIND_AI_BM25_REBUILD_MINUTES", 30*time.Minute)
	bm25Retriever := rag.NewBM25Retriever(segmenter, bm25TTL)

	// RAG 管道（查询改写 → 多路检索 → 混合检索 → 重排序）
	pipeline := rag.NewPipeline(vectorRetriever, bm25Retriever, llmClient, embedder, reranker)

	// 文档异步处理器（goroutine pool：解析→分块→embedding→pgvector 写入）
	procWorkers := envInt("OPSMIND_AI_PROCESSOR_WORKERS", 2)
	processor := rag.NewProcessor(docParser, chunker, embedder, vectorStore, storageClient, procWorkers)

	// KnowledgeService（CRUD + pgvector 管道 + 文档上传）
	knowledgeService := service.NewKnowledgeService(knowledgeRepo,
		service.WithUserNames(userRepo),
		service.WithChunker(chunker),
		service.WithEmbedder(embedder),
		service.WithVectorStore(vectorStore),
		service.WithDocParser(docParser),
		service.WithProcessor(processor),
		service.WithStorage(storageClient),
		service.WithAuditRepo(auditRepo),
	)
	slog.Info("KnowledgeService 已初始化（含 Chunker + Processor）")
	ticketService.SetKnowledgeService(knowledgeService)

	// LLMService（RAG + prompt + LLM 统一编排，供 ChatService 使用）
	llmService := service.NewLLMService(llmClient, llmConfigSvc.GetManager(), cfg.LLM.Model, pipeline, cfg.AI.MaxHistoryMessages)
	slog.Info("LLMService 已初始化（RAG + prompt + LLM 统一调用）")

	// ChatService（会话生命周期管理）
	chatService := service.NewChatService(knowledgeRepo, chatRepo, llmService, service.RAGDefaults{
		TopK:         cfg.AI.DefaultTopK,
		QueryRewrite: cfg.AI.RAGQueryRewrite,
		MultiRoute:   cfg.AI.RAGMultiRoute,
		Hybrid:       cfg.AI.RAGHybrid,
		Rerank:       cfg.AI.RAGRerank,
	})
	slog.Info("ChatService 已初始化（含 RAG Pipeline + LLMService）")

	// AuditService
	auditService := service.NewAuditService(auditRepo)

	// 7. 初始化 Handler 层
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	roleHandler := handler.NewRoleHandler(roleService)
	ticketHandler := handler.NewTicketHandler(ticketService)
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeService)
	chatHandler := handler.NewChatHandler(chatService)
	messageHandler := handler.NewMessageHandler(messageService)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)
	auditHandler := handler.NewAuditHandler(auditService)
	configHandler := handler.NewConfigHandler(configService)
	llmConfigHandler := handler.NewLLMConfigHandler(llmConfigSvc)

	// 8. 初始化后台调度器
	scheduler := service.NewScheduler(ticketService)
	slog.Info("后台调度器已创建")

	// 9. 设置路由
	r := router.Setup(cfg, db, &router.Handlers{
		Auth:      authHandler,
		User:      userHandler,
		Role:      roleHandler,
		Ticket:    ticketHandler,
		Knowledge: knowledgeHandler,
		Chat:      chatHandler,
		Message:   messageHandler,
		Dashboard: dashboardHandler,
		Audit:     auditHandler,
		Config:    configHandler,
		LLMConfig: llmConfigHandler,
	})

	// 10. 创建 HTTP Server
	// TODO(cmd/main): ReadTimeout/WriteTimeout/IdleTimeout 应配置化，并为 SSE 单独提供更长写超时策略。
	// 全局 WriteTimeout=60s 对慢速 LLM 生成仍可能过短。
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // SSE 路由内部通过 SetWriteDeadline 续期
		IdleTimeout:  60 * time.Second,
	}

	// 11. 启动调度器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)

	// 12. 启动 HTTP 服务
	go func() {
		slog.Info("HTTP 服务已启动", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	// 13. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("收到退出信号，开始优雅关闭...", "signal", sig)

	scheduler.Stop()
	authService.Shutdown()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP 服务关闭失败", "error", err)
	} else {
		slog.Info("HTTP 服务已关闭")
	}

	slog.Info("OpsMind 服务已停止")
}
