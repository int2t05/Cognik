// 页面提取后端抽象与降级链。
//
// FetchChain 按优先级尝试后端，首个成功则返回：
// Firecrawl API（首选，JS 渲染 + 干净 Markdown）→ 本地 http.Get + html-to-markdown（兜底，零配置）。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FetchMeta 页面元数据。
type FetchMeta struct {
	Title       string `json:"title"`
}

// FetchClient 页面提取后端抽象。
type FetchClient interface {
	Fetch(ctx context.Context, url string) (markdown string, meta FetchMeta, err error)
	Name() string
}

// FetchChain 提取降级链：按优先级尝试后端，首个成功则返回。
type FetchChain struct {
	backends []FetchClient
}

// NewFetchChain 创建提取降级链（末尾应为本地兜底）。
func NewFetchChain(backends []FetchClient) *FetchChain {
	return &FetchChain{backends: backends}
}

// Fetch 按降级链尝试提取，首个成功则返回。SSRF 防护：拒绝内网地址。
func (c *FetchChain) Fetch(ctx context.Context, rawURL string) (string, FetchMeta, error) {
	if err := validateURL(rawURL); err != nil {
		return "", FetchMeta{}, fmt.Errorf("URL 安全检查失败: %w", err)
	}
	if len(c.backends) == 0 {
		return "", FetchMeta{}, fmt.Errorf("提取降级链为空，无可用后端")
	}
	var lastErr error
	for _, backend := range c.backends {
		md, meta, err := backend.Fetch(ctx, rawURL)
		if err == nil && md != "" {
			return md, meta, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", backend.Name(), err)
		}
	}
	if lastErr != nil {
		return "", FetchMeta{}, fmt.Errorf("所有提取后端不可用，最后错误: %w", lastErr)
	}
	return "", FetchMeta{}, fmt.Errorf("所有提取后端返回空内容")
}

// validateURL SSRF 防护：只允许 http/https + 公网地址（拒绝 localhost/内网/云元数据）。
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 解析失败: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("只允许 http/https 协议，当前: %s", u.Scheme)
	}
	host := u.Hostname()
	// 拒绝内网地址
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("拒绝内网地址: %s", host)
	}
	// 拒绝私有网段（10.x / 172.16-31.x / 192.168.x）
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
		return fmt.Errorf("拒绝私有网段: %s", host)
	}
	if strings.HasPrefix(host, "172.") {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			if octet, e := strconv.Atoi(parts[1]); e == nil && octet >= 16 && octet <= 31 {
				return fmt.Errorf("拒绝私有网段: %s", host)
			}
		}
	}
	// 拒绝云元数据端点
	if host == "169.254.169.254" {
		return fmt.Errorf("拒绝云元数据端点: %s", host)
	}
	return nil
}

// Firecrawl API 客户端（降级链首选，需 API Key）

// FirecrawlClient Firecrawl 页面提取客户端（URL → 干净 Markdown，JS 渲染）。
type FirecrawlClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewFirecrawlClient 创建 Firecrawl 客户端。
func NewFirecrawlClient(apiKey string) *FirecrawlClient {
	return &FirecrawlClient{
		apiKey:  apiKey,
		baseURL: "https://api.firecrawl.dev",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name 返回后端名。
func (c *FirecrawlClient) Name() string { return "firecrawl" }

type firecrawlRequest struct {
	URL     string   `json:"url"`
	Formats []string `json:"formats"`
}

type firecrawlResponse struct {
	Data struct {
		Markdown string `json:"markdown"`
		Metadata struct {
			Title       string `json:"title"`
		} `json:"metadata"`
	} `json:"data"`
}

// Fetch POST /v2/scrape { url, formats: ["markdown"] }。
func (c *FirecrawlClient) Fetch(ctx context.Context, targetURL string) (string, FetchMeta, error) {
	reqBody, _ := json.Marshal(firecrawlRequest{URL: targetURL, Formats: []string{"markdown"}})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/scrape", bytes.NewReader(reqBody))
	if err != nil {
		return "", FetchMeta{}, fmt.Errorf("创建 Firecrawl 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", FetchMeta{}, fmt.Errorf("Firecrawl 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", FetchMeta{}, fmt.Errorf("Firecrawl 返回 HTTP %d", resp.StatusCode)
	}

	var fcResp firecrawlResponse
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return "", FetchMeta{}, fmt.Errorf("解析 Firecrawl 响应失败: %w", err)
	}

	return fcResp.Data.Markdown, FetchMeta{
		Title:       fcResp.Data.Metadata.Title,
	}, nil
}

// 本地 http.Get 兜底客户端（零配置，无 JS 渲染）

var (
	htmlTitleRe  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	htmlTagRe    = regexp.MustCompile(`(?is)<(script|style|noscript|iframe)\b[^>]*>.*?</(script|style|noscript|iframe)>`)
	htmlAllTagRe = regexp.MustCompile(`<[^>]+>`)
	htmlSpaceRe  = regexp.MustCompile(`\s+`)
)

// LocalFetchClient 本地页面提取兜底（Go 标准库 net/http + 正则转 Markdown）。
// 无 JS 渲染，简单静态页面可用。不依赖任何外部服务。
type LocalFetchClient struct {
	httpClient *http.Client
}

// NewLocalFetchClient 创建本地提取客户端（始终在降级链末尾）。
func NewLocalFetchClient() *LocalFetchClient {
	return &LocalFetchClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 返回后端名。
func (c *LocalFetchClient) Name() string { return "local" }

// Fetch GET {url} → 读取 HTML → 去 script/style → 提取 title + 正文 → 粗转 Markdown。
func (c *LocalFetchClient) Fetch(ctx context.Context, targetURL string) (string, FetchMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", FetchMeta{}, fmt.Errorf("创建本地请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "OpsMind (AI Ops Assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", FetchMeta{}, fmt.Errorf("本地请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", FetchMeta{}, fmt.Errorf("本地请求返回 HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB 上限防 OOM
	if err != nil {
		return "", FetchMeta{}, fmt.Errorf("读取响应失败: %w", err)
	}
	html := string(body)

	// 提取元数据
	var meta FetchMeta
	if m := htmlTitleRe.FindStringSubmatch(html); len(m) >= 2 {
		meta.Title = strings.TrimSpace(decodeEntities(m[1]))
	}

	// 粗转 Markdown：去 script/style → 去 HTML 标签 → 压缩空白
	cleaned := htmlTagRe.ReplaceAllString(html, "")
	text := htmlAllTagRe.ReplaceAllString(cleaned, "")
	text = decodeEntities(text)
	text = htmlSpaceRe.ReplaceAllString(strings.TrimSpace(text), " ")

	if text == "" {
		return "", meta, fmt.Errorf("本地提取内容为空（可能是 JS 渲染页面）")
	}
	return text, meta, nil
}
