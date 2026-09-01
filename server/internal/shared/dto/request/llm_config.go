// Package request 定义 API 请求体结构。
//
// llm_config.go 定义 LLM 配置相关请求体。
package request

// CreateLLMConfigRequest 创建 LLM 配置请求。
type CreateLLMConfigRequest struct {
	Name             string `json:"name" binding:"required"`
	LLMBaseURL       string `json:"llm_base_url" binding:"required"`
	LLMAPIKey        string `json:"llm_api_key"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingAPIKey  string `json:"embedding_api_key"`
	LLMModel         string `json:"llm_model" binding:"required"`
	EmbeddingModel   string `json:"embedding_model" binding:"required"`
	SystemPrompt     string `json:"system_prompt"`
	MaxTokens        int    `json:"max_tokens"`
	VectorDimension  int    `json:"vector_dimension"`
	IsDefault        bool   `json:"is_default"`
}

// UpdateLLMConfigRequest 更新 LLM 配置请求。
type UpdateLLMConfigRequest struct {
	Name             string `json:"name" binding:"required"`
	LLMBaseURL       string `json:"llm_base_url" binding:"required"`
	LLMAPIKey        string `json:"llm_api_key"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingAPIKey  string `json:"embedding_api_key"`
	LLMModel         string `json:"llm_model" binding:"required"`
	EmbeddingModel   string `json:"embedding_model" binding:"required"`
	SystemPrompt     string `json:"system_prompt"`
	MaxTokens        int    `json:"max_tokens"`
	VectorDimension  int    `json:"vector_dimension"`
	IsDefault        bool   `json:"is_default"`
}
