// Package parser 实现多格式文档解析，支持文本和图片提取。
//
// 支持格式：PDF / DOCX / XLSX / PPTX / MD / TXT。
// 解析结果为 Markdown 正文 + 图片名→字节映射，图片在正文中以 ![](images/{name}) 引用。
//
// 双引擎架构：MinerU 云端高精度优先，本地纯 Go 库兜底降级。
// MinerU 还原 LaTeX 公式 / 表格结构 / 版面布局，补足本地库短板；
// 无 API Key 或调用失败时自动降级到本地解析，保证链路不中断。
//
// 为什么独立成包：原先解析逻辑内嵌于 rag 包，接口仅返回纯文本。
// 独立 parser 包解耦解析职责，支持图片提取，便于知识库上传时同时存储正文与图片。
package parser

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// maxDocumentSize 文档最大解析大小（100MB），防止恶意文件导致 OOM。
const maxDocumentSize = 100 * 1024 * 1024

// ParseResult 解析结果。
type ParseResult struct {
	Markdown string            // 正文 Markdown（图片用 ![](images/{name}) 引用）
	Images   map[string][]byte // 图片名→字节（如 "img1.png": []byte）
}

// Parser 多格式文档解析器。
//
// mineru 为可选的 MinerU 云端引擎，nil 时直接走本地解析。
type Parser struct {
	mineru *MinerUEngine
}

// ParserOption Parser 构造选项。
type ParserOption func(*Parser)

// WithMinerU 注入 MinerU 云端引擎（nil 表示不启用云端解析）。
func WithMinerU(engine *MinerUEngine) ParserOption {
	return func(p *Parser) { p.mineru = engine }
}

// NewParser 创建文档解析器实例。
//
// 可选注入 MinerU 引擎：parser.NewParser(parser.WithMinerU(engine))
func NewParser(opts ...ParserOption) *Parser {
	p := &Parser{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Parse 根据文件类型解析文档，返回 Markdown + 图片。
//
// 解析顺序：
//  1. MinerU 云端引擎（若已配置且 API Key 可用）—— 成功直接返回
//  2. 本地纯 Go 解析（MinerU 失败时降级）
//
// reader 在解析完成后不会关闭，由调用方负责关闭。
// fileType 支持：pdf / docx / xlsx / pptx / md / txt（大小写不敏感）。
func (p *Parser) Parse(reader io.Reader, fileType string) (*ParseResult, error) {
	// 缓冲全部字节：MinerU 失败后降级到本地需要复用原始数据
	data, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}
	if len(data) >= maxDocumentSize {
		return nil, fmt.Errorf("文档超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	// 1. MinerU 云端解析（若已配置）
	if p.mineru != nil {
		fileName := "document." + strings.ToLower(fileType)
		result, err := p.mineru.Parse(bytes.NewReader(data), fileName)
		if err == nil {
			return result, nil
		}
		slog.Warn("MinerU 解析失败，降级到本地解析", "file_type", fileType, "error", err)
	}

	// 2. 本地降级
	switch strings.ToLower(fileType) {
	case "txt", "md", "markdown":
		return p.parseTxt(bytes.NewReader(data))
	case "docx":
		return p.parseDocx(bytes.NewReader(data))
	case "pdf":
		return p.parsePDF(bytes.NewReader(data))
	case "xlsx":
		return p.parseXLSX(bytes.NewReader(data))
	case "pptx":
		return p.parsePPTX(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s（支持: pdf/docx/xlsx/pptx/md/txt）", fileType)
	}
}
