// Package parser_test 提供 parser 包及子包的集成测试。
//
// xlsx_test.go 测试 XLSX 格式解析（通过 local 子包和 parser.Parser 双重验证）。
package parser_test

import (
	"bytes"
	"strings"
	"testing"

	"cognos/internal/parser"
	"cognos/internal/parser/local"
)

// TestParseXLSX 验证 XLSX 解析基本功能（直接调用 local.ParseXLSX）。
func TestParseXLSX(t *testing.T) {
	xlsxBytes := createMinimalXLSX(t)
	result, err := local.ParseXLSX(bytes.NewReader(xlsxBytes))
	if err != nil {
		t.Fatalf("XLSX 解析失败: %v", err)
	}
	if result.Markdown == "" {
		t.Error("XLSX 解析不应返回空内容")
	}
	if !strings.Contains(result.Markdown, "CPU 使用率") {
		t.Error("XLSX 解析应包含单元格内容")
	}
	if !strings.Contains(result.Markdown, "Sheet1") {
		t.Error("XLSX 解析应包含 sheet 名称")
	}
}

// TestParser_XLSX_ThroughParser 通过 Parser.Parse 路由验证 XLSX 解析。
func TestParser_XLSX_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	xlsxBytes := createMinimalXLSX(t)
	result, err := p.Parse(bytes.NewReader(xlsxBytes), "xlsx")
	if err != nil {
		t.Fatalf("XLSX 解析失败: %v", err)
	}
	if !strings.Contains(result.Markdown, "CPU 使用率") {
		t.Error("XLSX 解析应包含单元格内容")
	}
	if !strings.Contains(result.Markdown, "内存使用率") {
		t.Error("XLSX 解析应包含所有行数据")
	}
}
