// Package adapter_test 测试 LLMClient 适配器。
//
// 优先使用真实 LLM 服务（从环境变量或默认地址读取），不可用时回退到 mock HTTP server。
// 错误/超时/取消类测试始终使用 mock，因为真实服务无法稳定复现这些异常。
//
// 环境变量：
//   - OPSMIND_LLM_BASE_URL: LLM API 地址（默认 http://localhost:8080/v1）
//   - OPSMIND_LLM_API_KEY:  API 密钥（可选，llama.cpp 本地部署不需要）
//   - OPSMIND_LLM_MODEL:    模型名称（默认 qwen3-4b）
package adapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"opsmind/internal/adapter"
)

// =============================================================================
// 测试基础设施 — 真实服务优先，不可用则回退 mock
// =============================================================================

// llmServiceInfo 描述可用的 LLM 服务。
type llmServiceInfo struct {
	BaseURL string
	APIKey  string
	Model   string
	IsReal  bool // true=真实服务, false=mock
}

// probeLLM 探测真实 LLM 服务的可用性。
//
// 从环境变量读取配置，默认指向本地 llama.cpp 服务。
// 通过 GET /models 端点验证服务可达性，不可达时返回 mock 配置。
func probeLLM(t *testing.T) llmServiceInfo {
	t.Helper()

	baseURL := os.Getenv("OPSMIND_LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	apiKey := os.Getenv("OPSMIND_LLM_API_KEY")
	model := os.Getenv("OPSMIND_LLM_MODEL")
	if model == "" {
		model = "qwen3-4b"
	}

	// 探测真实服务：GET /models
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		t.Logf("LLM 真实服务不可用（创建请求失败: %v），回退 mock", err)
		return llmServiceInfo{IsReal: false}
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("LLM 真实服务不可用（%v），回退 mock", err)
		return llmServiceInfo{IsReal: false}
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Logf("LLM 真实服务不可用（HTTP %d），回退 mock", resp.StatusCode)
		return llmServiceInfo{IsReal: false}
	}

	t.Logf("LLM 真实服务可用: %s (模型: %s)", baseURL, model)
	return llmServiceInfo{BaseURL: baseURL, APIKey: apiKey, Model: model, IsReal: true}
}

// newLLMClient 根据探测结果创建 LLM 客户端（真实或 mock）。
// 返回 client 和可选的 mockServer（需要 defer close）。
func newLLMClient(t *testing.T, info llmServiceInfo) (*adapter.OpenAIClient, *httptest.Server) {
	t.Helper()

	if info.IsReal {
		client, err := adapter.NewOpenAIClient(info.BaseURL, info.APIKey, 30*time.Second)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	return client, nil
	}

	// 回退到 mock server
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		var req adapter.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "账号冻结的处理步骤如下：1. 确认账号归属；2. 联系管理员。",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{"total_tokens": 45},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return adapter.MustNewOpenAIClient(server.URL, "test-key", 10*time.Second), server
}

// mockOpenAIServer 创建模拟 OpenAI-compatible API 的 HTTP 测试服务器。
func mockOpenAIServer(chatHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	if chatHandler != nil {
		mux.HandleFunc("/chat/completions", chatHandler)
	}
	return httptest.NewServer(mux)
}

// =============================================================================
// 同步 ChatCompletion 测试
// =============================================================================

func TestChatCompletion_Success(t *testing.T) {
	info := probeLLM(t)
	client, mockServer := newLLMClient(t, info)
	if mockServer != nil {
		defer mockServer.Close()
	}

	model := info.Model
	if !info.IsReal {
		model = "qwen3-4b" // mock 默认模型名
	}

	resp, err := client.ChatCompletion(context.Background(), adapter.ChatRequest{
		Model: model,
		Messages: []adapter.ChatMessage{
			{Role: "user", Content: "账号冻结怎么处理？"},
		},
		MaxTokens:   8192,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("ChatCompletion 失败: %v", err)
	}

	if resp.Content == "" {
		t.Error("响应内容不应为空")
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason 期望 stop, 实际 %s", resp.FinishReason)
	}

	if !info.IsReal {
		// mock 模式验证具体内容
		if !strings.Contains(resp.Content, "账号冻结") {
			t.Errorf("响应内容应包含「账号冻结」, 实际 %q", resp.Content)
		}
	} else {
		t.Logf("真实 LLM 响应: %s (finish_reason=%s)", resp.Content, resp.FinishReason)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	// 必须用 mock：真实服务不会稳定返回 401
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	})
	defer server.Close()

	client := adapter.MustNewOpenAIClient(server.URL, "bad-key", 10*time.Second)

	_, err := client.ChatCompletion(context.Background(), adapter.ChatRequest{
		Model:    "qwen3-4b",
		Messages: []adapter.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Error("HTTP 401 应返回错误, 实际 nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含 HTTP 状态码 401, 实际: %v", err)
	}
}

func TestChatCompletion_Timeout(t *testing.T) {
	// 必须用 mock：模拟超时需要服务端不响应
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	client := adapter.MustNewOpenAIClient(server.URL, "test-key", 500*time.Millisecond)

	_, err := client.ChatCompletion(context.Background(), adapter.ChatRequest{
		Model:    "qwen3-4b",
		Messages: []adapter.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Error("超时应返回错误, 实际 nil")
	}
}

func TestChatCompletion_ContextCancellation(t *testing.T) {
	// 必须用 mock：模拟取消需要精确控制时序
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	client := adapter.MustNewOpenAIClient(server.URL, "test-key", 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := client.ChatCompletion(ctx, adapter.ChatRequest{
		Model:    "qwen3-4b",
		Messages: []adapter.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Error("context 取消应返回错误, 实际 nil")
	}
}

// =============================================================================
// 流式 ChatCompletionStream 测试
// =============================================================================

func TestChatCompletionStream_Success(t *testing.T) {
	info := probeLLM(t)

	if info.IsReal {
		// 真实 LLM 流式调用
		client := adapter.MustNewOpenAIClient(info.BaseURL, info.APIKey, 60*time.Second)

		ch, err := client.ChatCompletionStream(context.Background(), adapter.ChatRequest{
			Model: info.Model,
			Messages: []adapter.ChatMessage{
				{Role: "user", Content: "用一句话介绍什么是运维。"},
			},
			MaxTokens: 512,
		})
		if err != nil {
			t.Fatalf("ChatCompletionStream 失败: %v", err)
		}

		var fullContent strings.Builder
		for chunk := range ch {
			if chunk.Error != nil {
				t.Fatalf("流式 chunk 错误: %v", chunk.Error)
			}
			fullContent.WriteString(chunk.Content)
		}

		if fullContent.Len() == 0 {
			t.Error("真实 LLM 流式输出不应为空")
		}
		t.Logf("真实 LLM 流式输出: %s", fullContent.String())
		return
	}

	// 回退到 mock SSE 流式
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		if stream, ok := req["stream"].(bool); !ok || !stream {
			t.Error("流式请求应设置 stream: true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter 不支持 Flusher")
		}

		tokens := []string{"账", "号", "冻", "结", "的", "处", "理", "步", "骤"}
		for _, tok := range tokens {
			chunk := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"delta": map[string]string{"content": tok}, "index": 0},
				},
			}
			data, _ := json.Marshal(chunk)
			w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	})
	defer server.Close()

	client := adapter.MustNewOpenAIClient(server.URL, "test-key", 10*time.Second)

	ch, err := client.ChatCompletionStream(context.Background(), adapter.ChatRequest{
		Model:    "qwen3-4b",
		Messages: []adapter.ChatMessage{{Role: "user", Content: "账号冻结怎么处理？"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var fullContent strings.Builder
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("流式 chunk 错误: %v", chunk.Error)
		}
		fullContent.WriteString(chunk.Content)
	}

	result := fullContent.String()
	if result != "账号冻结的处理步骤" {
		t.Errorf("流式拼接结果期望 %q, 实际 %q", "账号冻结的处理步骤", result)
	}
}

func TestChatCompletionStream_ClientDisconnect(t *testing.T) {
	// 必须用 mock：测试客户端断连行为需要精确控制
	server := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		chunk := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"delta": map[string]string{"content": "A"}, "index": 0},
			},
		}
		data, _ := json.Marshal(chunk)
		w.Write([]byte("data: " + string(data) + "\n\n"))
		flusher.Flush()

		time.Sleep(5 * time.Second)
	})
	defer server.Close()

	client := adapter.MustNewOpenAIClient(server.URL, "test-key", 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := client.ChatCompletionStream(ctx, adapter.ChatRequest{
		Model:    "qwen3-4b",
		Messages: []adapter.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream 初始化失败: %v", err)
	}

	// 读取第一个 token
	firstChunk := <-ch
	if firstChunk.Error != nil {
		t.Fatalf("第一个 chunk 错误: %v", firstChunk.Error)
	}

	// 立即取消 context（模拟客户端断开）
	cancel()

	// channel 应该在 context 取消后关闭
	for range ch {
		// drain channel
	}

	t.Log("context 取消后 channel 正确关闭")
}
