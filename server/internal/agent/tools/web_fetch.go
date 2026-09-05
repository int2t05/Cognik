// web_fetch.go：网页提取工具（SyncTool）。
//
// 调用 FetchChain 降级链（Firecrawl API→本地 http.Get），URL → 干净 Markdown。
// 降级在 FetchChain 内部完成，含 SSRF 防护。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cognik/internal/agent"
	"cognik/internal/infra/adapter"

	"cognik/internal/agent/llm"
)

// WebFetchTool 网页提取工具（实现 agent.SyncTool）。
type WebFetchTool struct {
	chain    *adapter.FetchChain
	maxBytes int64
}

// NewWebFetchTool 创建网页提取工具。
func NewWebFetchTool(chain *adapter.FetchChain, maxBytes int64) *WebFetchTool {
	return &WebFetchTool{chain: chain, maxBytes: maxBytes}
}

// Info 返回工具元信息。
func (t *WebFetchTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "web_fetch",
		Desc: `Fetch a web page and return clean Markdown (with title).
- Use to verify web_search snippets before citing, or read documentation.
- Do NOT fetch authenticated/private URLs.`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"url": {
				Type:     llm.String,
				Desc:     "URL to fetch (http/https, public addresses only)",
				Required: true,
			},
		}),
	}
}

// webFetchParams web_fetch 工具参数。
type webFetchParams struct {
	URL string `json:"url"`
}

// Call 提取网页内容，返回 Markdown。
func (t *WebFetchTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params webFetchParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.URL) == "" {
		return "", fmt.Errorf("url is required")
	}

	markdown, meta, err := t.chain.Fetch(ctx, params.URL)
	if err != nil {
		return "", fmt.Errorf("提取失败: %w", err)
	}
	if markdown == "" {
		return "页面内容为空", nil
	}

	// 截断防 token 膨胀
	content := truncate(markdown, t.maxBytes)
	if meta.Title != "" {
		return fmt.Sprintf("# %s\n\n%s", meta.Title, content), nil
	}
	return content, nil
}
