// Package parser_test 提供 parser 包及子包的集成测试。
//
// docx_test.go 测试 DOCX 格式解析（通过 local 子包和 parser.Parser 双重验证）。
package parser_test

import (
	"bytes"
	"strings"
	"testing"

	"cognik/internal/parser"
	"cognik/internal/parser/local"
)

// TestParseDocx 验证 DOCX 解析基本功能（直接调用 local.ParseDocx）。
func TestParseDocx(t *testing.T) {
	docxBytes := createMinimalDocx(t)
	result, err := local.ParseDocx(bytes.NewReader(docxBytes))
	if err != nil {
		t.Fatalf("DOCX 解析失败: %v", err)
	}
	if result.Markdown == "" {
		t.Error("DOCX 解析不应返回空内容")
	}
	if !strings.Contains(result.Markdown, "DOCX") {
		t.Error("DOCX 解析应包含文档内容")
	}
	if !strings.Contains(result.Markdown, "运维文档测试") {
		t.Error("DOCX 解析应包含中文内容")
	}
}

// TestParser_DOCX_ThroughParser 通过 Parser.Parse 路由验证 DOCX 解析。
func TestParser_DOCX_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	docxBytes := createMinimalDocx(t)
	result, err := p.Parse(bytes.NewReader(docxBytes), "docx")
	if err != nil {
		t.Fatalf("DOCX 解析失败: %v", err)
	}
	if !strings.Contains(result.Markdown, "DOCX") {
		t.Error("DOCX 解析应包含文档内容")
	}
	if !strings.Contains(result.Markdown, "运维文档测试") {
		t.Error("DOCX 解析应包含中文内容")
	}
	if !strings.Contains(result.Markdown, "VPN") {
		t.Error("DOCX 解析应包含第二段内容")
	}
}
