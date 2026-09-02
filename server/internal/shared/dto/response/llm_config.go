// Package response 定义 API 响应体结构。
package response

// LLMConfigResponse LLM 配置响应项。APIKey 在 Service 层脱敏。
type LLMConfigResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	ProviderType    int16  `json:"provider_type"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	LLMModel        string `json:"llm_model"`
	EmbeddingModel  string `json:"embedding_model"`
	MaxTokens       int    `json:"max_tokens"`
	VectorDimension int    `json:"vector_dimension"`
	IsDefault       bool   `json:"is_default"`
}
