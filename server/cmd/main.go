// Package main 是 OpsMind 后端服务的入口。
//
// main 负责流程编排：加载配置 → 装配依赖 → 运行服务。
// 初始化细节集中在 wireApp 中，生命周期管理集中在 runServer 中。
package main

import (
	"context"
	"encoding/json"
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
	"opsmind/internal/cache"
	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	opslog "opsmind/internal/log"
	"opsmind/internal/model"
	"opsmind/internal/rag"
	"opsmind/internal/repository"
	"opsmind/internal/router"
	"opsmind/internal/service"
)

// app 持有所有已初始化的组件。
// wireApp 负责装配，runServer 负责运行。
type app struct {
	cfg           *config.AppConfig
	logCleanup    func()
	llmClient     *adapter.OpenAIClient
	reranker      adapter.Reranker
	vectorStore   adapter.VectorStore
	storageClient adapter.StorageClient
	scheduler     *service.Scheduler
	authService   *service.AuthService
	server        *http.Server
}

func main() {
	slog.Info("OpsMind 服务启动中...")

	app, err := wireApp()
	if err != nil {
		slog.Error("装配应用失败", "error", err)
		os.Exit(1)
	}

	if err := app.run(); err != nil {
		slog.Error("服务运行失败", "error", err)
		os.Exit(1)
	}
}

// wireApp 加载配置、初始化所有组件并注入依赖。
//
// 为什么拆分为独立函数：
// main 原先同时负责配置加载、DB/Adapter/Service/Handler 初始化和 HTTP 生命周期，
// 400+ 行混合了装配逻辑和运行时逻辑，不利于集成测试复用装配流程。
//
// 注意：当前 wireApp 中无直接 repo 调用（所有 repo 访问都在 service 构造器内部）。
// 如果将来添加直接 repo 调用，应该在此函数顶部创建 ctx := context.Background() 并传入。
func wireApp() (*app, error) {
	a := &app{}

	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = cfg

	// 初始化日志
	logDir := os.Getenv("OPSMIND_LOG_DIR")
	if logDir == "" {
		logDir = filepath.Join("..", "logs")
	}
	if cleanup, err := opslog.Init(logDir); err != nil {
		slog.Warn("日志文件输出不可用，仅输出到控制台", "dir", logDir, "error", err)
	} else {
		a.logCleanup = cleanup
	}

	// 生产模式 JWT 密钥非空校验
	if cfg.JWT.Secret == "" {
		if cfg.Server.Mode == "release" {
			return nil, fmt.Errorf("JWT 密钥为空，生产模式不允许启动，请设置 OPSMIND_JWT_SECRET")
		}
		slog.Warn("JWT 密钥为空，JWT 认证功能不可用（仅调试模式允许）")
	}

	// 2. 数据库
	db, err := database.Init(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	slog.Info("数据库连接成功")

	// AutoMigrate（开发环境自动迁移，生产环境通过 OPSMIND_DB_SKIP_MIGRATE 跳过）
	if os.Getenv("OPSMIND_DB_SKIP_MIGRATE") == "true" {
		slog.Info("已跳过数据库自动迁移（OPSMIND_DB_SKIP_MIGRATE=true）")
	} else {
		if err := database.AutoMigrate(db); err != nil {
			return nil, fmt.Errorf("数据库迁移失败: %w", err)
		}
		slog.Info("数据库迁移完成")
	}

	// 3. Adapter 层
	llmTimeout := cfg.LLM.Timeout
	if llmTimeout <= 0 {
		llmTimeout = 60 * time.Second
	}
	llmClient, err := adapter.NewOpenAIClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, llmTimeout)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}
	a.llmClient = llmClient

	embedBaseURL := cfg.Embedding.BaseURL
	if embedBaseURL == "" {
		embedBaseURL = cfg.LLM.BaseURL
	}
	embedAPIKey := cfg.Embedding.APIKey
	if embedAPIKey == "" {
		embedAPIKey = cfg.LLM.APIKey
	}
	embedTimeout := cfg.Embedding.Timeout
	if embedTimeout <= 0 {
		embedTimeout = 30 * time.Second
	}
	embeddingClient := adapter.NewOpenAIEmbeddingClient(embedBaseURL, embedAPIKey, cfg.Embedding.Model, embedTimeout)
	slog.Info("LLM/Embedding 客户端已初始化",
		"llm_base_url", cfg.LLM.BaseURL,
		"embedding_base_url", embedBaseURL,
		"llm_model", cfg.LLM.Model,
		"embedding_model", cfg.Embedding.Model)

	// Cross-encoder 重排序
	if cfg.Rerank.Enabled && cfg.Rerank.PythonPath != "" && cfg.Rerank.ScriptPath != "" {
		a.reranker = adapter.NewSubprocessReranker(cfg.Rerank.PythonPath, cfg.Rerank.ScriptPath)
		slog.Info("Cross-encoder 重排序已启用", "python", cfg.Rerank.PythonPath, "script", cfg.Rerank.ScriptPath)
	} else {
		slog.Info("Cross-encoder 重排序已禁用，将降级跳过")
	}

	// pgvector 向量存储
	vectorStore, err := adapter.NewPgvectorStore(db)
	if err != nil {
		slog.Warn("pgvector 初始化失败，向量检索/知识发布功能将不可用", "error", err)
		// 不阻塞启动：问答仍可用（降级到纯 BM25），但知识发布返回 20002
	} else {
		a.vectorStore = vectorStore
		slog.Info("pgvector VectorStore 已连接")
	}

	// MinIO 对象存储
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
		} else if mc, err := adapter.NewMinIOClient(minioClient, "opsmind-attachments", "opsmind-documents", "opsmind-published"); err != nil {
			slog.Error("MinIO bucket 初始化失败，文档上传将降级", "error", err)
		} else {
			a.storageClient = mc
			slog.Info("MinIO 对象存储已连接", "endpoint", minioEndpoint)
		}
	}

	// 4. Repository 层
	configRepo := repository.NewConfigRepo(db)
	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	ticketRepo := repository.NewTicketRepo(db)
	knowledgeRepo := repository.NewKnowledgeRepo(db)
	chatRepo := repository.NewChatRepo(db)
	messageRepo := repository.NewMessageRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	dashboardRepo := repository.NewDashboardRepo(db)

	// 5. Service 层（无 RAG 依赖的部分）
	txManager := service.NewGormTxManager(db)
	menuRepo := repository.NewMenuRepo(db)

	// 用户状态缓存（减少每个 API 请求的 DB 查询）
	userCache := cache.NewUserStatusCache(db, 30*time.Second)
	slog.Info("用户状态缓存已创建", "ttl", "30s")

	a.authService = service.NewAuthService(userRepo, menuRepo, db, cfg.JWT)
	userService := service.NewUserService(userRepo, auditRepo, db, userCache)
	roleService := service.NewRoleService(roleRepo, menuRepo, auditRepo, db)
	messageService := service.NewMessageService(messageRepo)
	dashboardService := service.NewDashboardService(dashboardRepo)
	configService := service.NewConfigService(configRepo, auditRepo)
	configService.SetChatRepo(chatRepo)

	llmConfigRepo := repository.NewLlmConfigRepo(db)
	llmConfigSvc, err := service.NewLLMConfigService(llmConfigRepo, db, auditRepo)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 配置服务失败: %w", err)
	}
	slog.Info("LLM 配置服务已初始化")

	// RAG 引擎组件
	embedder := rag.NewEmbedder(embeddingClient, 5)
	docParser := rag.NewDocParser()
	chunker := rag.NewChunker(cfg.AI.ChunkSize, cfg.AI.ChunkOverlap)

	// 向量检索器仅当 vectorStore 可用时创建
	var vectorRetriever *rag.VectorRetriever
	if a.vectorStore != nil {
		vectorRetriever = rag.NewVectorRetriever(embedder, a.vectorStore)
	}

	bm25TTL := 30 * time.Minute
	if s := os.Getenv("OPSMIND_AI_BM25_REBUILD_MINUTES"); s != "" {
		var minutes int
		if _, err := fmt.Sscanf(s, "%d", &minutes); err == nil && minutes > 0 {
			bm25TTL = time.Duration(minutes) * time.Minute
		}
	}
	segmenter := rag.NewGseSegmenter()
	bm25Retriever := rag.NewBM25Retriever(segmenter, bm25TTL)

	pipeline := rag.NewPipeline(vectorRetriever, bm25Retriever, llmClient, embedder, a.reranker, cfg.LLM.Model)

	// 文档处理器仅当 vectorStore 或 storageClient 可用时创建
	var processor *rag.Processor
	if a.vectorStore != nil || a.storageClient != nil {
		procWorkers := 2
		if s := os.Getenv("OPSMIND_AI_PROCESSOR_WORKERS"); s != "" {
			var n int
			if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
				procWorkers = n
			}
		}
		processor = rag.NewProcessor(docParser, chunker, embedder, a.vectorStore, a.storageClient, procWorkers)
	}

	knowledgeService := service.NewKnowledgeService(knowledgeRepo,
		service.WithUserNames(userRepo),
		service.WithChunker(chunker),
		service.WithEmbedder(embedder),
		service.WithVectorStore(a.vectorStore),
		service.WithDocParser(docParser),
		service.WithProcessor(processor),
		service.WithStorage(a.storageClient),
		service.WithAuditRepo(auditRepo),
		service.WithDefaultEmbeddingModel(cfg.Embedding.Model),
		service.WithOnKBChanged(func(kbID int64) {
			// publish/disable 后异步重建该 KB 的 BM25 索引（含标签关键词）
			go rebuildBM25ForKB(knowledgeRepo, a.vectorStore, bm25Retriever, kbID)
		}),
		service.WithMessageNotifier(messageService),
	)
	slog.Info("KnowledgeService 已初始化")

	// TicketService 依赖 KnowledgeService 的 CreateArticle（知识候选），
	// 通过 KnowledgeCandidateSaver 消费者接口注入，消除循环依赖。
	ticketService := service.NewTicketService(ticketRepo, txManager, messageService, knowledgeService, nil) // feedbackMarker 在 chatService 创建后注入

	llmService := service.NewLLMService(llmClient, llmConfigSvc.GetManager(), cfg.LLM.Model, pipeline, embedder, cfg.AI.MaxHistoryMessages)
	slog.Info("LLMService 已初始化")

	// LLM 默认配置变更回调：重建 LLM/Embedding 客户端以反映新的 BaseURL/APIKey/Model
	llmConfigSvc.GetManager().OnChange(func() {
		newCfg := llmConfigSvc.GetManager().GetConfig()
		if newCfg == nil {
			return
		}
		newLLM, err := adapter.NewOpenAIClient(newCfg.LLMBaseURL, newCfg.LLMAPIKey, llmTimeout)
		if err != nil {
			slog.Error("LLM 配置变更后重建客户端失败", "error", err)
			return
		}
		llmService.SetLLMClient(newLLM)

		embedBase := newCfg.GetEmbeddingBaseURL()
		embedKey := newCfg.GetEmbeddingAPIKey()
		newEmbed := adapter.NewOpenAIEmbeddingClient(embedBase, embedKey, newCfg.EmbeddingModel, embedTimeout)
		embedder.SetClient(newEmbed)
		knowledgeService.SetDefaultEmbeddingConfig(newCfg.EmbeddingModel)

		slog.Info("LLM/Embedding 客户端已按新默认配置重建",
			"llm_base_url", newCfg.LLMBaseURL,
			"embedding_base_url", embedBase,
			"llm_model", newCfg.LLMModel,
			"embedding_model", newCfg.EmbeddingModel,
		)
	})

	genHub := service.NewGenerationHub()
	slog.Info("GenerationHub 已初始化")

	chatService := service.NewChatService(knowledgeRepo, chatRepo, llmService, service.RAGDefaults{
		TopK:         cfg.AI.DefaultTopK,
		QueryRewrite: cfg.AI.RAGQueryRewrite,
		MultiRoute:   cfg.AI.RAGMultiRoute,
		Hybrid:       cfg.AI.RAGHybrid,
		Rerank:       cfg.AI.RAGRerank,
	}, configService, auditRepo, genHub)
	slog.Info("ChatService 已初始化")

	// 将 ChatService 作为隐式反馈标记器注入 TicketService（申告创建 → AI 回答自动标记）
	ticketService.SetFeedbackMarker(chatService)

	// 启动时清理残留的 generating 消息（上次异常退出遗留）
	if err := chatService.CleanupStaleGenerating(context.Background()); err != nil {
		slog.Warn("清理残留 generating 消息失败", "error", err)
	}

	auditService := service.NewAuditService(auditRepo)

	// 6. Handler 层
	handlers := &router.Handlers{
		Auth:      handler.NewAuthHandler(a.authService),
		User:      handler.NewUserHandler(userService),
		Role:      handler.NewRoleHandler(roleService),
		Ticket:    handler.NewTicketHandler(ticketService),
		Knowledge: handler.NewKnowledgeHandler(knowledgeService),
		Chat:      handler.NewChatHandler(chatService),
		Message:   handler.NewMessageHandler(messageService),
		Dashboard: handler.NewDashboardHandler(dashboardService),
		Audit:     handler.NewAuditHandler(auditService),
		Config:    handler.NewConfigHandler(configService),
		LLMConfig: handler.NewLLMConfigHandler(llmConfigSvc),
	}

	// 7. 调度器
	a.scheduler = service.NewScheduler(ticketService)
	slog.Info("后台调度器已创建")

	// 8. HTTP Server
	r := router.Setup(cfg, userCache, handlers, func() error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Ping()
		})

	readTimeout := cfg.Server.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 15 * time.Second
	}
	writeTimeout := cfg.Server.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 60 * time.Second
	}
	idleTimeout := cfg.Server.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 60 * time.Second
	}

	a.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return a, nil
}

// run 启动服务并等待退出信号，执行优雅关闭。
//
// 使用 channel 替代 goroutine 中的 os.Exit：
// goroutine 内 os.Exit(1) 会跳过所有 defer 导致资源泄漏。
// 通过 serveErr 通道将错误传递给主 goroutine 统一处理。
func (a *app) run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.scheduler.Start(ctx)

	// 启动 HTTP 服务
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("HTTP 服务已启动", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- fmt.Errorf("HTTP 服务启动失败: %w", err)
		}
	}()

	// 等待退出信号或服务错误
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("收到退出信号，开始优雅关闭...", "signal", sig)
	case err := <-serveErr:
		// HTTP 启动失败，仍然执行关闭链（scheduler/cancel/cleanup）
		slog.Error("HTTP 服务异常退出，开始关闭...", "error", err)
		defer func() {
			// 仅在 serveErr 路径返回错误给 main
			// 正常信号退出不返回错误
		}()
	}

	// 优雅关闭
	a.scheduler.Stop()
	a.authService.Shutdown()
	cancel()

	// 关闭 reranker 子进程
	if r, ok := a.reranker.(*adapter.SubprocessReranker); ok {
		r.Close()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP 服务关闭失败", "error", err)
	} else {
		slog.Info("HTTP 服务已关闭")
	}

	if a.logCleanup != nil {
		a.logCleanup()
	}

	slog.Info("OpsMind 服务已停止")
	return nil
}

// rebuildBM25ForKB 从 DB 加载 KB 下所有已发布文章的分块和标签，重建 BM25 索引。
func rebuildBM25ForKB(repo *repository.KnowledgeRepo, store adapter.VectorStore, bm25 *rag.BM25Retriever, kbID int64) {
	ctx := context.Background()

	// 查询该 KB 下所有已发布文章
	articles, _, err := repo.ListArticles(ctx, kbID, int(model.ArticleStatusPublished), 0, "", "", 1, 10000)
	if err != nil {
		slog.Warn("BM25 索引重建：查询文章列表失败", "kb_id", kbID, "error", err)
		return
	}

	var docs []rag.BM25Document
	for _, a := range articles {
		chunks, err := store.GetChunksByArticle(ctx, a.ID)
		if err != nil {
			slog.Warn("BM25 索引重建：查询分块失败", "article_id", a.ID, "error", err)
			continue
		}

		// 解析标签 JSONB → []string
		var tagList []string
		if len(a.Tags) > 0 {
			_ = json.Unmarshal(a.Tags, &tagList)
		}

		for _, c := range chunks {
			docs = append(docs, rag.BM25Document{
				ChunkID:    c.ID,
				ArticleID:  a.ID,
				KBID:       kbID,
				Content:    c.Content,
				ChunkIndex: c.ChunkIndex,
				Tags:       tagList,
			})
		}
	}

	if len(docs) == 0 {
		// 无已发布文章 → 清空索引
		bm25.BuildIndex(kbID, nil)
		return
	}

	bm25.BuildIndex(kbID, docs)
	slog.Info("BM25 索引重建完成", "kb_id", kbID, "docs", len(docs))
}
