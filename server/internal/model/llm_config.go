// Package model 定义 GORM 数据模型。
//
// llm_config.go 定义 LLM 配置模型（表 llm_configs），管理 LLM 和 Embedding 的连接参数。
//
// 设计决策：LLM 和 Embedding 各自拥有独立的 Base URL。
// 虽然它们通常指向同一服务（如 llama.cpp server），但以下场景需要拆分：
//   - 使用 OpenAI 做 LLM 生成 + 本地部署 bge-m3 做 Embedding
//   - 使用 DeepSeek API 做 LLM 生成 + Moonshot API 做 Embedding
// EmbeddingBaseURL 为空时回退到 BaseURL（保持向后兼容）。
//
// 提供商类型仅支持两种：
//   1 = llama.cpp（本地部署，无需 API Key）
//   2 = OpenAI-compatible API（OpenAI / DeepSeek / Moonshot 等）
package model

import (
	"time"

	"opsmind/pkg/crypto"
)

// LlmConfig LLM/Embedding 提供商配置。
type LlmConfig struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"type:varchar(128);not null" json:"name"`
	ProviderType     int16     `gorm:"not null;default:1;column:provider_type" json:"provider_type"` // 1=llama.cpp, 2=OpenAI-compatible
	BaseURL          string    `gorm:"type:varchar(512);not null;column:base_url" json:"base_url"`
	EmbeddingBaseURL string    `gorm:"type:varchar(512);column:embedding_base_url" json:"embedding_base_url"`
	APIKey           string    `gorm:"type:varchar(512);column:api_key" json:"api_key"`
	LLMModel         string    `gorm:"type:varchar(128);not null;column:llm_model" json:"llm_model"`
	EmbeddingModel   string    `gorm:"type:varchar(128);not null;column:embedding_model" json:"embedding_model"`
	MaxTokens        int       `gorm:"not null;default:8192;column:max_tokens" json:"max_tokens"`
	VectorDimension  int       `gorm:"not null;default:1024;column:vector_dimension" json:"vector_dimension"`
	SystemPrompt     string    `gorm:"type:text;column:system_prompt" json:"system_prompt"` // 系统提示词，空时使用默认值
	IsDefault        bool      `gorm:"not null;default:false;column:is_default" json:"is_default"`
	CreatedAt        time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"not null" json:"updated_at"`
}

// BeforeSave GORM 钩子：保存前加密 APIKey。
func (c *LlmConfig) BeforeSave() error {
	if c.APIKey != "" {
		enc, err := crypto.Encrypt(c.APIKey)
		if err != nil {
			return err
		}
		c.APIKey = enc
	}
	return nil
}

// AfterFind GORM 钩子：查询后解密 APIKey。
func (c *LlmConfig) AfterFind() error {
	if c.APIKey != "" {
		dec, err := crypto.Decrypt(c.APIKey)
		if err != nil {
			return err
		}
		c.APIKey = dec
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
	return c.BaseURL
}
