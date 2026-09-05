// web_search.go：网络搜索工具（SyncTool）。
//
// 调用 SearchChain 降级链（Exa→Tavily→DuckDuckGo），Agent 不感知后端差异。
// 降级在 SearchChain 内部完成，首个成功则返回。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cognik/internal/agent"
	"cognik/internal/infra/adapter"

	"cognik/internal/agent/llm"
)

// WebSearchTool 网络搜索工具（实现 agent.SyncTool）。
type WebSearchTool struct {
	chain   *adapter.SearchChain
	timeout time.Duration
}

// NewWebSearchTool 创建网络搜索工具。
func NewWebSearchTool(chain *adapter.SearchChain, timeout time.Duration) *WebSearchTool {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &WebSearchTool{chain: chain, timeout: timeout}
}

// Info 返回工具元信息。
func (t *WebSearchTool) Info() *llm.ToolInfo {
	return &llm.ToolInfo{
		Name: "web_search",
		Desc: `Search the web for current information. Returns title + url list.
- Snippets are leads, NOT evidence — always follow with web_fetch to confirm before citing.
- Use short keyword queries (max 3 queries). For multi-source synthesis, dispatch_subagent with deep_research.
- Do NOT cite search snippets without fetching the source page.`,
		ParamsOneOf: llm.NewParamsOneOfByParams(map[string]*llm.ParameterInfo{
			"query": {
				Type:     llm.String,
				Desc:     "Short keyword query (<1500 chars)",
				Required: true,
			},
			"max_results": {
				Type: llm.Integer,
				Desc: "Max results (default 5)",
			},
		}),
	}
}

// webSearchParams web_search 工具参数。
type webSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// Call 执行网络搜索，返回 title + url 列表。
func (t *WebSearchTool) Call(ctx context.Context, args string, emit agent.EventSink) (string, error) {
	var params webSearchParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	searchCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	results, err := t.chain.Search(searchCtx, params.Query, maxResults)
	if err != nil {
		return "", fmt.Errorf("搜索失败: %w", err)
	}
	if len(results) == 0 {
		return "无搜索结果", nil
	}

	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[%d] %s\n    %s\n", i+1, r.Title, r.URL)
	}
	return strings.TrimSpace(sb.String()), nil
}
