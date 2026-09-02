// Package parser 实现多格式文档解析，支持文本和图片提取。
//
// 支持格式：PDF / DOCX / MD / TXT。
// 解析结果为 Markdown 正文 + 图片名→字节映射，图片在正文中以 ![](images/{name}) 引用。
//
// 为什么独立成包：原先解析逻辑内嵌于 rag 包，接口仅返回纯文本。
// 独立 parser 包解耦解析职责，支持图片提取，便于知识库上传时同时存储正文与图片。
package parser

import (
	"fmt"
	"io"
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
type Parser struct{}

// NewParser 创建文档解析器实例。
func NewParser() *Parser { return &Parser{} }

// Parse 根据文件类型解析文档，返回 Markdown + 图片。
//
// reader 在解析完成后不会关闭，由调用方负责关闭。
// fileType 支持：pdf / docx / md / txt（大小写不敏感）。
func (p *Parser) Parse(reader io.Reader, fileType string) (*ParseResult, error) {
	switch strings.ToLower(fileType) {
	case "txt", "md", "markdown":
		return p.parseTxt(reader)
	case "docx":
		return p.parseDocx(reader)
	case "pdf":
		return p.parsePDF(reader)
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s（支持: pdf/docx/md/txt）", fileType)
	}
}
