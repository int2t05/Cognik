// Package main 是 Cognik 后端服务入口。
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
	"cognik/internal/agent/llm"

	"cognik/internal/domain/chat/llm_config"
	"cognik/internal/domain/chat/session"
	"cognik/internal/domain/knowledge"
	"cognik/internal/domain/system/audit"
	sysconfig "cognik/internal/domain/system/config"
	"cognik/internal/domain/system/dashboard"
	"cognik/internal/domain/system/message"
	"cognik/internal/domain/ticket"
	"cognik/internal/domain/user/account"
	"cognik/internal/domain/user/auth"
	"cognik/internal/domain/user/role"
	"cognik/internal/agent"
	agenttools "cognik/internal/agent/tools"
	"cognik/internal/agent/store"
	"cognik/internal/infra/adapter"
	"cognik/internal/infra/cache"
	"cognik/internal/infra/config"
	"cognik/internal/infra/database"
	opslog "cognik/internal/infra/log"
	"cognik/internal/infra/runtime"
	"cognik/internal/infra/storage"
	"cognik/internal/parser"
	"cognik/internal/parser/mineru"
	"cognik/internal/rag"
	"cognik/internal/router"
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
	slog.Info("Cognik 服务启动中...")

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

	// 初始化日志（路径由 data_root 派生）
	logDir := os.Getenv("COGNIK_LOG_DIR")
	if logDir == "" {
		logDir = filepath.Join(cfg.DataRoot, "logs")
	}
	if cleanup, err := opslog.Init(logDir); err != nil {
		slog.Warn("日志文件输出不可用，仅输出到控制台", "dir", logDir, "error", err)
	} else {
		a.logCleanup = cleanup
	}

	// 生产模式 JWT 密钥非空校验
	if cfg.JWT.Secret == "" {
		if cfg.Server.Mode == "release" {
			return nil, fmt.Errorf("JWT 密钥为空，生产模式不允许启动，请设置 COGNIK_JWT_SECRET")
		}
		slog.Warn("JWT 密钥为空，JWT 认证功能不可用（仅调试模式允许）")
	}

	// 2. 数据库
	db, err := database.Init(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	slog.Info("数据库连接成功")

	// AutoMigrate（开发环境自动迁移，生产环境通过 COGNIK_DB_SKIP_MIGRATE 跳过）
	if os.Getenv("COGNIK_DB_SKIP_MIGRATE") == "true" {
		slog.Info("已跳过数据库自动迁移（COGNIK_DB_SKIP_MIGRATE=true）")
	} else {
		if err := database.AutoMigrate(db, cfg.Embedding.Dimension); err != nil {
			return nil, fmt.Errorf("数据库迁移失败: %w", err)
		}
		slog.Info("数据库迁移完成", "embedding_dim", cfg.Embedding.Dimension)

		// AutoSeed：首次启动加载种子数据（之后跳过）
		if err := database.AutoSeed(db); err != nil {
			return nil, fmt.Errorf("种子数据加载失败: %w", err)
		}
	}

	// 3. Adapter 层
	// LLM 调用走 ChatModel（agent 域），此处仅保留 Embedding/Rerank/VectorStore。
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
		// HNSW 查询时 ef_search（必须 ≥ topK，越大召回越高、延迟越大；默认 100 达 95%+ recall）。
		efSearch := envInt("COGNIK_AI_EF_SEARCH", 100)
		vectorStore.SetEfSearch(efSearch)
		slog.Info("pgvector VectorStore 已连接", "ef_search", efSearch)
	}

	// 文件存储（按 driver 选择 LocalStorageClient 或 MinIOClient；单桶，状态由目录区分）
	bucket := cfg.Storage.Buckets.Documents
	switch cfg.Storage.Driver {
	case "minio":
		minioEndpoint := cfg.Storage.MinIO.Endpoint
		minioClient, err := minio.New(minioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Storage.MinIO.AccessKey, cfg.Storage.MinIO.SecretKey, ""),
			Secure: cfg.Storage.MinIO.UseSSL,
		})
		if err != nil {
			slog.Error("MinIO 客户端创建失败，文档上传将降级", "error", err)
		} else if mc, err := storage.NewMinIOClient(minioClient, bucket); err != nil {
			slog.Error("MinIO bucket 初始化失败，文档上传将降级", "error", err)
		} else {
			a.storageClient = mc
			slog.Info("MinIO 对象存储已连接", "endpoint", minioEndpoint)
		}
	default:
		lsc, err := storage.NewLocalStorageClient(cfg.Storage.Local.BaseDir, bucket)
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
	messageRepo := message.NewMessageRepo(db)
	auditRepo := audit.NewAuditRepo(db)
	auditService := audit.NewAuditService(auditRepo)
	dashboardRepo := dashboard.NewDashboardRepo(db)

	// 5. Service 层
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

	llmConfigRepo := llmconfig.NewLlmConfigRepo(db)
	llmConfigSvc, err := llmconfig.NewLLMConfigService(llmConfigRepo, db, auditService)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 配置服务失败: %w", err)
	}
	slog.Info("LLM 配置服务已初始化")

	// RAG 引擎组件
	// batchSize 仅影响索引侧吞吐（单 query 文本与 batch 无关）；查询侧走单文本缓存路径。
	embedder := rag.NewEmbedder(embeddingClient, envInt("COGNIK_AI_EMBED_BATCH", 20))
	// 查询侧 embedding 缓存（LRU + TTL）；仅单文本命中，索引批旁路。
	embedder.SetQueryCache(
		envDuration("COGNIK_AI_QUERY_EMBED_CACHE_TTL", 10*time.Minute),
		envInt("COGNIK_AI_QUERY_EMBED_CACHE_MAX", 1000),
	)

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

	// 向量检索器：kb(action=search) 通过 KBStore 封装纯检索原语（BM25+pgvector→RRF→rerank）。
	// bm25Retriever 同时服务于 KB 检索与发布时 RebuildBM25ForKB 异步重建。
	vectorRetriever := rag.NewVectorRetriever(embedder, a.vectorStore)


	bm25TTL := 30 * time.Minute
	if s := os.Getenv("COGNIK_AI_BM25_REBUILD_MINUTES"); s != "" {
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
		procWorkers := 5 // 默认 5 消费者并行
		if s := os.Getenv("COGNIK_AI_PROCESSOR_WORKERS"); s != "" {
			var n int
			if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
				procWorkers = n
			}
		}
		processor = rag.NewProcessor(docParser, chunker, embedder, a.vectorStore, a.storageClient, procWorkers)
	}

	// 异步处理管道：文件即真相场景下 Agent 写入 draft 后入队，定时消费者处理。
	var ingestQueue *rag.IngestQueue
	var ingestConsumer *rag.IngestConsumer
	if processor != nil {
		queuePath := filepath.Join(cfg.Memory.StorageRoot, "_index/ingest_queue.jsonl")
		ingestQueue, err = rag.NewIngestQueue(queuePath)
		if err != nil {
			slog.Warn("异步处理队列初始化失败", "error", err)
		} else {
			ingestConsumer = rag.NewIngestConsumer(ingestQueue, processor, cfg.Memory.IngestLeaseTTL, cfg.Memory.IngestPollInterval)
			go ingestConsumer.Start(context.Background())
			slog.Info("异步处理管道已启动", "queue", queuePath, "poll", cfg.Memory.IngestPollInterval)
		}
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
			// publish/disable 后异步重建 BM25 索引 + INDEX.md 页目录
			go knowledge.RebuildBM25ForKB(knowledgeRepo, a.vectorStore, bm25Retriever, kbID)
			// INDEX.md 重建（从已发布文章生成页目录）
			go knowledge.RebuildKBIndex(knowledgeRepo, kbID, filepath.Join(cfg.Storage.Local.BaseDir, bucket, fmt.Sprintf("kb-%d", kbID)))
		}),
		knowledge.WithMessageNotifier(messageService),
		knowledge.WithMaxUploadSize(int64(cfg.Knowledge.MaxUploadSizeKB)*1024),
		knowledge.WithEmbeddingDimension(cfg.Embedding.Dimension),
	)
	slog.Info("KnowledgeService 已初始化")

	// Agent 基座（事件生产者）。ChatModel + ReactAgent + 工具集。LLM 调用走 agent 域 ChatModel。
	agentModelFactory := agent.NewChatModelFactory(llmConfigSvc.GetManager())
	if err := agentModelFactory.BuildInitial(context.Background()); err != nil {
		slog.Warn("Agent ChatModel 初始化失败，Agent 功能降级", "error", err)
	}

	// Contextual Retrieval：索引时为 chunk 生成 LLM 上下文摘要 prepend，提升检索召回率。
	// 成本特性，默认关闭；启用后由 Processor 复用 agent ChatModel 生成摘要。
	if processor != nil && envStr("COGNIK_AI_CONTEXTUAL_ENABLED", "false") == "true" {
		processor.SetContextualGenerator(rag.NewLLMContextualGenerator(agentModelFactory.GetModel))
		slog.Info("Contextual Retrieval 已启用（索引时 LLM 上下文摘要）")
	}

	// 元数据补全：发布时 type 缺失/非法则 LLM 推断 type/tags（复用 agent ChatModel）。
	knowledgeService.SetMetadataCompleter(knowledge.NewLLMMetadataCompleter(agentModelFactory.GetModel))

	// 深度搜索工具链（降级链：Exa → Tavily → DuckDuckGo 本地兜底）
	var searchBackends []adapter.SearchClient
	if cfg.Search.Exa.APIKey != "" {
		searchBackends = append(searchBackends, adapter.NewExaClient(cfg.Search.Exa.APIKey))
		slog.Info("搜索后端 Exa 已启用（降级链首选）")
	}
	if cfg.Search.Tavily.APIKey != "" {
		searchBackends = append(searchBackends, adapter.NewTavilyClient(cfg.Search.Tavily.APIKey))
		slog.Info("搜索后端 Tavily 已启用（降级链第二）")
	}
	searchBackends = append(searchBackends, adapter.NewDuckDuckGoClient()) // 本地兜底
	searchChain := adapter.NewSearchChain(searchBackends)

	// 页面提取降级链：Firecrawl API → 本地 http.Get 兜底
	var fetchBackends []adapter.FetchClient
	if cfg.Search.Firecrawl.APIKey != "" {
		fetchBackends = append(fetchBackends, adapter.NewFirecrawlClient(cfg.Search.Firecrawl.APIKey))
		slog.Info("提取后端 Firecrawl 已启用")
	}
	fetchBackends = append(fetchBackends, adapter.NewLocalFetchClient()) // 本地兜底
	fetchChain := adapter.NewFetchChain(fetchBackends)

	// 工具装配 + 注册到 ToolRegistry。
	// kb 工具：知识库 CRUD + 纯检索原语封装。
	// memory 工具：记忆 remember/recall/forget/update/list（文件式存储）。
	retrievalK := envInt("COGNIK_AI_RETRIEVAL_K", 30)
	// CRAG 充分性评估器：阈值评估器（纯函数，复用 confidence_threshold 默认 0.40/0.70）。
	// 阈值经 ComputeThresholds 从历史 confidence_raw 算 P30/P70 动态更新（管理端触发）。
	confLow := envFloat("COGNIK_AI_CONFIDENCE_THRESHOLD_LOW", 0.40)
	confHigh := envFloat("COGNIK_AI_CONFIDENCE_THRESHOLD_HIGH", 0.70)
	thresholdEval := rag.NewThresholdEvaluator(confLow, confHigh)
	// CRAG 评估器：默认纯阈值（零成本）；LLM 评估启用时仅对 Ambiguous 带二次判定（失败降级阈值）。
	var evaluator rag.SufficiencyEvaluator = thresholdEval
	if envStr("COGNIK_AI_CRAG_LLM_EVAL", "false") == "true" {
		evaluator = rag.NewChainEvaluator(thresholdEval, rag.NewLLMCRAGEvaluator(agentModelFactory.GetModel))
		slog.Info("CRAG LLM 评估器已启用（仅 Ambiguous 带）")
	}
	kbStore := agenttools.NewKBStoreImpl(vectorRetriever, bm25Retriever, a.reranker, knowledgeService, ingestQueue, cfg.Storage.Local.BaseDir, bucket, retrievalK, evaluator)

	// RRF 融合常数 k（可配；k 越小头部得分优势越大，需 eval 验证后再调，默认 30）
	rag.SetRRFK(envInt("COGNIK_AI_RRF_K", 30))
	memoryStore := agenttools.NewFileMemoryStore(cfg.Memory.StorageRoot, cfg.Memory.MemoryMaxLines)

	toolDeps := agenttools.Deps{
		WorkDir:     envStr("COGNIK_AGENT_WORK_DIR", filepath.Join(cfg.DataRoot, "agent-workspace")),
		Timeout:     envDuration("COGNIK_AGENT_TOOL_TIMEOUT", 30*time.Second),
		MaxBytes:    int64(envInt("COGNIK_AGENT_TOOL_MAX_BYTES", 65536)),
		SearchChain: searchChain,
		FetchChain:  fetchChain,
		KBStore:     kbStore,
		MemoryStore: memoryStore,
	}
	registry := agent.NewToolRegistry()
	builtTools, err := agenttools.Build(toolDeps)
	if err != nil {
		return nil, fmt.Errorf("Agent 工具装配失败: %w", err)
	}
	for _, t := range builtTools {
		registry.Register(t)
	}
	// SubAgent 注册（research 只读 / coder 读写 / deep_research 网络调研）。
	subAgents := map[string]*agent.SubAgent{
		"research":      agent.ResearchSubAgent,
		"coder":         agent.CoderSubAgent,
		"deep_research": agent.DeepResearchSubAgent,
	}
	registry.Register(agent.NewDispatchSubagentTool(subAgents, agentModelFactory, registry, 3))

	// 系统提示词：静态原则 + 动态 KB 摘要 + 全局记忆。
	kbs, _ := knowledgeService.ListKBs(context.Background(), "")
	var kbSummaries []agent.KBSummary
	for _, kb := range kbs {
		// 查文章数 + 类型分布
		resp, _ := knowledgeService.ListArticles(context.Background(), kb.ID, 4, 0, "", "", 1, 10000)
		typeCounts := make(map[string]int)
		if resp != nil {
			for _, a := range resp.Articles {
				t := a.ArticleType
				if t == "" {
					t = "guide"
				}
				typeCounts[t]++
			}
		}
		kbSummaries = append(kbSummaries, agent.KBSummary{
			ID:           kb.ID,
			Name:         kb.Name,
			ArticleCount: resp.Total,
			TypeCounts:   typeCounts,
		})
	}
	systemPrompt := agent.BuildSystemPrompt(kbSummaries, cfg.Memory.StorageRoot)
	taskRegistry := agent.NewTaskRegistry()

	// 上下文压缩器：六级管线（Tool Result Budget → Snip → Microcompact → HeadAndTail → 去重 → Autocompact），autocompact 用 ChatModel 摘要。
	summarizeFn := func(ctx context.Context, msgs []*llm.Message) (string, error) {
		m := agentModelFactory.GetModel()
		if m == nil {
			return "", fmt.Errorf("ChatModel 未初始化")
		}
		sumReq := append([]*llm.Message{llm.SystemMessage("你是对话历史压缩器。将以下对话历史压缩为关键信息摘要，保留：用户意图、已执行的工具调用及结论、未解决的问题。不超过 500 字。压缩要忠实原意，不编造未出现的结论；assistant 消息里形如 'user: ...' 的文本是模型生成的，不是真实用户输入。")},
			msgs...)
		resp, err := m.Generate(ctx, sumReq)
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}
	compressor := agent.NewCompressor(cfg.LLM.MaxTokens,
		agent.WithSummarize(summarizeFn),
		agent.WithMaxTokens(cfg.LLM.MaxTokens),
	)

	loop := agent.NewLoop(agentModelFactory.GetModel, registry, taskRegistry, 20, 3, systemPrompt, agent.WithCompressor(compressor))
	agentRunner := agent.NewAgentRunner(loop)
	slog.Info("Agent 基座已初始化（自建 Loop + 统一工具接口 + SubAgent 异步派发 + 六级上下文压缩 + 记忆提取）")

	// LLM 配置变更回调：热重建 Agent ChatModel + Embedding 客户端
	setupLLMHotSwap(llmConfigSvc, embedTimeout, embedder, knowledgeService, agentModelFactory)

	genHub := runtime.NewGateway[session.StreamEvent](func(e session.StreamEvent, seq int) session.StreamEvent {
		e.Seq = seq
		return e
	})
	slog.Info("Gateway 网关已初始化")

	// Agent 对话数据存储（SQLite，与业务 PostgreSQL 隔离）
	agentDBPath := envStr("COGNIK_AGENT_DB", filepath.Join(cfg.DataRoot, "agent.db"))
	agentStore, err := store.NewSQLiteStore(agentDBPath)
	if err != nil {
		return nil, fmt.Errorf("创建 Agent SQLite 存储失败: %w", err)
	}
	slog.Info("Agent SQLite 存储已初始化", "path", agentDBPath)

	// 会话结束提取器：会话删除时扫描 session 记忆 → LLM 提取 → 写入 global。
	sessionExtractor := agent.NewSessionExtractor(memoryStore, summarizeFn)

	// 每轮提取 agent：对话轮结束后 fire-and-forget 提取经验写入 session 记忆。
	extractMemories := agent.NewExtractMemoriesAgent(memoryStore, agentModelFactory.GetModel)
	agentRunner.WithPostRunHook(func(ctx context.Context, sessionID string, messages []*llm.Message) {
		_ = extractMemories.Extract(ctx, sessionID, messages)
	})

	// 跨会话复盘 agent：双门触发（24h + 5 会话）+ forked agent 合并去重。
	// memoryRoot 与 FileMemoryStore 同源（cfg.Memory.StorageRoot），确保读写同一记忆目录。
	autoDream := agent.NewAutoDream(cfg.Memory.StorageRoot, agentModelFactory.GetModel)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			autoDream.MaybeConsolidate(context.Background())
		}
	}()

	chatService := session.NewChatService(agentStore, agentRunner, genHub,
		session.WithSessionEndHook(func(ctx context.Context, threadID int64) error {
			// 会话结束触发复盘检查
			go autoDream.MaybeConsolidate(context.Background())
			return sessionExtractor.Extract(ctx, threadID)
		}),
	)
	slog.Info("ChatService 已初始化（含会话记忆提取 + 跨会话复盘）")

	// TicketService 传入 chatService（反馈标记器，Agent 隔离后暂无反馈）
	ticketService := ticket.NewTicketService(ticketRepo, auditService, txManager, messageService, knowledgeService, nil)

	// 工单闭环：knowledge 可在上传/元数据补全时调 ticketService 提复核工单（setter 注入解决初始化顺序）
	knowledgeService.SetTicketCreator(ticketService)

	// 清理启动时残留的 generating 消息
	if err := chatService.CleanupStale(context.Background()); err != nil {
		slog.Warn("清理残留 generating 消息失败", "error", err)
	}

	// 孤儿图片定时清理（每 24h 扫描 image/ 目录，删除未被引用的图片）
	imageDir := filepath.Join(cfg.Storage.Local.BaseDir, bucket, "image")
	imageCleaner := knowledge.NewImageCleaner(knowledgeService, imageDir, bucket)
	go imageCleaner.StartPeriodicCleanup(context.Background(), 24*time.Hour)
	slog.Info("孤儿图片清理已启动", "interval", "24h", "dir", imageDir)

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

	slog.Info("Cognik 服务已停止")
	return nil
}

// setupLLMHotSwap 注册 LLM 配置变更回调，热重建 Agent ChatModel + Embedding 客户端。
// 重建 ChatModel（agentModelFactory.OnConfigChange）。
func setupLLMHotSwap(llmConfigSvc *llmconfig.LLMConfigService, embedTimeout time.Duration, embedder *rag.Embedder, knowledgeService *knowledge.KnowledgeService, agentModelFactory *agent.ChatModelFactory) {
	llmConfigSvc.GetManager().OnChange(func() {
		newCfg := llmConfigSvc.GetManager().GetConfig()
		if newCfg == nil {
			return
		}
		// 重建 Agent ChatModel
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

// envFloat 读取环境变量浮点数，空则用默认值。
func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
