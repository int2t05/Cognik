// Package parser 实现多格式文档解析。
//
// docx.go 实现 DOCX (OOXML) 文件的文本与图片提取。
//
// 文本提取使用 Go 标准库 archive/zip + encoding/xml，
// 优先按命名空间匹配，命名空间不匹配时回退到标签名正则匹配（兼容非标准生成器）。
// 图片提取遍历 ZIP 中 word/media/ 前缀的文件，按原始扩展名命名。
package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// parseDocx 解析 DOCX (OOXML) 文件：提取文本 + 提取 word/media/ 图片。
func (p *Parser) parseDocx(reader io.Reader) (*ParseResult, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取 DOCX 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return nil, fmt.Errorf("DOCX 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	zipReader, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("打开 DOCX ZIP 失败: %w", err)
	}

	text, err := extractDocxTextFromZip(zipReader)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{Markdown: text}

	// 图片提取
	imgEntries := extractDocxImages(zipReader)
	if len(imgEntries) > 0 {
		images := make(map[string][]byte, len(imgEntries))
		var refs strings.Builder
		for _, e := range imgEntries {
			images[e.name] = e.data
			refs.WriteString("\n\n![](images/")
			refs.WriteString(e.name)
			refs.WriteString(")")
		}
		result.Images = images
		result.Markdown = text + refs.String()
	}

	return result, nil
}

// extractDocxTextFromZip 从 ZIP 中定位 word/document.xml 并提取文本。
func extractDocxTextFromZip(zipReader *zip.Reader) (string, error) {
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

// extractDocxImages 遍历 ZIP 中 word/media/ 前缀的文件，提取图片字节。
//
// 返回有序的图片列表，命名 img{N}.{ext}（按原始扩展名）。
func extractDocxImages(zipReader *zip.Reader) []imageEntry {
	var mediaNames []string
	for _, f := range zipReader.File {
		if strings.HasPrefix(f.Name, "word/media/") && !f.FileInfo().IsDir() {
			mediaNames = append(mediaNames, f.Name)
		}
	}
	if len(mediaNames) == 0 {
		return nil
	}
	// 排序保证命名稳定
	sort.Strings(mediaNames)

	var entries []imageEntry
	for i, name := range mediaNames {
		f := findZipFile(zipReader, name)
		if f == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		if ext == "" {
			ext = "png"
		}
		entries = append(entries, imageEntry{
			name: fmt.Sprintf("img%d.%s", i+1, ext),
			data: data,
		})
	}

	return entries
}

// findZipFile 在 ZIP 中按名称查找文件。
func findZipFile(zipReader *zip.Reader, name string) *zip.File {
	for _, f := range zipReader.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// =============================================================================
// DOCX XML 结构化解析
// =============================================================================

// docxDocument DOCX document.xml 的 XML 结构，支持标准 OOXML 命名空间。
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
	Text    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main t"`
	TabChar string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tab"` // 制表符
	LineBrk string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main br"`   // 换行
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

// =============================================================================
// DOCX 正则回退解析
// =============================================================================

// extractDocxTextRegex 正则回退提取 DOCX 文本（兼容非标准命名空间）。
//
// 当命名空间不匹配导致结构化解析为空时启用此回退。
// 直接匹配 <w:t...>...</w:t> 标签，忽略命名空间前缀差异。
//
// 段落边界检测：判断 </w:p> 是否落在当前文本节点与下一个 w:t 之间，
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
