// Tavily Agent 优化型搜索客户端（降级链第二）。
// API: https://api.tavily.com/search，认证方式 Bearer token。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TavilyClient Tavily 搜索客户端。
type TavilyClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewTavilyClient 创建 Tavily 客户端。
func NewTavilyClient(apiKey string) *TavilyClient {
	return &TavilyClient{
		apiKey:  apiKey,
		baseURL: "https://api.tavily.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 返回后端名。
func (c *TavilyClient) Name() string { return "tavily" }

type tavilyRequest struct {
	Query         string `json:"query"`
	MaxResults    int    `json:"max_results"`
}

type tavilyResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// Search POST /search { query, max_results, include_answer }。
func (c *TavilyClient) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	reqBody, _ := json.Marshal(tavilyRequest{
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建 Tavily 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Tavily 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily 返回 HTTP %d", resp.StatusCode)
	}

	var tavResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavResp); err != nil {
		return nil, fmt.Errorf("解析 Tavily 响应失败: %w", err)
	}

	var results []WebSearchResult
	for _, r := range tavResp.Results {
		results = append(results, WebSearchResult{
			Title: r.Title, URL: r.URL, Engine: "tavily",
		})
	}
	return results, nil
}
