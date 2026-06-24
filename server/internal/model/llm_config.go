// Package model 定义 GORM 数据模型。
//
// llm_config.go 定义 LLM 配置模型（表 llm_configs），管理 LLM 和 Embedding 的连接参数。
//
// 设计决策：LLM 和 Embedding 各自拥有独立的 Base URL 和 API Key。
// 虽然本地 llama.cpp server 也兼容 OpenAI v1 协议，但以下场景需要独立配置：
//   - 使用 OpenAI 做 LLM 生成 + 本地部署 bge-m3 做 Embedding
//   - 使用 DeepSeek API 做 LLM 生成 + Moonshot API 做 Embedding
// EmbeddingBaseURL 为空时回退到 LLMBaseURL，EmbeddingAPIKey 为空时回退到 LLMAPIKey。
//
// 所有提供商统一使用 OpenAI-compatible v1 API（llama.cpp server 也兼容此协议）。
package model

import (
	"time"

	"opsmind/pkg/crypto"

	"gorm.io/gorm"
)

// LlmConfig LLM/Embedding 提供商配置。
type LlmConfig struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"type:varchar(128);not null" json:"name"`
	LLMBaseURL       string    `gorm:"type:varchar(512);default:'';column:llm_base_url" json:"llm_base_url"`
	LLMAPIKey        string    `gorm:"type:varchar(512);column:llm_api_key" json:"llm_api_key"`
	EmbeddingBaseURL string    `gorm:"type:varchar(512);column:embedding_base_url" json:"embedding_base_url"`
	EmbeddingAPIKey  string    `gorm:"type:varchar(512);column:embedding_api_key" json:"embedding_api_key"`
	LLMModel         string    `gorm:"type:varchar(128);not null;column:llm_model" json:"llm_model"`
	EmbeddingModel   string    `gorm:"type:varchar(128);not null;column:embedding_model" json:"embedding_model"`
	MaxTokens        int       `gorm:"not null;default:8192;column:max_tokens" json:"max_tokens"`
	VectorDimension  int       `gorm:"not null;default:1024;column:vector_dimension" json:"vector_dimension"`
	SystemPrompt     string    `gorm:"type:text;column:system_prompt" json:"system_prompt"` // 系统提示词，空时使用默认值
	IsDefault        bool      `gorm:"not null;default:false;column:is_default" json:"is_default"`
	CreatedAt        time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"not null" json:"updated_at"`
}

// BeforeSave GORM 钩子：保存前加密 API Key。
func (c *LlmConfig) BeforeSave(tx *gorm.DB) error {
	if c.LLMAPIKey != "" {
		enc, err := crypto.Encrypt(c.LLMAPIKey)
		if err != nil {
			return err
		}
		c.LLMAPIKey = enc
	}
	if c.EmbeddingAPIKey != "" {
		enc, err := crypto.Encrypt(c.EmbeddingAPIKey)
		if err != nil {
			return err
		}
		c.EmbeddingAPIKey = enc
	}
	return nil
}

// AfterFind GORM 钩子：查询后解密 API Key。
func (c *LlmConfig) AfterFind(tx *gorm.DB) error {
	if c.LLMAPIKey != "" {
		dec, err := crypto.Decrypt(c.LLMAPIKey)
		if err != nil {
			return err
		}
		c.LLMAPIKey = dec
	}
	if c.EmbeddingAPIKey != "" {
		dec, err := crypto.Decrypt(c.EmbeddingAPIKey)
		if err != nil {
			return err
		}
		c.EmbeddingAPIKey = dec
	}
	return nil
}

// TableName 指定表名。
func (LlmConfig) TableName() string { return "llm_configs" }

// GetEmbeddingBaseURL 返回 Embedding 服务地址，空时回退到 LLM BaseURL。
func (c *LlmConfig) GetEmbeddingBaseURL() string {
	if c.EmbeddingBaseURL != "" {
		return c.EmbeddingBaseURL
	}
	return c.LLMBaseURL
}

// GetEmbeddingAPIKey 返回 Embedding API Key，空时回退到 LLM API Key。
func (c *LlmConfig) GetEmbeddingAPIKey() string {
	if c.EmbeddingAPIKey != "" {
		return c.EmbeddingAPIKey
	}
	return c.LLMAPIKey
}
