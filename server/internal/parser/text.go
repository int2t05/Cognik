// Package parser 实现多格式文档解析。
//
// text.go 实现 TXT/MD 文件的纯文本读取。
package parser

import (
	"fmt"
	"io"
)

// parseTxt 读取纯文本/Markdown 文件，原样返回文本内容。
func (p *Parser) parseTxt(reader io.Reader) (*ParseResult, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取文本文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return nil, fmt.Errorf("文档超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}
	return &ParseResult{Markdown: string(b)}, nil
}
