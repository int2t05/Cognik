// Package adapter_test 测试 EmbeddingClient 适配器。
//
// 优先使用真实 Embedding 服务（从环境变量或默认地址读取），不可用时回退到 mock HTTP server。
// 错误/空输入类测试始终使用 mock，因为真实服务无法稳定复现异常。
//
// 环境变量：
//   - OPSMIND_EMBEDDING_BASE_URL: Embedding API 地址（空则回退到 LLM_BASE_URL）
//   - OPSMIND_EMBEDDING_API_KEY:  API 密钥（空则回退到 LLM_API_KEY）
//   - OPSMIND_EMBEDDING_MODEL:    模型名称（默认 bge-m3）
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

// embeddingServiceInfo 描述可用的 Embedding 服务。
type embeddingServiceInfo struct {
	BaseURL string
	APIKey  string
	Model   string
	IsReal  bool
}

// probeEmbedding 探测真实 Embedding 服务的可用性。
//
// 优先使用 EMBEDDING 专用变量，为空时回退到 LLM 变量。
// 通过 GET /models 端点验证服务可达性。
func probeEmbedding(t *testing.T) embeddingServiceInfo {
	t.Helper()

	baseURL := os.Getenv("OPSMIND_EMBEDDING_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("OPSMIND_LLM_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	apiKey := os.Getenv("OPSMIND_EMBEDDING_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPSMIND_LLM_API_KEY")
	}
	model := os.Getenv("OPSMIND_EMBEDDING_MODEL")
	if model == "" {
		model = "bge-m3"
	}

	// 探测真实服务：GET /models
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		t.Logf("Embedding 真实服务不可用（创建请求失败: %v），回退 mock", err)
		return embeddingServiceInfo{IsReal: false}
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Embedding 真实服务不可用（%v），回退 mock", err)
		return embeddingServiceInfo{IsReal: false}
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Logf("Embedding 真实服务不可用（HTTP %d），回退 mock", resp.StatusCode)
		return embeddingServiceInfo{IsReal: false}
	}

	t.Logf("Embedding 真实服务可用: %s (模型: %s)", baseURL, model)
	return embeddingServiceInfo{BaseURL: baseURL, APIKey: apiKey, Model: model, IsReal: true}
}

// newEmbeddingClient 根据探测结果创建 Embedding 客户端（真实或 mock）。
func newEmbeddingClient(t *testing.T, info embeddingServiceInfo) (*adapter.OpenAIEmbeddingClient, *httptest.Server) {
	t.Helper()

	if info.IsReal {
		return adapter.NewOpenAIEmbeddingClient(info.BaseURL, info.APIKey, info.Model, 30*time.Second), nil
	}

	// 回退到 mock server
	server := mockEmbeddingServer(func(w http.ResponseWriter, r *http.Request) {
		var req adapter.EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var dim int
		switch {
		case strings.Contains(req.Model, "bge-m3"):
			dim = 1024
		case strings.Contains(req.Model, "text-embedding-3-small"):
			dim = 1536
		default:
			dim = 768
		}

		embeddings := make([][]float32, len(req.Input))
		for i := range embeddings {
			emb := make([]float32, dim)
			emb[0] = float32(i) * 0.1
			emb[dim-1] = 0.9
			embeddings[i] = emb
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   buildEmbeddingData(embeddings),
			"model":  req.Model,
			"usage":  map[string]int{"total_tokens": len(req.Input) * 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return adapter.NewOpenAIEmbeddingClient(server.URL, "test-key", "", 10*time.Second), server
}

// mockEmbeddingServer 创建模拟 OpenAI-compatible embeddings API 的 HTTP 测试服务器。
func mockEmbeddingServer(handler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/embeddings", handler)
	return httptest.NewServer(mux)
}

// =============================================================================
// 测试用例
// =============================================================================

func TestCreateEmbeddings_Single(t *testing.T) {
	info := probeEmbedding(t)
	client, mockServer := newEmbeddingClient(t, info)
	if mockServer != nil {
		defer mockServer.Close()
	}

	model := info.Model
	if !info.IsReal {
		model = "bge-m3"
	}

	resp, err := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: model,
		Input: []string{"如何重置 VPN 密码？"},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings 失败: %v", err)
	}

	if len(resp.Embeddings) != 1 {
		t.Fatalf("期望 1 个 embedding, 实际 %d", len(resp.Embeddings))
	}
	if len(resp.Embeddings[0]) == 0 {
		t.Error("向量维度不应为 0")
	}
	if resp.Dimension == 0 {
		t.Error("Dimension 不应为 0")
	}

	if info.IsReal {
		t.Logf("真实 Embedding 服务: 维度=%d", resp.Dimension)
	} else {
		if len(resp.Embeddings[0]) != 1024 {
			t.Errorf("mock 模式期望 1024 维, 实际 %d", len(resp.Embeddings[0]))
		}
	}
}

func TestCreateEmbeddings_Batch(t *testing.T) {
	info := probeEmbedding(t)
	client, mockServer := newEmbeddingClient(t, info)
	if mockServer != nil {
		defer mockServer.Close()
	}

	model := info.Model
	if !info.IsReal {
		model = "bge-m3"
	}

	resp, err := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: model,
		Input: []string{"文本A", "文本B", "文本C"},
	})
	if err != nil {
		t.Fatalf("批量 CreateEmbeddings 失败: %v", err)
	}

	if len(resp.Embeddings) != 3 {
		t.Fatalf("期望 3 个 embedding, 实际 %d", len(resp.Embeddings))
	}
	if resp.Dimension == 0 {
		t.Error("Dimension 不应为 0")
	}

	t.Logf("批量 Embedding: %d 条输入 → %d 维向量", len(resp.Embeddings), resp.Dimension)
}

func TestCreateEmbeddings_DimensionValidation(t *testing.T) {
	// 优先使用真实服务测试两个模型
	info := probeEmbedding(t)

	if info.IsReal {
		client := adapter.NewOpenAIEmbeddingClient(info.BaseURL, info.APIKey, info.Model, 30*time.Second)

		resp, err := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
			Model: info.Model,
			Input: []string{"测试维度"},
		})
		if err != nil {
			t.Fatalf("真实 Embedding 调用失败: %v", err)
		}
		if resp.Dimension == 0 {
			t.Error("真实服务返回 Dimension 不应为 0")
		}
		t.Logf("真实服务 %s → 维度=%d", info.Model, resp.Dimension)
		return
	}

	// 回退到 mock：验证不同模型的维度
	server := mockEmbeddingServer(func(w http.ResponseWriter, r *http.Request) {
		var req adapter.EmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		var dim int
		switch {
		case req.Model == "bge-m3":
			dim = 1024
		case req.Model == "text-embedding-3-small":
			dim = 1536
		default:
			dim = 768
		}

		emb := make([]float32, dim)
		resp := map[string]interface{}{
			"object": "list",
			"data":   buildEmbeddingData([][]float32{emb}),
			"model":  req.Model,
			"usage":  map[string]int{"total_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := adapter.NewOpenAIEmbeddingClient(server.URL, "test-key", "", 10*time.Second)

	// bge-m3 → 1024 维
	resp1, err1 := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: "bge-m3",
		Input: []string{"test"},
	})
	if err1 != nil {
		t.Fatalf("bge-m3 CreateEmbeddings 失败: %v", err1)
	}
	if resp1.Dimension != 1024 {
		t.Errorf("bge-m3 维期望 1024, 实际 %d", resp1.Dimension)
	}

	// text-embedding-3-small → 1536 维
	resp2, err2 := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []string{"test"},
	})
	if err2 != nil {
		t.Fatalf("text-embedding-3-small CreateEmbeddings 失败: %v", err2)
	}
	if resp2.Dimension != 1536 {
		t.Errorf("text-embedding-3-small 维期望 1536, 实际 %d", resp2.Dimension)
	}
}

func TestCreateEmbeddings_HTTPError(t *testing.T) {
	// 必须用 mock：真实服务不会稳定返回 429
	server := mockEmbeddingServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	})
	defer server.Close()

	client := adapter.NewOpenAIEmbeddingClient(server.URL, "test-key", "", 10*time.Second)

	_, err := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: "bge-m3",
		Input: []string{"test"},
	})
	if err == nil {
		t.Error("HTTP 429 应返回错误, 实际 nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("错误信息应包含 429, 实际: %v", err)
	}
}

func TestCreateEmbeddings_EmptyInput(t *testing.T) {
	// 必须用 mock：验证客户端对空输入的处理，无需真实 Embedding
	server := mockEmbeddingServer(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data":   []interface{}{},
			"model":  "bge-m3",
			"usage":  map[string]int{"total_tokens": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := adapter.NewOpenAIEmbeddingClient(server.URL, "test-key", "", 10*time.Second)

	resp, err := client.CreateEmbeddings(context.Background(), adapter.EmbeddingRequest{
		Model: "bge-m3",
		Input: []string{},
	})
	if err != nil {
		t.Fatalf("空输入应成功, 实际错误: %v", err)
	}
	if resp.Dimension != 0 {
		t.Errorf("空输入 Dimension 期望 0, 实际 %d", resp.Dimension)
	}
}

// =============================================================================
// 辅助函数
// =============================================================================

func buildEmbeddingData(embeddings [][]float32) []map[string]interface{} {
	data := make([]map[string]interface{}, len(embeddings))
	for i, emb := range embeddings {
		data[i] = map[string]interface{}{
			"object":    "embedding",
			"index":     i,
			"embedding": emb,
		}
	}
	return data
}
