// Package parser_test 提供 parser 包及子包的集成测试。
//
// 本文件包含测试用小型文档构造器，在内存中生成合法的最小格式文件，
// 避免测试依赖外部文件资源。
package parser_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"
)

// createMinimalDocx 创建一个最小可用的 DOCX 文件（ZIP 格式）。
//
// DOCX 文件结构（OOXML 标准）：
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

// createMinimalPPTX 创建一个最小可用的 PPTX 文件（ZIP 格式）。
//
// PPTX 文件结构（OOXML 标准）：
//
//	[Content_Types].xml      — 内容类型声明
//	_rels/.rels              — 包关系文件
//	ppt/presentation.xml     — 演示文稿定义
//	ppt/slides/slide1.xml    — 幻灯片内容
func createMinimalPPTX(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// [Content_Types].xml
	ct, _ := w.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`))

	// _rels/.rels
	rels, _ := w.Create("_rels/.rels")
	rels.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`))

	// ppt/presentation.xml
	pres, _ := w.Create("ppt/presentation.xml")
	pres.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldIdLst><p:sldId id="1" r:id="rId1"/></p:sldIdLst>
</p:presentation>`))

	// ppt/slides/slide1.xml — DrawingML 文本节点
	slide, _ := w.Create("ppt/slides/slide1.xml")
	slide.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<a:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
    <p:spTree>
      <p:sp>
        <p:txBody>
          <a:p><a:r><a:t>PPTX 运维幻灯片测试</a:t></a:r></a:p>
          <a:p><a:r><a:t>第二行：系统监控</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</a:sld>`))

	w.Close()
	return buf.Bytes()
}

// createMinimalXLSX 使用 excelize 创建一个最小可用的 XLSX 文件。
func createMinimalXLSX(t *testing.T) []byte {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()

	f.SetCellValue("Sheet1", "A1", "指标")
	f.SetCellValue("Sheet1", "B1", "值")
	f.SetCellValue("Sheet1", "A2", "CPU 使用率")
	f.SetCellValue("Sheet1", "B2", "85%")
	f.SetCellValue("Sheet1", "A3", "内存使用率")
	f.SetCellValue("Sheet1", "B3", "72%")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("创建测试 XLSX 失败: %v", err)
	}
	return buf.Bytes()
}

// createMinimalPDF 创建一个最小可用的 PDF 文件。
//
// 使用 PDF 规范构造合法文件，正确计算 xref 偏移量。
func createMinimalPDF(t *testing.T) []byte {
	t.Helper()

	var b bytes.Buffer

	// Header
	b.WriteString("%PDF-1.4\n")

	// Object 1 (Catalog)
	obj1Offset := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2 (Pages)
	obj2Offset := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3 (Page)
	obj3Offset := b.Len()
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n/Contents 4 0 R /Resources << >> >>\nendobj\n")

	// Object 4 (Content stream)
	obj4Offset := b.Len()
	b.WriteString("4 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello Cognik) Tj\nET\nendstream\nendobj\n")

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
