// Package mineru 实现 MinerU 云端高精度结构化提取引擎。
//
// MinerU (https://mineru.net) 还原 LaTeX 公式 / 表格结构 / 版面布局，
// 补足本地纯 Go 库在复杂版面和公式场景下的短板。
// 原生支持 PDF / DOCX / PPTX / XLSX / 图片，云端按格式自动路由。
//
// 调用流程（Precise API 本地文件上传）：
//  1. 读全部字节到内存（LimitReader 200MB）
//  2. POST /file-urls/batch（Bearer token）→ 获取 batch_id + 预签名 upload_url
//  3. PUT 上传文件到 OSS → 自动触发解析
//  4. GET 轮询 /extract-results/batch/{batch_id}（每 3s，最长 timeout）
//  5. 下载 full_zip_url → 解压 ZIP → 提取 *.md + images/*
//  6. 返回 ParseResult{Markdown, Images}
//
// 失败 / 超时 / 无 key 由 parser 包统一降级到本地解析。
package mineru

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cognik/internal/infra/config"
	"cognik/internal/parser/local"
)

// fileSizeLimit MinerU 单文件大小上限（200MB，与服务端一致）。
const fileSizeLimit = 200 * 1024 * 1024

// pollInterval 轮询 batch 结果的间隔。
const pollInterval = 3 * time.Second

// Engine MinerU 云端结构化提取引擎。
//
// endpoint 形如 https://mineru.net/api/v4（含 /api/v4 前缀）。
// apiKey 为空时引擎不可用，由调用方决定是否降级。
type Engine struct {
	apiKey     string
	endpoint   string
	timeout    time.Duration
	httpClient *http.Client
}

// NewEngine 根据 MinerUConfig 创建引擎实例。
//
// apiKey 为空时返回 nil 引擎（调用方据此降级到本地解析）。
func NewEngine(cfg config.MinerUConfig) *Engine {
	if cfg.APIKey == "" {
		return nil
	}
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://mineru.net/api/v4"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	return &Engine{
		apiKey:   cfg.APIKey,
		endpoint: endpoint,
		timeout:  timeout,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // 单次 HTTP 请求超时（轮询总超时由 timeout 控制）
		},
	}
}

// Parse 上传文件到 MinerU 云端解析，返回 Markdown + 图片。
//
// fileName 需含扩展名（如 report.pdf），MinerU 据此路由解析器。
func (e *Engine) Parse(reader io.Reader, fileName string) (*local.ParseResult, error) {
	// 1. 读全部字节到内存（LimitReader 防止 OOM）
	data, err := io.ReadAll(io.LimitReader(reader, fileSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("MinerU 读取文件失败: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("MinerU 输入文件为空")
	}
	if len(data) >= fileSizeLimit {
		return nil, fmt.Errorf("MinerU 文件超过 %dMB 上限", fileSizeLimit/(1024*1024))
	}

	// 2. 获取预签名上传 URL
	batchID, uploadURL, err := e.getUploadURL(fileName)
	if err != nil {
		return nil, err
	}

	// 3. PUT 上传到 OSS（触发自动解析）
	if err := e.uploadFile(data, uploadURL); err != nil {
		return nil, err
	}

	// 4. 轮询解析结果
	zipURL, err := e.pollBatch(batchID)
	if err != nil {
		return nil, err
	}

	// 5. 下载 ZIP 并提取 Markdown + 图片
	return e.downloadAndExtract(zipURL)
}

// getUploadURL POST /file-urls/batch 获取预签名上传 URL + batch_id。
func (e *Engine) getUploadURL(fileName string) (batchID, uploadURL string, err error) {
	body := map[string]any{
		"files":          []map[string]string{{"name": fileName}},
		"model_version":  "vlm",
		"enable_formula": true,
		"enable_table":   true,
		"language":       "ch",
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, e.endpoint+"/file-urls/batch", bytes.NewReader(payload))
	if err != nil {
		return "", "", fmt.Errorf("MinerU 创建上传请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("MinerU 请求上传 URL 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("MinerU 认证失败(401): API key 无效")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", "", fmt.Errorf("MinerU 限频(429): 请求过多")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("MinerU 获取上传 URL 失败: HTTP %d — %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BatchID  string   `json:"batch_id"`
			FileURLs []string `json:"file_urls"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("MinerU 解析上传 URL 响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", "", fmt.Errorf("MinerU API code=%d msg=%s", result.Code, result.Msg)
	}
	if result.Data.BatchID == "" || len(result.Data.FileURLs) == 0 {
		return "", "", fmt.Errorf("MinerU 响应缺少 batch_id 或 file_urls")
	}
	return result.Data.BatchID, result.Data.FileURLs[0], nil
}

// uploadFile PUT 上传文件字节到 OSS 预签名 URL。
func (e *Engine) uploadFile(data []byte, uploadURL string) error {
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("MinerU 创建上传 OSS 请求失败: %w", err)
	}
	// OSS 预签名 URL 直接接收二进制流，不设 Content-Type
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MinerU 上传 OSS 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("MinerU 上传 OSS 失败: HTTP %d — %s", resp.StatusCode, truncate(string(body), 200))
	}
	return nil
}

// pollBatch GET 轮询 /extract-results/batch/{batch_id}，返回 full_zip_url。
//
// 每隔 pollInterval 轮询一次，总等待不超过 e.timeout。
func (e *Engine) pollBatch(batchID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/extract-results/batch/%s", e.endpoint, batchID)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		zipURL, err := e.checkBatch(url)
		if err != nil {
			return "", err
		}
		if zipURL != "" {
			return zipURL, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("MinerU 轮询超时(%s, batch_id=%s)", e.timeout, batchID)
		case <-ticker.C:
		}
	}
}

// checkBatch 单次查询 batch 状态，返回 full_zip_url（未完成时为空）。
func (e *Engine) checkBatch(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("MinerU 创建轮询请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MinerU 轮询请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MinerU 轮询失败: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ExtractResult []struct {
				State      string `json:"state"`
				FullZipURL string `json:"full_zip_url"`
				ErrMsg     string `json:"err_msg"`
			} `json:"extract_result"`
			FullZipURL string `json:"full_zip_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("MinerU 解析轮询响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("MinerU API code=%d msg=%s", result.Code, result.Msg)
	}

	// 优先从 extract_result 取结果
	for _, item := range result.Data.ExtractResult {
		state := strings.ToLower(item.State)
		if state == "failed" || state == "error" {
			return "", fmt.Errorf("MinerU 解析失败: %s", item.ErrMsg)
		}
		if state == "done" || item.FullZipURL != "" {
			if item.FullZipURL != "" {
				return item.FullZipURL, nil
			}
		}
	}
	// 兼容顶层 full_zip_url
	if result.Data.FullZipURL != "" {
		return result.Data.FullZipURL, nil
	}
	return "", nil
}

// downloadAndExtract 下载结果 ZIP 并提取 Markdown + 图片。
func (e *Engine) downloadAndExtract(zipURL string) (*local.ParseResult, error) {
	resp, err := e.httpClient.Get(zipURL)
	if err != nil {
		return nil, fmt.Errorf("MinerU 下载结果 ZIP 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MinerU 下载结果 ZIP 失败: HTTP %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(io.LimitReader(resp.Body, fileSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("MinerU 读取结果 ZIP 失败: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("MinerU 解压 ZIP 失败: %w", err)
	}

	var markdown string
	images := make(map[string][]byte)

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		switch ext {
		case ".md":
			if markdown != "" {
				continue // 取第一个 .md 文件
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			markdown = string(data)

		case ".png", ".jpg", ".jpeg", ".svg", ".bmp", ".gif", ".webp":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil || len(data) == 0 {
				continue
			}
			// 保持原始文件名，不重命名（markdown 引用已是正确的原始路径）
			name := filepath.Base(f.Name)
			images[name] = data
		}
	}

	if strings.TrimSpace(markdown) == "" {
		return nil, fmt.Errorf("MinerU 结果 ZIP 中未找到 Markdown 内容")
	}

	return &local.ParseResult{Markdown: markdown, Images: images}, nil
}

// truncate 截断字符串到指定长度，超出加省略号。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
