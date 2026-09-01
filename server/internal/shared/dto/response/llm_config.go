// Package response 定义 API 响应体结构。
//
// llm_config.go 定义 LLM 配置相关响应体。
package response

// LLMConfigResponse LLM 配置列表响应项。
//
// 注意：APIKey 字段在 Service 层的 LlmConfigResponse.MarshalJSON() 中自动脱敏，
// 此处保留原始类型以支持详情接口返回完整密钥。
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
