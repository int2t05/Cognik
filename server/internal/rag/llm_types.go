// Package rag 实现自建 RAG 检索引擎。
// llm_types.go：RAG 引擎专用的 LLM 调用接口与类型（查询改写/多路检索用）。
//
// Agent 聊天走 Eino ChatModel，不依赖此接口。此处供 rag/ 的 query_rewrite/multi_route 使用。
package rag

import "context"

// LLMClient 定义 LLM 调用接口（OpenAI-compatible 协议）。
// 供 RAG 查询改写/多路检索使用；Agent 聊天走 Eino ChatModel 不依赖此接口。
type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// ChatRequest 对话请求。
type ChatRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
	EnableThinking bool          `json:"-"` // 流式回答是否启用思考模式（同步调用始终关闭）
}

// ChatMessage 对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse 同步对话响应。
type ChatResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
}

// StreamChunk SSE 流式的单个 token 块。
type StreamChunk struct {
	Content      string `json:"content"`
	Reasoning    string `json:"-"` // 思考/推理内容（仅思考模式开启时非空）
	FinishReason string `json:"finish_reason"`
	Error        error  `json:"-"`
}
