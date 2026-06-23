package rag_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"opsmind/internal/rag"
)

// =============================================================================
// TXT / MD 解析测试
// =============================================================================

// TestDocParser_TXT 验证纯文本文件解析。
func TestDocParser_TXT(t *testing.T) {
	parser := rag.NewDocParser()

	text := "这是运维系统的使用说明文档。\n包含多行内容。"
	reader := strings.NewReader(text)

	result, err := parser.Parse(reader, "txt")
	if err != nil {
		t.Fatalf("TXT 解析失败: %v", err)
	}
	if result != text {
		t.Errorf("TXT 内容应原样输出:\n  期望: %q\n  实际: %q", text, result)
	}
}

// TestDocParser_MD 验证 Markdown 文件解析。
func TestDocParser_MD(t *testing.T) {
	parser := rag.NewDocParser()

	md := "# 运维手册\n\n## VPN 配置\n\n1. 下载客户端\n2. 输入服务器地址\n"
	reader := strings.NewReader(md)

	result, err := parser.Parse(reader, "md")
	if err != nil {
		t.Fatalf("MD 解析失败: %v", err)
	}
	if !strings.Contains(result, "运维手册") {
		t.Error("MD 解析应保留标题")
	}
	if !strings.Contains(result, "VPN 配置") {
		t.Error("MD 解析应保留标题")
	}
	if !strings.Contains(result, "下载客户端") {
		t.Error("MD 解析应保留列表内容")
	}
	if !strings.Contains(result, "\n") {
		t.Error("MD 解析应保留换行")
	}
}

// TestDocParser_TXT_UTF8 验证 UTF-8 编码文本正确解析。
func TestDocParser_TXT_UTF8(t *testing.T) {
	parser := rag.NewDocParser()

	text := "运维 OpsMind 系统 - 账号 Account 管理"
	reader := strings.NewReader(text)

	result, err := parser.Parse(reader, "txt")
	if err != nil {
		t.Fatalf("UTF-8 文本解析失败: %v", err)
	}
	if !strings.Contains(result, "OpsMind") {
		t.Error("UTF-8 英文内容应保留")
	}
	if !strings.Contains(result, "账号") {
		t.Error("UTF-8 中文内容应保留")
	}
}

// =============================================================================
// DOCX 解析测试
// =============================================================================

// TestDocParser_DOCX 验证 DOCX 解析基本功能。
//
// DOCX 是 ZIP 压缩包，构造最小的合法结构进行测试。
func TestDocParser_DOCX(t *testing.T) {
	parser := rag.NewDocParser()

	docxBytes := createMinimalDocx(t)
	reader := bytes.NewReader(docxBytes)

	result, err := parser.Parse(reader, "docx")
	if err != nil {
		t.Fatalf("DOCX 解析失败: %v", err)
	}
	if !strings.Contains(result, "DOCX") {
		t.Error("DOCX 解析应包含文档内容")
	}
	if !strings.Contains(result, "运维文档测试") {
		t.Error("DOCX 解析应包含中文内容")
	}
}

// createMinimalDocx 创建一个最小可用的 DOCX 文件（ZIP 格式）。
//
// DOCX 文件结构（OASIS OpenDocument 标准）：
//
//	[Content_Types].xml — 内容类型声明
//	_rels/.rels       — 关系文件
//	word/document.xml — 主文档内容
func createMinimalDocx(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// [Content_Types].xml
	ct, _ := w.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`))

	// _rels/.rels
	rels, _ := w.Create("_rels/.rels")
	rels.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`))

	// word/document.xml
	doc, _ := w.Create("word/document.xml")
	doc.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>DOCX 运维文档测试内容</w:t></w:r></w:p>
    <w:p><w:r><w:t>第二段：VPN 配置说明</w:t></w:r></w:p>
  </w:body>
</w:document>`))

	w.Close()
	return buf.Bytes()
}

// =============================================================================
// 错误处理测试
// =============================================================================

// TestDocParser_UnsupportedType 验证不支持的文件类型返回错误。
func TestDocParser_UnsupportedType(t *testing.T) {
	parser := rag.NewDocParser()

	reader := strings.NewReader("test")
	_, err := parser.Parse(reader, "xlsx")
	if err == nil {
		t.Error("不支持的文件类型应返回错误")
	}
	if !strings.Contains(err.Error(), "不支持") {
		t.Errorf("错误信息应说明不支持, 实际: %v", err)
	}
}

// TestDocParser_EmptyFile 验证空文件处理。
func TestDocParser_EmptyFile(t *testing.T) {
	parser := rag.NewDocParser()

	reader := strings.NewReader("")
	result, err := parser.Parse(reader, "txt")
	if err != nil {
		t.Fatalf("空文件解析不应报错: %v", err)
	}
	if result != "" {
		t.Errorf("空文件返回空字符串, 实际: %q", result)
	}
}

// =============================================================================
// PDF 解析测试
// =============================================================================

// TestDocParser_PDF 验证 PDF 解析基本功能。
func TestDocParser_PDF(t *testing.T) {
	parser := rag.NewDocParser()

	// 构造最小 PDF 文件
	pdfContent := createMinimalPDF(t)
	reader := bytes.NewReader(pdfContent)

	result, err := parser.Parse(reader, "pdf")
	if err != nil {
		t.Fatalf("PDF 解析失败: %v", err)
	}
	if result == "" {
		t.Error("PDF 解析不应返回空内容")
	}
}

// createMinimalPDF 创建一个最小可用的 PDF 文件。
//
// 使用 PDF 规范构造合法文件，正确计算 xref 偏移量。
func createMinimalPDF(t *testing.T) []byte {
	t.Helper()

	// 逐段构造 PDF 并精确记录每行偏移
	var b bytes.Buffer

	// Header
	b.WriteString("%PDF-1.4\n")

	// Object 1 (Catalog): offset 9
	obj1Offset := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2 (Pages): offset 58
	obj2Offset := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3 (Page)
	obj3Offset := b.Len()
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n/Contents 4 0 R /Resources << >> >>\nendobj\n")

	// Object 4 (Content stream)
	obj4Offset := b.Len()
	b.WriteString("4 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello OpsMind) Tj\nET\nendstream\nendobj\n")

	// Cross-reference table
	xrefOffset := b.Len()
	b.WriteString("xref\n0 5\n")
	b.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", obj1Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj2Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj3Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj4Offset)

	// Trailer
	b.WriteString("trailer\n<< /Size 5 /Root 1 0 R >>\n")
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF", xrefOffset)

	return b.Bytes()
}
