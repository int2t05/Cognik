// Package adapter 提供外部服务的适配层。
//
// 共享 HTTP 辅助（doHTTPRequest/retryableError/重试常量）供 Embedding/VectorStore 客户端复用。
// LLM 调用走 Eino ChatModel（agent 域）；RAG 的 LLM 调用类型在 rag/llm_types.go。
package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// =============================================================================
// 共享 HTTP 辅助（供 Embedding / VectorStore 客户端复用）
// =============================================================================

const (
	defaultMaxRetries = 3
	retryBaseDelay    = 500 * time.Millisecond
)

// retryableError 可重试错误（HTTP 429/503）。
type retryableError struct {
	statusCode int
	body       string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, e.body)
}

// doHTTPRequest 发送 HTTP 请求并返回响应体（供 Embedding 客户端复用）。
// 返回 retryableError 以便复用方识别 429/503。
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
