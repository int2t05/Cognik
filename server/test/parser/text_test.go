// Package parser_test 提供 parser 包及子包的集成测试。
//
// text_test.go 测试 TXT/MD 格式解析（通过 parser.Parser 和 local 子包双重验证）。
package parser_test

import (
	"strings"
	"testing"

	"cognos/internal/parser"
	"cognos/internal/parser/local"
)

// TestParseTxt 验证纯文本文件解析。
func TestParseTxt(t *testing.T) {
	text := "这是运维系统的使用说明文档。\n包含多行内容。"
	result, err := local.ParseTxt(strings.NewReader(text))
	if err != nil {
		t.Fatalf("TXT 解析失败: %v", err)
	}
	if result.Markdown != text {
		t.Errorf("TXT 内容应原样输出:\n  期望: %q\n  实际: %q", text, result.Markdown)
	}
}

// TestParseTxt_UTF8 验证 UTF-8 编码文本正确解析。
func TestParseTxt_UTF8(t *testing.T) {
	text := "运维 Cognos 系统 - 账号 Account 管理"
	result, err := local.ParseTxt(strings.NewReader(text))
	if err != nil {
		t.Fatalf("UTF-8 文本解析失败: %v", err)
	}
	if !strings.Contains(result.Markdown, "Cognos") {
		t.Error("UTF-8 英文内容应保留")
	}
	if !strings.Contains(result.Markdown, "账号") {
		t.Error("UTF-8 中文内容应保留")
	}
}

// TestParseTxt_Empty 验证空文本处理。
func TestParseTxt_Empty(t *testing.T) {
	result, err := local.ParseTxt(strings.NewReader(""))
	if err != nil {
		t.Fatalf("空文本解析不应报错: %v", err)
	}
	if result.Markdown != "" {
		t.Errorf("空文本应返回空字符串, 实际: %q", result.Markdown)
	}
}

// TestParser_TXT_ThroughParser 通过 Parser.Parse 路由验证 TXT 解析。
func TestParser_TXT_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	text := "这是运维系统的使用说明文档。\n包含多行内容。"
	result, err := p.Parse(strings.NewReader(text), "txt")
	if err != nil {
		t.Fatalf("TXT 解析失败: %v", err)
	}
	if result.Markdown != text {
		t.Errorf("TXT 内容应原样输出:\n  期望: %q\n  实际: %q", text, result.Markdown)
	}
}

// TestParser_MD_ThroughParser 通过 Parser.Parse 路由验证 MD 解析。
func TestParser_MD_ThroughParser(t *testing.T) {
	p := parser.NewParser()

	md := "# 运维手册\n\n## VPN 配置\n\n1. 下载客户端\n2. 输入服务器地址\n"
	result, err := p.Parse(strings.NewReader(md), "md")
	if err != nil {
		t.Fatalf("MD 解析失败: %v", err)
	}
	if !strings.Contains(result.Markdown, "运维手册") {
		t.Error("MD 解析应保留标题")
	}
	if !strings.Contains(result.Markdown, "VPN 配置") {
		t.Error("MD 解析应保留标题")
	}
	if !strings.Contains(result.Markdown, "下载客户端") {
		t.Error("MD 解析应保留列表内容")
	}
}
