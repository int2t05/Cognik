// Package parser_test 提供 parser 包及子包的集成测试。
//
// pptx_test.go 测试 PPTX 格式解析（通过 local 子包和 parser.Parser 双重验证）。
package parser_test

import (
	"bytes"
	"strings"
	"testing"

	"cognos/internal/parser"
	"cognos/internal/parser/local"
)

// TestParsePPTX 验证 PPTX 解析基本功能（直接调用 local.ParsePPTX）。
func TestParsePPTX(t *testing.T) {
	pptxBytes := createMinimalPPTX(t)
	result, err := local.ParsePPTX(bytes.NewReader(pptxBytes))
	if err != nil {
		t.Fatalf("PPTX 解析失败: %v", err)
	}
	if result.Markdown == "" {
		t.Error("PPTX 解析不应返回空内容")
	}
	if !strings.Contains(result.Markdown, "运维幻灯片测试") {
		t.Error("PPTX 解析应包含幻灯片文本内容")
	}
}

// TestParser_PPTX_ThroughParser 通过 Parser.Parse 路由验证 PPTX 解析。
func TestParser_PPTX_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	pptxBytes := createMinimalPPTX(t)
	result, err := p.Parse(bytes.NewReader(pptxBytes), "pptx")
	if err != nil {
		t.Fatalf("PPTX 解析失败: %v", err)
	}
	if !strings.Contains(result.Markdown, "运维幻灯片测试") {
		t.Error("PPTX 解析应包含幻灯片文本内容")
	}
	if !strings.Contains(result.Markdown, "系统监控") {
		t.Error("PPTX 解析应包含第二行文本")
	}
}
