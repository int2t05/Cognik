// Package adapter 提供外部服务的适配层。
//
// llm_client.go 定义 LLMClient 接口和 OpenAI-compatible HTTP 实现。
// 所有 LLM 调用（文本生成、流式输出）必须通过此适配层，禁止直接 HTTP 调用。
package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LLMClient 定义 LLM 调用接口（OpenAI-compatible 协议）。
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
	TokensUsed   int    `json:"tokens_used"`
}

// StreamChunk SSE 流式的单个 token 块。
type StreamChunk struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
	Error        error  `json:"-"`
}

const (
	defaultMaxRetries = 3
	retryBaseDelay    = 500 * time.Millisecond
)

// OpenAIClient 实现 LLMClient。
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

// NewOpenAIClient 创建 OpenAIClient 实例。baseURL 必须非空。
func NewOpenAIClient(baseURL, apiKey string, timeout time.Duration) (*OpenAIClient, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL 不能为空")
	}
	return &OpenAIClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: defaultMaxRetries,
	}, nil
}

// =============================================================================
// ChatCompletion — 同步调用
// =============================================================================

type openAICompletionRequest struct {
	Model              string         `json:"model"`
	Messages           []ChatMessage  `json:"messages"`
	MaxTokens          int            `json:"max_tokens,omitempty"`
	Temperature        float64        `json:"temperature,omitempty"`
	Stream             bool           `json:"stream"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type openAICompletionResponse struct {
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *OpenAIClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model 不能为空")
	}
	start := time.Now()
	body := openAICompletionRequest{
		Model: req.Model, Messages: req.Messages, MaxTokens: req.MaxTokens,
		Temperature: req.Temperature, Stream: false,
	}
	// 同步调用默认禁用思考（管道步骤需要干净输出）；
	// 复杂分析任务（如反馈分析）可显式设置 EnableThinking=true 开启思考
	if !req.EnableThinking {
		body.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	}

	respBody, err := c.doRequest(ctx, "/chat/completions", body)
	if err != nil {
		slog.Error("LLM 同步调用失败", "model", req.Model, "latency_ms", time.Since(start).Milliseconds(), "error", err)
		return nil, err
	}

	var apiResp openAICompletionResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回空 choices")
	}

	// 思考模式已通过 chat_template_kwargs 禁用，content 即为实际回答。
	// reasoning_content 作为兜底：部分旧版 llama.cpp 可能将回答放在此处。
	content := apiResp.Choices[0].Message.Content
	if content == "" {
		content = apiResp.Choices[0].Message.ReasoningContent
	}
	slog.Info("LLM 同步调用完成", "model", req.Model, "tokens", apiResp.Usage.TotalTokens,
		"latency_ms", time.Since(start).Milliseconds(), "content_len", len(content))
	return &ChatResponse{
		Content:      content,
		FinishReason: apiResp.Choices[0].FinishReason,
		TokensUsed:   apiResp.Usage.TotalTokens,
	}, nil
}

// =============================================================================
// ChatCompletionStream — 流式调用（含 429/503 重试）
// =============================================================================

type openAIStreamChunk struct {
	Choices []struct {
		Index        int     `json:"index"`
		Delta        struct{ Content string } `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (c *OpenAIClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	body := openAICompletionRequest{
		Model: req.Model, Messages: req.Messages, MaxTokens: req.MaxTokens,
		Temperature: req.Temperature, Stream: true,
	}
	// 流式调用根据请求决定是否启用思考；默认关闭
	if req.EnableThinking {
		body.ChatTemplateKwargs = nil // 不传 = 使用模型默认（启用思考）
	} else {
		body.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化流式请求失败: %w", err)
	}

	slog.Info("LLM 流式调用开始", "model", req.Model, "enable_thinking", req.EnableThinking)

	// 流式请求支持 429/503 重试
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			slog.Warn("LLM 流式请求重试中", "attempt", attempt, "delay_ms", delay.Milliseconds())
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
		c.setHeaders(httpReq)
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, lastErr = c.httpClient.Do(httpReq)
		if lastErr != nil {
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			lastErr = &retryableError{statusCode: resp.StatusCode, body: "stream retry"}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			slog.Error("LLM 流式 API 返回错误", "model", req.Model, "status", resp.StatusCode)
			return nil, fmt.Errorf("LLM API 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		break
	}
	if resp == nil {
		slog.Error("LLM 流式请求失败", "model", req.Model, "error", lastErr)
		return nil, fmt.Errorf("流式请求重试 %d 次后仍失败: %w", c.maxRetries, lastErr)
	}

	ch := make(chan StreamChunk, 100)
	go c.readSSEStream(ctx, resp, ch)
	return ch, nil
}

func (c *OpenAIClient) readSSEStream(ctx context.Context, resp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB 上限，防止大行截断
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			if !sendChunk(ctx, ch, StreamChunk{Error: fmt.Errorf("解析 SSE chunk 失败: %w", err)}) {
				return
			}
			continue
		}

		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			var finishReason string
			if chunk.Choices[0].FinishReason != nil {
				finishReason = *chunk.Choices[0].FinishReason
			}
			if content != "" || finishReason != "" {
				if !sendChunk(ctx, ch, StreamChunk{Content: content, FinishReason: finishReason}) {
					return
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		sendChunk(ctx, ch, StreamChunk{Error: fmt.Errorf("读取 SSE 流失败: %w", err)})
	}
}

func sendChunk(ctx context.Context, ch chan<- StreamChunk, chunk StreamChunk) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- chunk:
		return true
	}
}

// =============================================================================
// HTTP 请求辅助
// =============================================================================

func (c *OpenAIClient) doRequest(ctx context.Context, path string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			slog.Warn("LLM HTTP 请求重试中", "attempt", attempt, "delay_ms", delay.Milliseconds(), "error", lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		respBody, err := c.tryRequest(ctx, path, jsonBody)
		if err == nil {
			return respBody, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", c.maxRetries, lastErr)
}

func (c *OpenAIClient) tryRequest(ctx context.Context, path string, jsonBody []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, &retryableError{statusCode: resp.StatusCode, body: string(respBody)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type retryableError struct {
	statusCode int
	body       string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, e.body)
}

func isRetryable(err error) bool {
	_, ok := err.(*retryableError)
	return ok
}

// doHTTPRequest 发送 HTTP 请求并返回响应体（供 Embedding 客户端复用）。
// 返回 retryableError 使 EmbeddingClient 的 isRetryable 能正确识别 429/503。
func doHTTPRequest(ctx context.Context, baseURL, apiKey, path string, jsonBody []byte, client *http.Client) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", baseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, &retryableError{statusCode: resp.StatusCode, body: string(respBody)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// MustNewOpenAIClient 创建客户端，失败时 panic（仅用于测试初始化）。
func MustNewOpenAIClient(baseURL, apiKey string, timeout time.Duration) *OpenAIClient {
	c, err := NewOpenAIClient(baseURL, apiKey, timeout)
	if err != nil {
		panic("MustNewOpenAIClient: " + err.Error())
	}
	return c
}

func (c *OpenAIClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
