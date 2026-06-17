// Package config 负责加载和管理 OpsMind 后端配置。
//
// 使用 Viper 读取 config.yaml，支持环境变量覆盖。
// 环境变量前缀为 OPSMIND，例如 OPSMIND_DATABASE_HOST 覆盖 database.host。
// 这样做的原因：Docker Compose 通过环境变量注入运行时配置，
// 本地开发使用 config.yaml 默认值，两者互不冲突。
package config

import (
	"time"

	"github.com/spf13/viper"
)

// AppConfig 是顶层配置结构体，包含所有子模块配置。
type AppConfig struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	MinIO     MinIOConfig     `mapstructure:"minio"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Rerank    RerankConfig    `mapstructure:"rerank"`
	AI        AIConfig        `mapstructure:"ai"`
	CORS      CORSConfig      `mapstructure:"cors"`
}

// CORSConfig 是跨域资源共享配置。
//
// AllowOrigins 为逗号分隔的允许来源列表（如 "http://localhost:5173,https://opsmind.example.com"）。
// 为空时默认使用 http://localhost:5173（本地开发环境）。
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

// LLMConfig 是大语言模型调用配置。
//
// 支持任意 OpenAI-compatible API（llama.cpp / OpenAI / DeepSeek 等）。
// BaseURL 指向 /v1 根路径（如 http://llama-cpp:8080/v1），
// APIKey 对 llama.cpp 本地部署可为空。
type LLMConfig struct {
	BaseURL   string        `mapstructure:"base_url"`
	APIKey    string        `mapstructure:"api_key"`
	Model     string        `mapstructure:"model"`
	MaxTokens int           `mapstructure:"max_tokens"`
	Timeout   time.Duration `mapstructure:"timeout"` // LLM 调用超时，默认 60s
}

// EmbeddingConfig 是文本向量化配置。
//
// RerankConfig 是 cross-encoder 重排序子进程配置。
//
// 使用 Python 子进程运行 sentence-transformers CrossEncoder，
// 通过 stdin/stdout JSON Lines 协议通信。
// Enabled 为 false 时 Pipeline 降级跳过重排序步骤。
type RerankConfig struct {
	Enabled    bool   `mapstructure:"enabled"`     // 是否启用 cross-encoder 重排序，默认 true
	PythonPath string `mapstructure:"python_path"` // Python 解释器路径，如 "python3"
	ScriptPath string `mapstructure:"script_path"` // rerank_server.py 绝对路径
}

// LLM 和 Embedding 可独立配置 Base URL 和 API Key，支持以下场景：
//   - OpenAI LLM + 本地 bge-m3 Embedding（无需 Embedding API Key）
//   - DeepSeek LLM + Moonshot Embedding（需要各自的 API Key）
//   - OpenAI LLM + DashScope Embedding（跨服务商，需要独立 API Key）
//
// BaseURL 为空时回退到 llm.base_url。
// APIKey 为空时回退到 llm.api_key。
type EmbeddingConfig struct {
	BaseURL   string        `mapstructure:"base_url"`
	APIKey    string        `mapstructure:"api_key"`
	Model     string        `mapstructure:"model"`
	Dimension int           `mapstructure:"dimension"`
	Timeout   time.Duration `mapstructure:"timeout"` // Embedding 调用超时，默认 30s
}

// AIConfig 是 AI 问答相关配置。
//
// RAG 管道步骤可通过 rag_* 开关单独控制，均默认启用。
// ConfidenceThreshold 低于此阈值的问答结果会引导用户提交申告（can_submit_ticket=true）。
type AIConfig struct {
	DefaultTopK         int     `mapstructure:"default_top_k"`
	ConfidenceThreshold float64 `mapstructure:"confidence_threshold"`
	MaxHistoryMessages  int     `mapstructure:"max_history_messages"`
	RAGQueryRewrite     bool    `mapstructure:"rag_query_rewrite"`
	RAGMultiRoute       bool    `mapstructure:"rag_multi_route"`
	RAGHybrid           bool    `mapstructure:"rag_hybrid"`
	RAGRerank           bool    `mapstructure:"rag_rerank"`
}

// Load 加载配置文件并应用环境变量覆盖。
//
// configPath 为空时使用默认路径 ./internal/config/config.yaml。
// 环境变量前缀为 OPSMIND，例如 OPSMIND_DATABASE_HOST 覆盖 database.host。
// 使用 BindEnv 显式绑定关键配置项，确保 Unmarshal 时能正确读取环境变量。
func Load(configPath string) (*AppConfig, error) {
	v := viper.New()

	// 设置默认值
	// TODO(config): 增加 Validate() 阶段统一校验配置合法性。
	// 例如 server.mode 只能是 debug/release、端口范围、TopK 范围、confidence_threshold 范围，
	// 以及 release 模式下 JWT/数据库/对象存储关键配置必须显式提供。
	setDefaults(v)

	// 读取配置文件
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./internal/config")
	}

	// 配置文件不存在时不报错（使用默认值和环境变量）
	// TODO(config): 非 ConfigFileNotFoundError 的 ReadInConfig 错误（如 YAML 格式错误）被静默丢弃，
	// 应用以默认值启动而不报错——生产环境可能以错误配置运行。应区分错误类型并至少 Warn。
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// 显式绑定环境变量，确保 Unmarshal 能正确覆盖嵌套字段。
	// 为什么用 BindEnv 而非 AutomaticEnv：AutomaticEnv 对嵌套 key 的
	// 环境变量映射不一致（需要 key 和 env 同名），BindEnv 更可控。
	bindEnvs(v)

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// TODO(config): 记录实际生效配置时应对 password/api_key/secret 做脱敏。
	// 后续如果添加配置诊断日志，避免把敏感值打到 stdout 或容器日志。
	//
	// TODO(config): time.Duration 字段的 BindEnv 映射需注意格式差异。
	// Viper 要求 time.Duration 为字符串形式（如 "2h"），裸数字 3600 会导致解析失败。
	// 应在 Validate() 中检测 duration 字段为零值并生成可操作的错误消息。
	return &cfg, nil
}

// bindEnvs 显式绑定环境变量到配置 key。
//
// 环境变量命名规则：OPSMIND_ + 字段路径（下划线分隔），
// 例如 database.host → OPSMIND_DATABASE_HOST。
func bindEnvs(v *viper.Viper) {
	// TODO(config): 这里的 BindEnv 调用应检查返回 error。
	// Viper 当前通常返回 nil，但忽略错误会掩盖未来 key 绑定失败或测试替身异常。
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

	// MinIO
	v.BindEnv("minio.endpoint", "OPSMIND_MINIO_ENDPOINT")
	v.BindEnv("minio.access_key", "OPSMIND_MINIO_ACCESS_KEY")
	v.BindEnv("minio.secret_key", "OPSMIND_MINIO_SECRET_KEY")
	v.BindEnv("minio.use_ssl", "OPSMIND_MINIO_USE_SSL")

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
	v.BindEnv("ai.default_top_k", "OPSMIND_AI_DEFAULT_TOP_K")
	v.BindEnv("ai.confidence_threshold", "OPSMIND_AI_CONFIDENCE_THRESHOLD")
	v.BindEnv("ai.max_history_messages", "OPSMIND_AI_MAX_HISTORY_MESSAGES")
	v.BindEnv("ai.rag_query_rewrite", "OPSMIND_AI_RAG_QUERY_REWRITE")
	v.BindEnv("ai.rag_multi_route", "OPSMIND_AI_RAG_MULTI_ROUTE")
	v.BindEnv("ai.rag_hybrid", "OPSMIND_AI_RAG_HYBRID")
	v.BindEnv("ai.rag_rerank", "OPSMIND_AI_RAG_RERANK")

	// CORS
	v.BindEnv("cors.allow_origins", "OPSMIND_CORS_ALLOW_ORIGINS")

	// Rerank（cross-encoder 子进程）
	v.BindEnv("rerank.enabled", "OPSMIND_RERANK_ENABLED")
	v.BindEnv("rerank.python_path", "OPSMIND_RERANK_PYTHON_PATH")
	v.BindEnv("rerank.script_path", "OPSMIND_RERANK_SCRIPT_PATH")
}

// setDefaults 设置配置默认值，与 config.yaml 保持一致。
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "60s")
	v.SetDefault("server.idle_timeout", "60s")

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "opsmind")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "opsmind")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "5m")

	// JWT
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.access_expire", "2h")
	v.SetDefault("jwt.refresh_expire", "168h")

	// MinIO
	v.SetDefault("minio.endpoint", "localhost:9000")
	v.SetDefault("minio.access_key", "minioadmin")
	v.SetDefault("minio.secret_key", "minioadmin")
	v.SetDefault("minio.use_ssl", false)

	// LLM
	v.SetDefault("llm.base_url", "http://llama-cpp:8080/v1")
	v.SetDefault("llm.api_key", "")
	v.SetDefault("llm.model", "qwen3-4b")
	v.SetDefault("llm.max_tokens", 8192)
	v.SetDefault("llm.timeout", "60s")

	// Embedding
	v.SetDefault("embedding.base_url", "")
	v.SetDefault("embedding.api_key", "")
	v.SetDefault("embedding.model", "bge-m3")
	v.SetDefault("embedding.dimension", 1024)
	v.SetDefault("embedding.timeout", "30s")

	// AI
	v.SetDefault("ai.default_top_k", 5)
	v.SetDefault("ai.confidence_threshold", 0.6)
	v.SetDefault("ai.max_history_messages", 10)
	v.SetDefault("ai.rag_query_rewrite", true)
	v.SetDefault("ai.rag_multi_route", true)
	v.SetDefault("ai.rag_hybrid", true)
	v.SetDefault("ai.rag_rerank", true)

	// Rerank（cross-encoder 子进程）
	v.SetDefault("rerank.enabled", true)
	v.SetDefault("rerank.python_path", "python3")
	v.SetDefault("rerank.script_path", "rerank_server.py")

	// CORS
	v.SetDefault("cors.allow_origins", "http://localhost:5173")
}
