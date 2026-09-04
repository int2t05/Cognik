// 搜索后端抽象与降级链。
//
// SearchChain 按优先级尝试后端，首个成功则返回：
// Exa（首选，语义搜索）→ Tavily（降级，Agent 优化型）→ DuckDuckGo（本地兜底，零配置）。
// DuckDuckGo 无需 API Key，解析 html.duckduckgo.com HTML。
package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebSearchResult 搜索结果项（统一格式，屏蔽各后端差异）。
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Engine  string `json:"engine,omitempty"` // 来源引擎/后端
}

// SearchClient 搜索后端抽象（Exa / Tavily / DuckDuckGo 实现同一接口）。
type SearchClient interface {
	Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error)
	Name() string
}

// SearchChain 搜索降级链：按优先级尝试后端，首个成功则返回。
type SearchChain struct {
	backends []SearchClient
}

// NewSearchChain 创建降级链（backends 顺序即优先级，末尾应为本地兜底）。
func NewSearchChain(backends []SearchClient) *SearchChain {
	return &SearchChain{backends: backends}
}

// Search 按降级链尝试搜索，首个成功则返回；全部失败返回最后一个错误。
func (c *SearchChain) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if len(c.backends) == 0 {
		return nil, fmt.Errorf("搜索降级链为空，无可用后端")
	}
	var lastErr error
	for _, backend := range c.backends {
		results, err := backend.Search(ctx, query, maxResults)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", backend.Name(), err)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("所有搜索后端不可用，最后错误: %w", lastErr)
	}
	return nil, fmt.Errorf("所有搜索后端返回空结果")
}

// DuckDuckGo HTML 结果页正则（结构稳定，无需 HTML 解析器依赖）。
var (
	ddgResultRe = regexp.MustCompile(`(?s)<div class="result[^"]*">.*?</div>`)
	ddgTitleRe  = regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	ddgTagRe    = regexp.MustCompile(`<[^>]+>`)
)

// DuckDuckGoClient 零配置搜索兜底（无需 API Key，解析 html.duckduckgo.com HTML）。
// 参考 DeepResearchAgent 的 DDGSSearch。
type DuckDuckGoClient struct {
	httpClient *http.Client
}

// NewDuckDuckGoClient 创建 DuckDuckGo 客户端（本地兜底，始终在降级链末尾）。
func NewDuckDuckGoClient() *DuckDuckGoClient {
	return &DuckDuckGoClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Name 返回后端名。
func (c *DuckDuckGoClient) Name() string { return "duckduckgo" }

// Search 通过 DuckDuckGo HTML 接口搜索（无 JSON API，正则解析 HTML 结果页）。
func (c *DuckDuckGoClient) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 DuckDuckGo 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "OpsMind (AI Ops Assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DuckDuckGo 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB 上限防 OOM
	if err != nil {
		return nil, fmt.Errorf("读取 DuckDuckGo 响应失败: %w", err)
	}
	html := string(body)

	var results []WebSearchResult
	for _, block := range ddgResultRe.FindAllString(html, maxResults) {
		var title, href string
		if m := ddgTitleRe.FindStringSubmatch(block); len(m) >= 3 {
			href = m[1]
			title = decodeEntities(strings.TrimSpace(ddgTagRe.ReplaceAllString(m[2], "")))
		}
		// DuckDuckGo 重定向链接：/l/?uddg=<编码URL>
		if u, err := url.Parse(href); err == nil && u.Path == "/l/" {
			href = u.Query().Get("uddg")
		}
		if title != "" && href != "" {
			results = append(results, WebSearchResult{Title: title, URL: href, Engine: "duckduckgo"})
		}
	}
	return results, nil
}

// decodeEntities 解码 HTML 实体（&amp; &lt; &gt; &quot; &#39;）。
func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", `"`, "&#39;", "'", "&#x27;", "'",
		"&nbsp;", " ",
	)
	return r.Replace(s)
}
