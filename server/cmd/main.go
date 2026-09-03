// Package main 是 OpsMind 后端服务入口。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"opsmind/internal/domain/chat/llm_config"
	"opsmind/internal/domain/chat/session"
	"opsmind/internal/domain/knowledge"
	"opsmind/internal/domain/system/audit"
	sysconfig "opsmind/internal/domain/system/config"
	"opsmind/internal/domain/system/dashboard"
	"opsmind/internal/domain/system/message"
	"opsmind/internal/domain/ticket"
	"opsmind/internal/domain/user/account"
	"opsmind/internal/domain/user/auth"
	"opsmind/internal/domain/user/role"
	"opsmind/internal/agent"
	agenttools "opsmind/internal/agent/tools"
	"opsmind/internal/infra/adapter"
	"opsmind/internal/infra/cache"
	"opsmind/internal/infra/config"
	"opsmind/internal/infra/database"
	opslog "opsmind/internal/infra/log"
	"opsmind/internal/infra/runtime"
	"opsmind/internal/infra/storage"
	"opsmind/internal/parser"
	"opsmind/internal/parser/mineru"
	"opsmind/internal/rag"
	"opsmind/internal/router"
)

// app 持有所有已初始化的组件。
type app struct {
	cfg           *config.AppConfig
	logCleanup    func()
	reranker      adapter.Reranker
	vectorStore   adapter.VectorStore
	storageClient storage.StorageClient
	scheduler     *runtime.Scheduler
	authService   *auth.AuthService
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

// wireApp 加载配置、初始化组件并注入依赖。
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

		// AutoSeed：首次启动加载种子数据（之后跳过）
		if err := database.AutoSeed(db); err != nil {
			return nil, fmt.Errorf("种子数据加载失败: %w", err)
		}
	}

	// 3. Adapter 层
	// LLM 调用走 Eino ChatModel（agent 域），此处仅保留 Embedding/Rerank/VectorStore。
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
		// 不阻塞启动，降级到纯 BM25
	} else {
		a.vectorStore = vectorStore
		slog.Info("pgvector VectorStore 已连接")
	}

	// 文件存储（按 driver 选择 LocalStorageClient 或 MinIOClient）
	bucketDocs := cfg.Storage.Buckets.Documents
	bucketPub := cfg.Storage.Buckets.Published
	switch cfg.Storage.Driver {
	case "minio":
		minioEndpoint := cfg.Storage.MinIO.Endpoint
		minioClient, err := minio.New(minioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Storage.MinIO.AccessKey, cfg.Storage.MinIO.SecretKey, ""),
			Secure: cfg.Storage.MinIO.UseSSL,
		})
		if err != nil {
			slog.Error("MinIO 客户端创建失败，文档上传将降级", "error", err)
		} else if mc, err := storage.NewMinIOClient(minioClient, bucketDocs, bucketPub); err != nil {
			slog.Error("MinIO bucket 初始化失败，文档上传将降级", "error", err)
		} else {
			a.storageClient = mc
			slog.Info("MinIO 对象存储已连接", "endpoint", minioEndpoint)
		}
	default:
		lsc, err := storage.NewLocalStorageClient(cfg.Storage.Local.BaseDir, bucketDocs, bucketPub)
		if err != nil {
			slog.Error("本地存储初始化失败，文档上传将降级", "error", err)
		} else {
			a.storageClient = lsc
			slog.Info("本地文件存储已就绪", "base_dir", cfg.Storage.Local.BaseDir)
		}
	}

	// 4. Repository 层
	configRepo := sysconfig.NewConfigRepo(db)
	userRepo := account.NewUserRepo(db)
	roleRepo := role.NewRoleRepo(db)
	ticketRepo := ticket.NewTicketRepo(db)
	knowledgeRepo := knowledge.NewKnowledgeRepo(db)
	chatRepo := session.NewChatRepo(db)
	messageRepo := message.NewMessageRepo(db)
	auditRepo := audit.NewAuditRepo(db)
	auditService := audit.NewAuditService(auditRepo)
	dashboardRepo := dashboard.NewDashboardRepo(db)

	// 5. Service 层（无 RAG 依赖的部分）
	txManager := runtime.NewGormTxManager(db)
	menuRepo := role.NewMenuRepo(db)

	// 用户状态缓存（减少每个 API 请求的 DB 查询）
	userCache := cache.NewUserStatusCache(db, 30*time.Second)
	slog.Info("用户状态缓存已创建", "ttl", "30s")

	a.authService = auth.NewAuthService(userRepo, menuRepo, db, cfg.JWT)
	userService := account.NewUserService(userRepo, auditService, db, userCache)
	roleService := role.NewRoleService(roleRepo, menuRepo, auditService, db)
	messageService := message.NewMessageService(messageRepo)
	dashboardService := dashboard.NewDashboardService(dashboardRepo)
	configService := sysconfig.NewConfigService(configRepo, auditService)
	configService.SetChatRepo(chatRepo)

	llmConfigRepo := llmconfig.NewLlmConfigRepo(db)
	llmConfigSvc, err := llmconfig.NewLLMConfigService(llmConfigRepo, db, auditService)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 配置服务失败: %w", err)
	}
	slog.Info("LLM 配置服务已初始化")

	// RAG 引擎组件
	embedder := rag.NewEmbedder(embeddingClient, 5)

	// 文档解析器：MinerU 云端高精度优先，本地纯 Go 库兜底
	var mineruEngine *mineru.Engine
	if cfg.Parser.Engine == "mineru" {
		mineruEngine = mineru.NewEngine(cfg.Parser.MinerU)
		if mineruEngine != nil {
			slog.Info("MinerU 云端解析引擎已启用", "endpoint", cfg.Parser.MinerU.Endpoint)
		} else {
			slog.Warn("MinerU API Key 为空，降级到本地解析", "engine", cfg.Parser.Engine)
		}
	}
	docParser := parser.NewParser(parser.WithMinerU(mineruEngine))
	chunker := rag.NewChunker(cfg.AI.ChunkSize, cfg.AI.ChunkOverlap)

	// 向量检索器 + RAG Pipeline 不参与 Agent 生成路径。
	// bm25Retriever 保留：KB 发布时 RebuildBM25ForKB 异步重建索引仍需。
	

	bm25TTL := 30 * time.Minute
	if s := os.Getenv("OPSMIND_AI_BM25_REBUILD_MINUTES"); s != "" {
		var minutes int
		if _, err := fmt.Sscanf(s, "%d", &minutes); err == nil && minutes > 0 {
			bm25TTL = time.Duration(minutes) * time.Minute
		}
	}
	segmenter := rag.NewGseSegmenter()
	bm25Retriever := rag.NewBM25Retriever(segmenter, bm25TTL)

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

	knowledgeService := knowledge.NewKnowledgeService(knowledgeRepo,
		knowledge.WithUserNames(userRepo),
		knowledge.WithChunker(chunker),
		knowledge.WithEmbedder(embedder),
		knowledge.WithVectorStore(a.vectorStore),
		knowledge.WithDocParser(docParser),
		knowledge.WithProcessor(processor),
		knowledge.WithStorage(a.storageClient),
		knowledge.WithAuditWriter(auditService),
		knowledge.WithDefaultEmbeddingModel(cfg.Embedding.Model),
		knowledge.WithOnKBChanged(func(kbID int64) {
			// publish/disable 后异步重建该 KB 的 BM25 索引（含标签关键词）
			go knowledge.RebuildBM25ForKB(knowledgeRepo, a.vectorStore, bm25Retriever, kbID)
		}),
		knowledge.WithMessageNotifier(messageService),
		knowledge.WithMaxUploadSize(int64(cfg.Knowledge.MaxUploadSizeKB)*1024),
	)
	slog.Info("KnowledgeService 已初始化")

	// Agent 基座（事件生产者）。Eino ChatModel + ReactAgent + 工具集。
	// LLM 调用从手写 OpenAIClient 迁移到 Eino ChatModel；ReAct 循环替代线性 RAG 管道。
	agentModelFactory := agent.NewChatModelFactory(llmConfigSvc.GetManager())
	if err := agentModelFactory.BuildInitial(context.Background()); err != nil {
		slog.Warn("Agent ChatModel 初始化失败，Agent 功能降级", "error", err)
	}
	agentToolFactory := agenttools.NewToolFactory(
		envStr("OPSMIND_AGENT_WORK_DIR", "./data/agent-workspace"),
		envDuration("OPSMIND_AGENT_TOOL_TIMEOUT", 30*time.Second),
		int64(envInt("OPSMIND_AGENT_TOOL_MAX_BYTES", 65536)),
	)
	agentFactory := agent.NewAgentFactory(agentModelFactory, agentToolFactory)
	if cfg := llmConfigSvc.GetManager().GetConfig(); cfg != nil && cfg.SystemPrompt != "" {
		agentFactory.SetInstruction(cfg.SystemPrompt)
	}
	agentRunner := agent.NewAgentRunner(agentFactory)
	slog.Info("Agent 基座已初始化")

	// LLM 配置变更回调：热重建 Agent ChatModel + Embedding 客户端
	setupLLMHotSwap(llmConfigSvc, embedTimeout, embedder, knowledgeService, agentModelFactory)

	genHub := runtime.NewGateway[session.StreamEvent](func(e session.StreamEvent, seq int) session.StreamEvent {
		e.Seq = seq
		return e
	})
	slog.Info("Gateway 网关已初始化")

	chatService := session.NewChatService(knowledgeRepo, chatRepo, agentRunner, agentModelFactory, configService, auditService, genHub)
	slog.Info("ChatService 已初始化")

	// ChatService 就绪后构造 TicketService，直接传入反馈标记器
	ticketService := ticket.NewTicketService(ticketRepo, auditService, txManager, messageService, knowledgeService, chatService)

	// 启动时清理残留的 generating 消息（上次异常退出遗留）
	if err := chatService.CleanupStaleGenerating(context.Background()); err != nil {
		slog.Warn("清理残留 generating 消息失败", "error", err)
	}

	// 6. Handler 层
	handlers := &router.Handlers{
		Auth:      auth.NewAuthHandler(a.authService),
		User:      account.NewUserHandler(userService),
		Role:      role.NewRoleHandler(roleService),
		Ticket:    ticket.NewTicketHandler(ticketService),
		Knowledge: knowledge.NewKnowledgeHandler(knowledgeService),
		Chat:      session.NewChatHandler(chatService),
		Message:   message.NewMessageHandler(messageService),
		Dashboard: dashboard.NewDashboardHandler(dashboardService),
		Audit:     audit.NewAuditHandler(auditService),
		Config:    sysconfig.NewConfigHandler(configService),
		LLMConfig: llmconfig.NewLLMConfigHandler(llmConfigSvc),
	}

	// 7. 调度器
	a.scheduler = runtime.NewScheduler(ticketService)
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
		writeTimeout = 300 * time.Second
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
		slog.Error("HTTP 服务异常退出，开始关闭...", "error", err)
		defer func() {
			// 仅在 serveErr 路径返回错误给 main
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

// setupLLMHotSwap 注册 LLM 配置变更回调，热重建 Agent ChatModel + Embedding 客户端。
// 重建 Eino ChatModel（agentModelFactory.OnConfigChange）。
func setupLLMHotSwap(llmConfigSvc *llmconfig.LLMConfigService, embedTimeout time.Duration, embedder *rag.Embedder, knowledgeService *knowledge.KnowledgeService, agentModelFactory *agent.ChatModelFactory) {
	llmConfigSvc.GetManager().OnChange(func() {
		newCfg := llmConfigSvc.GetManager().GetConfig()
		if newCfg == nil {
			return
		}
		// 重建 Agent Eino ChatModel
		agentModelFactory.OnConfigChange()

		// 重建 Embedding 客户端（文档处理管道仍需）
		embedBase := newCfg.GetEmbeddingBaseURL()
		embedKey := newCfg.GetEmbeddingAPIKey()
		newEmbed := adapter.NewOpenAIEmbeddingClient(embedBase, embedKey, newCfg.EmbeddingModel, embedTimeout)
		embedder.SetClient(newEmbed)
		knowledgeService.SetDefaultEmbeddingConfig(newCfg.EmbeddingModel)

		slog.Info("Agent ChatModel / Embedding 已按新默认配置重建",
			"llm_base_url", newCfg.LLMBaseURL,
			"embedding_base_url", embedBase,
			"llm_model", newCfg.LLMModel,
			"embedding_model", newCfg.EmbeddingModel,
		)
	})
}

// envStr 读取环境变量字符串，空则用默认值。
func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envDuration 读取环境变量 Duration，空则用默认值。
func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// envInt 读取环境变量整数，空则用默认值。
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
