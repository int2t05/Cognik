// Package config 负责加载配置（Viper 读取 config.yaml + 环境变量覆盖，前缀 OPSMIND）。
package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// AppConfig 是顶层配置结构体，包含所有子模块配置。
type AppConfig struct {
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
	Timeout   time.Duration `mapstructure:"timeout"` // LLM 调用超时，默认 60s
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
	Timeout   time.Duration `mapstructure:"timeout"` // Embedding 调用超时，默认 30s
}

// AIConfig AI 问答配置（ChunkSize/ChunkOverlap 供文档处理管道使用）。
type AIConfig struct {
	ChunkSize    int `mapstructure:"chunk_size"`    // 文本分块大小（字符数），默认 500
	ChunkOverlap int `mapstructure:"chunk_overlap"` // 分块重叠大小（字符数），默认 100
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}
	return &cfg, nil
}

// bindEnvs 显式绑定环境变量到配置 key。
func bindEnvs(v *viper.Viper) {
	// Server
	v.BindEnv("server.port", "OPSMIND_SERVER_PORT")
	v.BindEnv("server.mode", "OPSMIND_SERVER_MODE")
	v.BindEnv("server.read_timeout", "OPSMIND_SERVER_READ_TIMEOUT")
	v.BindEnv("server.write_timeout", "OPSMIND_SERVER_WRITE_TIMEOUT")
	v.BindEnv("server.idle_timeout", "OPSMIND_SERVER_IDLE_TIMEOUT")

	// Database
	v.BindEnv("database.host", "OPSMIND_DATABASE_HOST")
	v.BindEnv("database.port", "OPSMIND_DATABASE_PORT")
	v.BindEnv("database.user", "OPSMIND_DATABASE_USER")
	v.BindEnv("database.password", "OPSMIND_DATABASE_PASSWORD")
	v.BindEnv("database.dbname", "OPSMIND_DATABASE_DBNAME")
	v.BindEnv("database.sslmode", "OPSMIND_DATABASE_SSLMODE")
	v.BindEnv("database.max_open_conns", "OPSMIND_DATABASE_MAX_OPEN_CONNS")
	v.BindEnv("database.max_idle_conns", "OPSMIND_DATABASE_MAX_IDLE_CONNS")
	v.BindEnv("database.conn_max_lifetime", "OPSMIND_DATABASE_CONN_MAX_LIFETIME")

	// JWT
	v.BindEnv("jwt.secret", "OPSMIND_JWT_SECRET")
	v.BindEnv("jwt.access_expire", "OPSMIND_JWT_ACCESS_EXPIRE")
	v.BindEnv("jwt.refresh_expire", "OPSMIND_JWT_REFRESH_EXPIRE")

	// Storage
	v.BindEnv("storage.driver", "OPSMIND_STORAGE_DRIVER")
	v.BindEnv("storage.local.base_dir", "OPSMIND_STORAGE_LOCAL_BASE_DIR")
	v.BindEnv("storage.minio.endpoint", "OPSMIND_MINIO_ENDPOINT")
	v.BindEnv("storage.minio.access_key", "OPSMIND_MINIO_ACCESS_KEY")
	v.BindEnv("storage.minio.secret_key", "OPSMIND_MINIO_SECRET_KEY")
	v.BindEnv("storage.minio.use_ssl", "OPSMIND_MINIO_USE_SSL")

	// LLM
	v.BindEnv("llm.base_url", "OPSMIND_LLM_BASE_URL")
	v.BindEnv("llm.api_key", "OPSMIND_LLM_API_KEY")
	v.BindEnv("llm.model", "OPSMIND_LLM_MODEL")
	v.BindEnv("llm.max_tokens", "OPSMIND_LLM_MAX_TOKENS")
	v.BindEnv("llm.timeout", "OPSMIND_LLM_TIMEOUT")

	// Embedding
	v.BindEnv("embedding.base_url", "OPSMIND_EMBEDDING_BASE_URL")
	v.BindEnv("embedding.api_key", "OPSMIND_EMBEDDING_API_KEY")
	v.BindEnv("embedding.model", "OPSMIND_EMBEDDING_MODEL")
	v.BindEnv("embedding.dimension", "OPSMIND_EMBEDDING_DIMENSION")
	v.BindEnv("embedding.timeout", "OPSMIND_EMBEDDING_TIMEOUT")

	// AI
	v.BindEnv("ai.chunk_size", "OPSMIND_AI_CHUNK_SIZE")
	v.BindEnv("ai.chunk_overlap", "OPSMIND_AI_CHUNK_OVERLAP")

	// Search（深度搜索工具链，降级链模式）
	v.BindEnv("search.exa.api_key", "OPSMIND_SEARCH_EXA_API_KEY")
	v.BindEnv("search.tavily.api_key", "OPSMIND_SEARCH_TAVILY_API_KEY")
	v.BindEnv("search.firecrawl.api_key", "OPSMIND_SEARCH_FIRECRAWL_API_KEY")
	v.BindEnv("search.max_results", "OPSMIND_SEARCH_MAX_RESULTS")
	v.BindEnv("search.timeout", "OPSMIND_SEARCH_TIMEOUT")

	// CORS
	v.BindEnv("cors.allow_origins", "OPSMIND_CORS_ALLOW_ORIGINS")

	// Rerank
	v.BindEnv("rerank.enabled", "OPSMIND_RERANK_ENABLED")
	v.BindEnv("rerank.python_path", "OPSMIND_RERANK_PYTHON_PATH")
	v.BindEnv("rerank.script_path", "OPSMIND_RERANK_SCRIPT_PATH")

	// Parser
	v.BindEnv("parser.engine", "OPSMIND_PARSER_ENGINE")
	v.BindEnv("parser.mineru.api_key", "MINERU_API_KEY")
	v.BindEnv("parser.mineru.endpoint", "OPSMIND_MINERU_ENDPOINT")
	v.BindEnv("parser.mineru.timeout", "OPSMIND_MINERU_TIMEOUT")
	v.BindEnv("parser.python.path", "OPSMIND_PARSER_PYTHON_PATH")

	// Knowledge
	v.BindEnv("kb.max_upload_size", "OPSMIND_KB_MAX_UPLOAD_SIZE")
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
	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "300s")
	v.SetDefault("server.idle_timeout", "60s")

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "opsmind")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "opsmind")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 0) // 0 = 不限，由 PostgreSQL max_connections 控制
	v.SetDefault("database.max_idle_conns", 0) // 0 = 不限
	v.SetDefault("database.conn_max_lifetime", "5m")

	// JWT
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.access_expire", "2h")
	v.SetDefault("jwt.refresh_expire", "168h")

	// Storage
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.local.base_dir", "./data/storage")
	v.SetDefault("storage.minio.endpoint", "localhost:9000")
	v.SetDefault("storage.minio.access_key", "minioadmin")
	v.SetDefault("storage.minio.secret_key", "minioadmin")
	v.SetDefault("storage.minio.use_ssl", false)
	v.SetDefault("storage.buckets.documents", "opsmind-documents")

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
	v.SetDefault("embedding.dimension", 1024)
	v.SetDefault("embedding.timeout", "300s")

	// AI
	v.SetDefault("ai.chunk_size", 500)
	v.SetDefault("ai.chunk_overlap", 100)

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
}
