// Package model 定义 GORM 数据模型。
package model

import (
	"time"

	"cognik/internal/shared/pkg/crypto"

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
	VectorDimension  int       `gorm:"not null;default:1536;column:vector_dimension" json:"vector_dimension"`
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
