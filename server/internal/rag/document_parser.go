// Package rag 实现自建 RAG 检索引擎。
//
// document_parser.go 实现多格式文档解析器。
//
// 支持格式：PDF / DOCX / MD / TXT
//
// 为什么使用 ledongthuc/pdf 而非其他 PDF 库：
// ledongthuc/pdf 是纯 Go 实现，无 CGO 依赖，交叉编译友好。
// 虽然高级功能（表格、图片）不如 poppler，但 MVP 阶段仅需文本提取即可。
//
// DOCX 解析使用 Go 标准库 archive/zip + encoding/xml，
// 直接解析 OOXML 格式，优先按命名空间匹配，
// 命名空间不匹配时回退到标签名匹配（兼容非标准生成器）。
package rag

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"archive/zip"
	"bytes"
	"encoding/xml"
	"regexp"

	"github.com/ledongthuc/pdf"
)

// maxDocumentSize 文档最大解析大小（100MB），防止恶意文件导致 OOM。
const maxDocumentSize = 100 * 1024 * 1024

// DocParser 多格式文档解析器。
type DocParser struct{}

// NewDocParser 创建文档解析器实例。
func NewDocParser() *DocParser {
	return &DocParser{}
}

// Parse 根据文件类型解析文档并返回纯文本内容。
//
// reader 在解析完成后不会关闭，由调用方负责关闭。
// fileType 支持：pdf / docx / md / txt（大小写不敏感）。
func (p *DocParser) Parse(reader io.Reader, fileType string) (string, error) {
	switch strings.ToLower(fileType) {
	case "txt":
		return p.parseTxt(reader)
	case "md", "markdown":
		return p.parseTxt(reader)
	case "docx":
		return p.parseDocx(reader)
	case "pdf":
		return p.parsePDF(reader)
	default:
		return "", fmt.Errorf("不支持的文件类型: %s（支持的格式：pdf / docx / md / txt）", fileType)
	}
}

// parseTxt 读取纯文本/Markdown 文件。
func (p *DocParser) parseTxt(reader io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return "", fmt.Errorf("读取文本文件失败: %w", err)
	}
	// 检测是否达到上限
	if len(b) >= maxDocumentSize {
		return "", fmt.Errorf("文档超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}
	return string(b), nil
}

// parsePDF 解析 PDF 文件，逐页提取文本。
//
// 注意：使用 bytes.NewReader 而非 strings.NewReader(string(b))，
// 避免非 UTF-8 二进制字节在 Go string 往返中损坏。
func (p *DocParser) parsePDF(reader io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return "", fmt.Errorf("读取 PDF 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return "", fmt.Errorf("PDF 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	pdfReader, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", fmt.Errorf("打开 PDF 失败: %w", err)
	}

	var buf strings.Builder
	var pageErrors int
	for i := 1; i <= pdfReader.NumPage(); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			pageErrors++
			slog.Warn("PDF 页面解析失败", "page", i, "error", err)
			continue
		}
		buf.WriteString(text)
		buf.WriteByte('\n')
	}

	if pageErrors > 0 {
		slog.Warn("PDF 部分页面解析失败", "total_pages", pdfReader.NumPage(), "failed_pages", pageErrors)
	}

	result := strings.TrimSpace(buf.String())
	if result == "" && pageErrors == pdfReader.NumPage() {
		return "", fmt.Errorf("PDF 所有页面解析均失败")
	}

	return result, nil
}

// parseDocx 解析 DOCX (OOXML) 文件。
//
// 优先按命名空间解析，解析结果为空时回退到正则提取（兼容非标准命名空间）。
func (p *DocParser) parseDocx(reader io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return "", fmt.Errorf("读取 DOCX 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return "", fmt.Errorf("DOCX 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}
	return parseDocxFromBytes(b)
}

// parseDocxFromBytes 从字节数组中解析 DOCX 内容。
func parseDocxFromBytes(data []byte) (string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("打开 DOCX ZIP 失败: %w", err)
	}

	var documentXML []byte
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("打开 DOCX document.xml 失败: %w", err)
			}
			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("读取 DOCX document.xml 失败: %w", err)
			}
			break
		}
	}

	if documentXML == nil {
		return "", fmt.Errorf("DOCX 中未找到 word/document.xml")
	}

	// 先尝试结构化解析（标准 OOXML 命名空间）
	text, err := extractDocxText(documentXML)
	if err != nil {
		return "", err
	}

	// 结构化解析为空时，回退到正则提取（兼容非标准命名空间的生成器）
	if strings.TrimSpace(text) == "" {
		text = extractDocxTextRegex(documentXML)
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("DOCX 内容为空（可能使用了不支持的命名空间或格式）")
	}

	return text, nil
}

// docxDocument DOCX document.xml 的 XML 结构。
//
// 支持标准 OOXML 命名空间。
type docxDocument struct {
	XMLName xml.Name   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main document"`
	Body    docxBody   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p"`
	Tables     []docxTable     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tbl"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main r"`
}

type docxRun struct {
	Text     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main t"`
	TabChar  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tab"`  // 制表符
	LineBrk  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main br"`    // 换行
}

type docxTable struct {
	Rows []docxTableRow `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tr"`
}

type docxTableRow struct {
	Cells []docxTableCell `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tc"`
}

type docxTableCell struct {
	Paragraphs []docxParagraph `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p"`
}

// extractDocxText 从 DOCX XML 中提取文本（结构化解析）。
//
// 解析段落和表格，提取所有文本节点。
func extractDocxText(xmlData []byte) (string, error) {
	var doc docxDocument
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return "", fmt.Errorf("解析 DOCX XML 失败: %w", err)
	}

	var buf strings.Builder

	// 段落文本
	for _, para := range doc.Body.Paragraphs {
		text := extractParagraphText(para)
		if text != "" {
			buf.WriteString(text)
			buf.WriteByte('\n')
		}
	}

	// 表格文本
	for _, table := range doc.Body.Tables {
		for _, row := range table.Rows {
			var rowText []string
			for _, cell := range row.Cells {
				var cellBuf strings.Builder
				for _, para := range cell.Paragraphs {
					cellBuf.WriteString(extractParagraphText(para))
				}
				rowText = append(rowText, strings.TrimSpace(cellBuf.String()))
			}
			if len(rowText) > 0 {
				buf.WriteString(strings.Join(rowText, " | "))
				buf.WriteByte('\n')
			}
		}
		buf.WriteByte('\n')
	}

	return strings.TrimSpace(buf.String()), nil
}

// extractParagraphText 从段落中提取文本（含 tab/br 语义标记）。
func extractParagraphText(para docxParagraph) string {
	var buf strings.Builder
	for _, run := range para.Runs {
		if run.TabChar != "" || run.LineBrk != "" {
			buf.WriteByte('\n')
		}
		if run.Text != "" {
			buf.WriteString(run.Text)
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractDocxTextRegex 正则回退提取 DOCX 文本（兼容非标准命名空间）。
//
// 当命名空间不匹配导致结构化解析为空时启用此回退。
// 直接匹配 <w:t...>...</w:t> 标签，忽略命名空间前缀差异。
//
// 段落边界检测：判断 </w:p> 是否落在当前文本节点与下一个 w:t 之间（而非硬编码 200 字节窗口），
// 避免长文本节点中后段的段落标记被遗漏。
var docxTextRegex = regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
var docxTabRegex = regexp.MustCompile(`<w:tab[^>]*/>`)
var docxBrRegex = regexp.MustCompile(`<w:br[^>]*/>`)
var docxParaEndRegex = regexp.MustCompile(`</w:p>`)

func extractDocxTextRegex(xmlData []byte) string {
	s := string(xmlData)

	// 提取所有 w:t 文本节点
	matches := docxTextRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return ""
	}

	// 所有 w:t 标签的匹配位置
	matchIndices := docxTextRegex.FindAllStringSubmatchIndex(s, -1)

	// 段落边界位置（</w:p> 的结束位置）
	paraEnds := docxParaEndRegex.FindAllStringIndex(s, -1)

	var buf strings.Builder
	tagIdx := 0
	for i, m := range matchIndices {
		// 检查当前 w:t 前是否有 tab 或 br 标记
		region := s[tagIdx:m[0]]
		if docxTabRegex.MatchString(region) || docxBrRegex.MatchString(region) {
			buf.WriteByte('\n')
		}
		buf.WriteString(matches[i][1])
		tagIdx = m[1]

		// 确定下一个 w:t 的起始位置（或字符串末尾）作为段落边界搜索窗口上限
		nextTagStart := len(s)
		if i+1 < len(matchIndices) {
			nextTagStart = matchIndices[i+1][0]
		}

		// 检查当前 w:t 和下一个 w:t 之间是否有段落结束标记
		for _, pe := range paraEnds {
			if pe[1] > m[0] && pe[1] <= nextTagStart {
				buf.WriteByte('\n')
				break
			}
		}
	}

	return strings.TrimSpace(buf.String())
}
