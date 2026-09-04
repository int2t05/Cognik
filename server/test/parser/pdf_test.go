// Package parser_test 提供 parser 包及子包的集成测试。
//
// pdf_test.go 测试 PDF 格式解析（通过 local 子包和 parser.Parser 双重验证）。
package parser_test

import (
	"bytes"
	"strings"
	"testing"

	"cognos/internal/parser"
	"cognos/internal/parser/local"
)

// TestParsePDF 验证 PDF 解析基本功能（直接调用 local.ParsePDF）。
func TestParsePDF(t *testing.T) {
	pdfBytes := createMinimalPDF(t)
	result, err := local.ParsePDF(bytes.NewReader(pdfBytes))
	if err != nil {
		t.Fatalf("PDF 解析失败: %v", err)
	}
	if result.Markdown == "" {
		t.Error("PDF 解析不应返回空内容")
	}
}

// TestParser_PDF_ThroughParser 通过 Parser.Parse 路由验证 PDF 解析。
func TestParser_PDF_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	pdfBytes := createMinimalPDF(t)
	result, err := p.Parse(bytes.NewReader(pdfBytes), "pdf")
	if err != nil {
		t.Fatalf("PDF 解析失败: %v", err)
	}
	if result.Markdown == "" {
		t.Error("PDF 解析不应返回空内容")
	}
	if !strings.Contains(result.Markdown, "Cognos") {
		t.Error("PDF 解析应包含文本内容")
	}
}
