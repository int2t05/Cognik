// Package adapter 提供外部服务的适配层。
//
// embedding_client.go 定义 EmbeddingClient 接口和 OpenAI-compatible HTTP 实现。
// 所有 Embedding 调用必须通过此适配层，禁止直接 HTTP 调用。
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// 接口定义
// =============================================================================

// EmbeddingClient 定义 Embedding 生成接口（OpenAI-compatible 协议）。
type EmbeddingClient interface {
	CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

// =============================================================================
// 请求/响应类型
// =============================================================================

// EmbeddingRequest embedding 请求。
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse embedding 响应。
type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimension  int         `json:"dimension"`
	TokensUsed int         `json:"tokens_used"`
}

// =============================================================================
// 实现
// =============================================================================

// OpenAIEmbeddingClient 对接 OpenAI-compatible /v1/embeddings。
//
// DashScope "兼容模式" 不完全兼容：缺少 encoding_format 时拒绝数组 input。
// 通过 isDashScope 标记自动附加 "encoding_format":"float"。
type OpenAIEmbeddingClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
	maxRetries   int
	isDashScope  bool
}

// NewOpenAIEmbeddingClient 创建客户端实例。
//
// defaultModel 用于 EmbeddingRequest.Model 为空时的回退模型名称。
func NewOpenAIEmbeddingClient(baseURL, apiKey, defaultModel string, timeout time.Duration) *OpenAIEmbeddingClient {
	return &OpenAIEmbeddingClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: defaultModel,
		httpClient:   &http.Client{Timeout: timeout},
		maxRetries:   defaultMaxRetries,
		isDashScope:  strings.Contains(baseURL, "dashscope"),
	}
}

// dashScopeEmbeddingsRequest DashScope 需要 encoding_format 字段才能接受数组 input。
type dashScopeEmbeddingsRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
}

// embeddingDataItem 单个 embedding 结果。
type embeddingDataItem struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// embeddingAPIResponse 通用 embedding API 响应（OpenAI 和 DashScope 兼容模式共用）。
type embeddingAPIResponse struct {
	Data  []embeddingDataItem `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// CreateEmbeddings 调用 /v1/embeddings 生成向量。
func (c *OpenAIEmbeddingClient) CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	jsonBody, err := c.marshalRequest(req)
	if err != nil {
		return nil, err
	}

	respBody, err := c.doRequest(ctx, jsonBody)
	if err != nil {
		return nil, err
	}

	return c.parseResponse(respBody, len(req.Input))
}

// marshalRequest 序列化请求体，DashScope 自动附加 encoding_format。
func (c *OpenAIEmbeddingClient) marshalRequest(req EmbeddingRequest) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	if c.isDashScope {
		return json.Marshal(dashScopeEmbeddingsRequest{
			Model:          model,
			Input:          req.Input,
			EncodingFormat: "float",
		})
	}
	return json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{model, req.Input})
}

// doRequest 带重试的 POST 请求。
func (c *OpenAIEmbeddingClient) doRequest(ctx context.Context, jsonBody []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := doHTTPRequest(ctx, c.baseURL, c.apiKey, "/embeddings", jsonBody, c.httpClient)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("embedding 重试 %d 次后仍失败: %w", c.maxRetries, lastErr)
}

// parseResponse 解析 API 响应为统一格式。
func (c *OpenAIEmbeddingClient) parseResponse(respBody []byte, expected int) (*EmbeddingResponse, error) {
	var apiResp embeddingAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析 embedding 响应失败: %w", err)
	}

	embeddings := make([][]float32, expected)
	for _, d := range apiResp.Data {
		if d.Index < 0 || d.Index >= expected {
			return nil, fmt.Errorf("embedding index 越界: %d (期望 0-%d)", d.Index, expected-1)
		}
		if embeddings[d.Index] != nil {
			return nil, fmt.Errorf("embedding index 重复: %d", d.Index)
		}
		embeddings[d.Index] = d.Embedding
	}

	dim := 0
	if len(embeddings) > 0 && len(embeddings[0]) > 0 {
		dim = len(embeddings[0])
	}

	return &EmbeddingResponse{
		Embeddings: embeddings,
		Dimension:  dim,
		TokensUsed: apiResp.Usage.TotalTokens,
	}, nil
}
