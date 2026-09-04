// Exa 语义搜索客户端（降级链首选）。
// API: https://api.exa.ai/search，认证方式 x-api-key header。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExaClient Exa 语义搜索客户端。
type ExaClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewExaClient 创建 Exa 客户端。
func NewExaClient(apiKey string) *ExaClient {
	return &ExaClient{
		apiKey:  apiKey,
		baseURL: "https://api.exa.ai",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name 返回后端名。
func (c *ExaClient) Name() string { return "exa" }

// exaRequest Exa 搜索请求体。
type exaRequest struct {
	Query       string          `json:"query"`
	Type        string          `json:"type"`        // auto | instant | fast | deep
	NumResults  int             `json:"numResults"`
	Contents    exaContents     `json:"contents"`
}

type exaContents struct {
	Highlights bool `json:"highlights"` // 启用 highlights 返回高亮片段
}

// exaResponse Exa 搜索响应。
type exaResponse struct {
	Results []struct {
		Title      string   `json:"title"`
		URL        string   `json:"url"`
		Highlights []string `json:"highlights"`
	} `json:"results"`
}

// Search POST /search { query, type: "auto", contents: { highlights: true }, numResults }。
func (c *ExaClient) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	reqBody, _ := json.Marshal(exaRequest{
		Query:      query,
		Type:       "auto",
		NumResults: maxResults,
		Contents:   exaContents{Highlights: true},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建 Exa 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Exa 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Exa 返回 HTTP %d", resp.StatusCode)
	}

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, fmt.Errorf("解析 Exa 响应失败: %w", err)
	}

	var results []WebSearchResult
	for _, r := range exaResp.Results {
		results = append(results, WebSearchResult{
			Title: r.Title, URL: r.URL, Engine: "exa",
		})
	}
	return results, nil
}
