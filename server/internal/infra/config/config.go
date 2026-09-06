// Package config 负责加载配置（Viper 读取 config.yaml + 环境变量覆盖，前缀 COGNIK）。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// AppConfig 是顶层配置结构体，包含所有子模块配置。
type AppConfig struct {
	AppName   string          `mapstructure:"app_name"` // 应用名称，显示在页面标题和系统通知中
	DataRoot  string          `mapstructure:"data_root"` // 数据根目录（storage/memory/logs/agent.db 派生自此）
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Storage   StorageConfig   `mapstructure:"storage"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Rerank    RerankConfig    `mapstructure:"rerank"`
	AI        AIConfig        `mapstructure:"ai"`
	Search    SearchConfig    `mapstructure:"search"`
	CORS      CORSConfig      `mapstructure:"cors"`
	Parser    ParserConfig    `mapstructure:"parser"`
	Knowledge KnowledgeConfig `mapstructure:"kb"`
	Memory    MemoryConfig    `mapstructure:"memory"`
}

// SearchConfig 深度搜索工具链配置（web_search/web_fetch 后端，降级链模式）。
type SearchConfig struct {
	Exa        ExaConfig       `mapstructure:"exa"`
	Tavily     TavilyConfig    `mapstructure:"tavily"`
	Firecrawl  FirecrawlConfig `mapstructure:"firecrawl"`
	MaxResults int             `mapstructure:"max_results"` // 默认 5
	Timeout    time.Duration   `mapstructure:"timeout"`     // 默认 10s
}

// ExaConfig 语义搜索 API（降级链首选）。
type ExaConfig struct {
	APIKey string `mapstructure:"api_key"` // 空=不加入降级链
}

// TavilyConfig Agent 优化型搜索 API（降级链第二）。
type TavilyConfig struct {
	APIKey string `mapstructure:"api_key"` // 空=不加入降级链
}

// FirecrawlConfig 页面提取 API（URL → 干净 Markdown，JS 渲染）。
type FirecrawlConfig struct {
	APIKey string `mapstructure:"api_key"` // 空=仅用本地兜底
}

// CORSConfig 跨域配置，AllowOrigins 为逗号分隔列表。
type CORSConfig struct {
	AllowOrigins string `mapstructure:"allow_origins"`
}

// ServerConfig 是 HTTP 服务器配置。
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`          // debug / release
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`  // HTTP 读取超时
	WriteTimeout time.Duration `mapstructure:"write_timeout"` // HTTP 写入超时（SSE 内部续期）
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`  // HTTP 空闲超时
}

// DatabaseConfig 是 PostgreSQL 数据库配置。
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`    // 最大连接数，默认 25
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`    // 最大空闲连接数，默认 10
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // 连接最大存活时间，默认 5m
}

// JWTConfig 是 JWT 令牌配置。
type JWTConfig struct {
	Secret        string        `mapstructure:"secret"`
	AccessExpire  time.Duration `mapstructure:"access_expire"`
	RefreshExpire time.Duration `mapstructure:"refresh_expire"`
}

// MinIOConfig 是 MinIO 对象存储配置。
type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// StorageConfig 文件存储配置，Driver 选择驱动：local | minio。
type StorageConfig struct {
	Driver  string             `mapstructure:"driver"` // local | minio
	Local   LocalStorageConfig `mapstructure:"local"`
	MinIO   MinIOConfig        `mapstructure:"minio"`
	Buckets BucketConfig       `mapstructure:"buckets"`
}

// LocalStorageConfig 是本地文件系统存储配置。
type LocalStorageConfig struct {
	BaseDir string `mapstructure:"base_dir"`
}

// BucketConfig 是存储桶配置（单桶；草稿/已发布由二级目录区分，不再按桶分状态）。
type BucketConfig struct {
	Documents string `mapstructure:"documents"`
}

// LLMConfig 大语言模型配置（支持任意 OpenAI-compatible API）。
type LLMConfig struct {
	BaseURL   string        `mapstructure:"base_url"`
	APIKey    string        `mapstructure:"api_key"`
	Model     string        `mapstructure:"model"`
	MaxTokens int           `mapstructure:"max_tokens"`
	Timeout   time.Duration `mapstructure:"timeout"` // LLM 调用超时，默认 300s
}

// RerankConfig cross-encoder 重排序子进程配置（Python stdin/stdout JSON Lines 通信）。
// Enabled 为 false 时 Pipeline 降级跳过重排序。
type RerankConfig struct {
	Enabled    bool   `mapstructure:"enabled"`     // 是否启用 cross-encoder 重排序，默认 true
	PythonPath string `mapstructure:"python_path"` // Python 解释器路径，如 "python3"
	ScriptPath string `mapstructure:"script_path"` // rerank_server.py 绝对路径
}

// EmbeddingConfig 文本向量化配置，可独立于 LLM 配置 BaseURL/APIKey。
// 为空时回退到 llm.base_url / llm.api_key。
type EmbeddingConfig struct {
	BaseURL   string        `mapstructure:"base_url"`
	APIKey    string        `mapstructure:"api_key"`
	Model     string        `mapstructure:"model"`
	Dimension int           `mapstructure:"dimension"`
	Timeout   time.Duration `mapstructure:"timeout"` // Embedding 调用超时，默认 60s
}

// AIConfig AI 问答配置（ChunkSize/ChunkOverlap 供文档处理管道使用）。
type AIConfig struct {
	ChunkSize           int `mapstructure:"chunk_size"`    // 文本分块大小（字符数），默认 500
	ChunkOverlap        int `mapstructure:"chunk_overlap"` // 分块重叠大小（字符数），默认 100
	RAGEnabled         bool `mapstructure:"rag_enabled"`     // RAG 检索开关，默认 true
	TopK               int `mapstructure:"top_k"`         // 默认检索 Top K，默认 5
	ConfidenceThreshold float64 `mapstructure:"confidence_threshold"` // 置信度阈值，默认 0.6
	MaxHistoryMessages int `mapstructure:"max_history_messages"` // 多轮对话历史消息数上限，默认 10
}

// ParserConfig 文档解析引擎配置（mineru 云端高精度 / local 本地库），mineru 不可用时自动降级。
type ParserConfig struct {
	Engine string       `mapstructure:"engine"` // mineru | local，默认 mineru
	MinerU MinerUConfig `mapstructure:"mineru"`
	Python PythonConfig `mapstructure:"python"`
}

// MinerUConfig MinerU 云端结构化提取配置，APIKey 为空时降级到本地解析。
type MinerUConfig struct {
	APIKey   string        `mapstructure:"api_key"`
	Endpoint string        `mapstructure:"endpoint"` // 默认 https://mineru.net/api/v4
	Timeout  time.Duration `mapstructure:"timeout"`  // 轮询超时，默认 180s
}

// PythonConfig 是本地解析可能调用的 Python 解释器路径。
type PythonConfig struct {
	Path string `mapstructure:"path"` // 默认 python3
}

// KnowledgeConfig 知识库配置。
type KnowledgeConfig struct {
	MaxUploadSizeKB int `mapstructure:"max_upload_size"` // 上传大小上限(KB)，默认 51200(50MB)
}

// MemoryConfig 记忆系统配置（文件式存储根目录 + MEMORY.md 行数上限 + 上下文压缩阈值）。
type MemoryConfig struct {
	StorageRoot        string        `mapstructure:"storage_root"`         // 记忆存储根目录，空 → {data_root}/memory
	MemoryMaxLines     int           `mapstructure:"memory_max_lines"`     // MEMORY.md 最大行数，默认 200
	CompressDedup      float64       `mapstructure:"compress_dedup"`       // 去重清理触发阈值，默认 0.70
	CompressCompact    float64       `mapstructure:"compress_compact"`     // Autocompact 触发阈值，默认 0.85
	IngestPollInterval time.Duration `mapstructure:"ingest_poll_interval"` // 重试扫描器轮询间隔，默认 5s
}

// Load 加载配置文件并应用环境变量覆盖。
// configPath 为空时使用默认路径 ./internal/config/config.yaml。
func Load(configPath string) (*AppConfig, error) {
	// 加载 .env 文件（Docker Compose 已注入环境变量，找不到文件不报错）
	_ = godotenv.Load("../.env")
	_ = godotenv.Load()

	v := viper.New()

	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./internal/config")
	}

	// 配置文件不存在不报错，但格式错误必须暴露
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	bindEnvs(v)

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 从 data_root 派生 storage/memory 路径（未显式配置时）
	cfg.resolveDataPaths()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}
	return &cfg, nil
}

// EnvConfigEntry .env 派生配置项(前端读写,触发后端热加载)。
type EnvConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetEnvConfigs 返回 .env 派生的全部配置项(API key 脱敏)。
func GetEnvConfigs(cfg *AppConfig) []EnvConfigEntry {
	return []EnvConfigEntry{
		// 通用
		{Key: "app_name", Value: cfg.AppName},
		// LLM
		{Key: "llm_base_url", Value: cfg.LLM.BaseURL},
		{Key: "llm_api_key", Value: maskEnvKey(cfg.LLM.APIKey)},
		{Key: "llm_model", Value: cfg.LLM.Model},
		{Key: "llm_max_tokens", Value: fmt.Sprintf("%d", cfg.LLM.MaxTokens)},
		// Embedding
		{Key: "embedding_base_url", Value: cfg.Embedding.BaseURL},
		{Key: "embedding_api_key", Value: maskEnvKey(cfg.Embedding.APIKey)},
		{Key: "embedding_model", Value: cfg.Embedding.Model},
		{Key: "embedding_dimension", Value: fmt.Sprintf("%d", cfg.Embedding.Dimension)},
		// RAG
		{Key: "ai.rag_enabled", Value: boolStr(cfg.AI.RAGEnabled)},
		{Key: "ai.top_k", Value: fmt.Sprintf("%d", cfg.AI.TopK)},
		{Key: "ai.confidence_threshold", Value: fmt.Sprintf("%f", cfg.AI.ConfidenceThreshold)},
		{Key: "ai.max_history_messages", Value: fmt.Sprintf("%d", cfg.AI.MaxHistoryMessages)},
		// Search
		{Key: "search.exa_api_key", Value: maskEnvKey(cfg.Search.Exa.APIKey)},
		{Key: "search.tavily_api_key", Value: maskEnvKey(cfg.Search.Tavily.APIKey)},
		{Key: "search.firecrawl_api_key", Value: maskEnvKey(cfg.Search.Firecrawl.APIKey)},
		// Upload
		{Key: "kb.max_upload_size", Value: fmt.Sprintf("%d", cfg.Knowledge.MaxUploadSizeKB)},
		// Rerank
		{Key: "rerank.enabled", Value: boolStr(cfg.Rerank.Enabled)},
	}
}

// boolStr 将 bool 转字符串。
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// maskEnvKey 脱敏 API key(仅显示前 8 字符)。
func maskEnvKey(k string) string {
	if len(k) <= 8 {
		return "***"
	}
	return k[:8] + "..."
}

// UpdateEnvConfig 更新 .env 中指定 key 的值,返回新 AppConfig(调用方触发热重建)。
func UpdateEnvConfig(key, value string) (*AppConfig, error) {
	envFile := findEnvFile()
	if envFile == "" {
		return nil, fmt.Errorf(".env 文件未找到")
	}

	// 读取现有 .env 行,替换或追加目标 key
	lines, err := readEnvLines(envFile)
	if err != nil {
		return nil, fmt.Errorf("读取 .env 失败: %w", err)
	}

	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}

	if err := writeEnvLines(envFile, lines); err != nil {
		return nil, fmt.Errorf("写入 .env 失败: %w", err)
	}

	// 重新加载配置
	return Load("")
}

// findEnvFile 查找 .env 文件路径(优先 ../.env)。
func findEnvFile() string {
	candidates := []string{"../.env", ".env", "../../.env"}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// readEnvLines 读取 .env 文件所有行。
func readEnvLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

// writeEnvLines 原子写入 .env 文件。
func writeEnvLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0644)
}

// resolveDataPaths 从 data_root 派生 storage/memory 路径。
// storage.local.base_dir / memory.storage_root 为空时，用 {data_root}/storage、{data_root}/memory 填充。
func (c *AppConfig) resolveDataPaths() {
	if c.DataRoot == "" {
		c.DataRoot = ".cognik"
	}
	if c.Storage.Local.BaseDir == "" {
		c.Storage.Local.BaseDir = filepath.Join(c.DataRoot, "storage")
	}
	if c.Memory.StorageRoot == "" {
		c.Memory.StorageRoot = filepath.Join(c.DataRoot, "memory")
	}
}

// bindEnvs 显式绑定环境变量到配置 key。
func bindEnvs(v *viper.Viper) {
	// AppName
	v.BindEnv("app_name", "COGNIK_APP_NAME")
	// DataRoot（数据根目录，storage/memory/logs/agent.db 派生自此）
	v.BindEnv("data_root", "COGNIK_DATA_ROOT")

	// Server
	v.BindEnv("server.port", "COGNIK_SERVER_PORT")
	v.BindEnv("server.mode", "COGNIK_SERVER_MODE")
	v.BindEnv("server.read_timeout", "COGNIK_SERVER_READ_TIMEOUT")
	v.BindEnv("server.write_timeout", "COGNIK_SERVER_WRITE_TIMEOUT")
	v.BindEnv("server.idle_timeout", "COGNIK_SERVER_IDLE_TIMEOUT")

	// Database
	v.BindEnv("database.host", "COGNIK_DATABASE_HOST")
	v.BindEnv("database.port", "COGNIK_DATABASE_PORT")
	v.BindEnv("database.user", "COGNIK_DATABASE_USER")
	v.BindEnv("database.password", "COGNIK_DATABASE_PASSWORD")
	v.BindEnv("database.dbname", "COGNIK_DATABASE_DBNAME")
	v.BindEnv("database.sslmode", "COGNIK_DATABASE_SSLMODE")
	v.BindEnv("database.max_open_conns", "COGNIK_DATABASE_MAX_OPEN_CONNS")
	v.BindEnv("database.max_idle_conns", "COGNIK_DATABASE_MAX_IDLE_CONNS")
	v.BindEnv("database.conn_max_lifetime", "COGNIK_DATABASE_CONN_MAX_LIFETIME")

	// JWT
	v.BindEnv("jwt.secret", "COGNIK_JWT_SECRET")
	v.BindEnv("jwt.access_expire", "COGNIK_JWT_ACCESS_EXPIRE")
	v.BindEnv("jwt.refresh_expire", "COGNIK_JWT_REFRESH_EXPIRE")

	// Storage
	v.BindEnv("storage.driver", "COGNIK_STORAGE_DRIVER")
	v.BindEnv("storage.local.base_dir", "COGNIK_STORAGE_LOCAL_BASE_DIR")
	v.BindEnv("storage.minio.endpoint", "COGNIK_MINIO_ENDPOINT")
	v.BindEnv("storage.minio.access_key", "COGNIK_MINIO_ACCESS_KEY")
	v.BindEnv("storage.minio.secret_key", "COGNIK_MINIO_SECRET_KEY")
	v.BindEnv("storage.minio.use_ssl", "COGNIK_MINIO_USE_SSL")

	// LLM
	v.BindEnv("llm.base_url", "COGNIK_LLM_BASE_URL")
	v.BindEnv("llm.api_key", "COGNIK_LLM_API_KEY")
	v.BindEnv("llm.model", "COGNIK_LLM_MODEL")
	v.BindEnv("llm.max_tokens", "COGNIK_LLM_MAX_TOKENS")
	v.BindEnv("llm.timeout", "COGNIK_LLM_TIMEOUT")

	// Embedding
	v.BindEnv("embedding.base_url", "COGNIK_EMBEDDING_BASE_URL")
	v.BindEnv("embedding.api_key", "COGNIK_EMBEDDING_API_KEY")
	v.BindEnv("embedding.model", "COGNIK_EMBEDDING_MODEL")
	v.BindEnv("embedding.dimension", "COGNIK_EMBEDDING_DIMENSION")
	v.BindEnv("embedding.timeout", "COGNIK_EMBEDDING_TIMEOUT")

	// AI
	v.BindEnv("ai.chunk_size", "COGNIK_AI_CHUNK_SIZE")
	v.BindEnv("ai.chunk_overlap", "COGNIK_AI_CHUNK_OVERLAP")
	v.BindEnv("ai.rag_enabled", "COGNIK_AI_RAG_ENABLED")
	v.BindEnv("ai.top_k", "COGNIK_AI_TOP_K")
	v.BindEnv("ai.confidence_threshold", "COGNIK_AI_CONFIDENCE_THRESHOLD")
	v.BindEnv("ai.max_history_messages", "COGNIK_AI_MAX_HISTORY_MESSAGES")

	// Search（深度搜索工具链，降级链模式）
	v.BindEnv("search.exa.api_key", "COGNIK_SEARCH_EXA_API_KEY")
	v.BindEnv("search.tavily.api_key", "COGNIK_SEARCH_TAVILY_API_KEY")
	v.BindEnv("search.firecrawl.api_key", "COGNIK_SEARCH_FIRECRAWL_API_KEY")
	v.BindEnv("search.max_results", "COGNIK_SEARCH_MAX_RESULTS")
	v.BindEnv("search.timeout", "COGNIK_SEARCH_TIMEOUT")

	// CORS
	v.BindEnv("cors.allow_origins", "COGNIK_CORS_ALLOW_ORIGINS")

	// Rerank
	v.BindEnv("rerank.enabled", "COGNIK_RERANK_ENABLED")
	v.BindEnv("rerank.python_path", "COGNIK_RERANK_PYTHON_PATH")
	v.BindEnv("rerank.script_path", "COGNIK_RERANK_SCRIPT_PATH")

	// Parser
	v.BindEnv("parser.engine", "COGNIK_PARSER_ENGINE")
	v.BindEnv("parser.mineru.api_key", "MINERU_API_KEY")
	v.BindEnv("parser.mineru.endpoint", "COGNIK_MINERU_ENDPOINT")
	v.BindEnv("parser.mineru.timeout", "COGNIK_MINERU_TIMEOUT")
	v.BindEnv("parser.python.path", "COGNIK_PARSER_PYTHON_PATH")

	// Knowledge
	v.BindEnv("kb.max_upload_size", "COGNIK_KB_MAX_UPLOAD_SIZE")

	// Memory（记忆系统，文件式存储 + 上下文压缩阈值）
	v.BindEnv("memory.storage_root", "COGNIK_MEMORY_STORAGE_ROOT")
	v.BindEnv("memory.memory_max_lines", "COGNIK_MEMORY_MAX_LINES")
	v.BindEnv("memory.compress_dedup", "COGNIK_MEMORY_COMPRESS_DEDUP")
	v.BindEnv("memory.compress_compact", "COGNIK_MEMORY_COMPRESS_COMPACT")
	v.BindEnv("memory.ingest_poll_interval", "COGNIK_MEMORY_INGEST_POLL_INTERVAL")
}

// Validate 校验配置合法性，在 Load 完成后自动调用。
func (c *AppConfig) Validate() error {
	if c.Server.Mode != "debug" && c.Server.Mode != "release" {
		return fmt.Errorf("server.mode 必须为 debug 或 release，当前值: %q", c.Server.Mode)
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1-65535 范围内，当前值: %d", c.Server.Port)
	}

	if c.Server.Mode == "release" && c.JWT.Secret == "" {
		return fmt.Errorf("release 模式下 jwt.secret 不能为空")
	}

	// duration 零值可能由 env 裸数字格式导致（需 "2h" 而非 3600）
	if c.JWT.AccessExpire == 0 {
		return fmt.Errorf("jwt.access_expire 为零值，可能是 env 格式错误（需字符串如 \"2h\"，而非裸数字 3600）")
	}
	if c.JWT.RefreshExpire == 0 {
		return fmt.Errorf("jwt.refresh_expire 为零值，可能是 env 格式错误（需字符串如 \"168h\"）")
	}

	if c.Knowledge.MaxUploadSizeKB <= 0 {
		return fmt.Errorf("kb.max_upload_size 必须大于 0，当前值: %d", c.Knowledge.MaxUploadSizeKB)
	}

	return nil
}

// setDefaults 设置配置默认值，与 config.yaml 保持一致。
func setDefaults(v *viper.Viper) {
	// AppName
	v.SetDefault("app_name", "Cognik")
	// DataRoot（数据根目录；server/ 运行时 .cognik = 项目根目录 .cognik/）
	v.SetDefault("data_root", ".cognik")

	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "300s")
	v.SetDefault("server.idle_timeout", "60s")

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "cognik")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "cognik")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 0) // 0 = 不限，由 PostgreSQL max_connections 控制
	v.SetDefault("database.max_idle_conns", 0) // 0 = 不限
	v.SetDefault("database.conn_max_lifetime", "5m")

	// JWT
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.access_expire", "2h")
	v.SetDefault("jwt.refresh_expire", "168h")

	// Storage（路径由 data_root 派生，见 resolveDataPaths）
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.local.base_dir", "") // 空 → {data_root}/storage
	v.SetDefault("storage.minio.endpoint", "localhost:9000")
	v.SetDefault("storage.minio.access_key", "minioadmin")
	v.SetDefault("storage.minio.secret_key", "minioadmin")
	v.SetDefault("storage.minio.use_ssl", false)
	v.SetDefault("storage.buckets.documents", "cognik-documents")

	// LLM
	v.SetDefault("llm.base_url", "http://llama-cpp:8080/v1")
	v.SetDefault("llm.api_key", "")
	v.SetDefault("llm.model", "qwen3-4b")
	v.SetDefault("llm.max_tokens", 8192)
	v.SetDefault("llm.timeout", "300s")

	// Embedding
	v.SetDefault("embedding.base_url", "")
	v.SetDefault("embedding.api_key", "")
	v.SetDefault("embedding.model", "") // 无默认值，必须通过环境变量或 LLM 配置指定
	v.SetDefault("embedding.dimension", 1536)
	v.SetDefault("embedding.timeout", "60s")

	// AI
	v.SetDefault("ai.chunk_size", 500)
	v.SetDefault("ai.chunk_overlap", 100)
	v.SetDefault("ai.rag_enabled", true)
	v.SetDefault("ai.top_k", 5)
	v.SetDefault("ai.confidence_threshold", 0.6)
	v.SetDefault("ai.max_history_messages", 10)

	// Search（深度搜索工具链，降级链模式）
	v.SetDefault("search.max_results", 5)
	v.SetDefault("search.timeout", "10s")

	// Rerank
	v.SetDefault("rerank.enabled", true)
	v.SetDefault("rerank.python_path", "python")
	v.SetDefault("rerank.script_path", "rerank_server.py")

	// CORS
	v.SetDefault("cors.allow_origins", "http://localhost:5173,http://localhost:3000")

	// Parser
	v.SetDefault("parser.engine", "mineru")
	v.SetDefault("parser.mineru.api_key", "")
	v.SetDefault("parser.mineru.endpoint", "https://mineru.net/api/v4")
	v.SetDefault("parser.mineru.timeout", "180s")
	v.SetDefault("parser.python.path", "python3")

	// Knowledge
	v.SetDefault("kb.max_upload_size", 51200)

	// Memory（路径由 data_root 派生，见 resolveDataPaths）
	v.SetDefault("memory.storage_root", "") // 空 → {data_root}/memory
	v.SetDefault("memory.memory_max_lines", 200)
	v.SetDefault("memory.compress_dedup", 0.70)
	v.SetDefault("memory.compress_compact", 0.85)
	v.SetDefault("memory.ingest_poll_interval", "5s")
}
