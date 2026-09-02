// Package parser 实现多格式文档解析。
//
// pptx.go 实现 PPTX (PowerPoint) 文件的文本与图片提取。
//
// PPTX 是 OOXML ZIP 包，结构与 DOCX 类似：
//   - 文本：遍历 ppt/slides/slide{N}.xml，提取 DrawingML <a:t> 标签文本
//   - 图片：遍历 ppt/media/ 目录，按原始扩展名命名
//
// 每张 slide 的文本作为一段，用 --- 分隔。
// 图片提取使用 Go 标准库 archive/zip + 正则匹配，无外部依赖。
package parser

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// pptxTextRegex 匹配 DrawingML 文本节点 <a:t>...</a:t>（忽略命名空间前缀差异）。
var pptxTextRegex = regexp.MustCompile(`<(?:\w+:)?t[^>]*>([^<]*)</(?:\w+:)?t>`)

// parsePPTX 解析 PPTX 文件：遍历 slide XML 提取文本 + 提取 ppt/media/ 图片。
func (p *Parser) parsePPTX(reader io.Reader) (*ParseResult, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取 PPTX 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return nil, fmt.Errorf("PPTX 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	zipReader, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("打开 PPTX ZIP 失败: %w", err)
	}

	text, err := extractPPTXTextFromZip(zipReader)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{Markdown: text}

	// 图片提取
	imgEntries := extractPPTXImages(zipReader)
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
		result.Markdown += refs.String()
	}

	return result, nil
}

// extractPPTXTextFromZip 定位 ppt/slides/slide{N}.xml 并按页码顺序提取文本。
func extractPPTXTextFromZip(zipReader *zip.Reader) (string, error) {
	// 收集 slide 文件名并按页码排序
	type slideFile struct {
		pageNum int
		name    string
	}
	var slides []slideFile
	for _, f := range zipReader.File {
		// 匹配 ppt/slides/slide1.xml, ppt/slides/slide2.xml 等
		base := filepath.Base(f.Name)
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(base, ".xml") {
			continue
		}
		numStr := strings.TrimSuffix(strings.TrimPrefix(base, "slide"), ".xml")
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		slides = append(slides, slideFile{pageNum: n, name: f.Name})
	}

	if len(slides) == 0 {
		return "", fmt.Errorf("PPTX 中未找到 ppt/slides/slide*.xml")
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].pageNum < slides[j].pageNum })

	var buf strings.Builder
	for i, sf := range slides {
		if i > 0 {
			buf.WriteString("\n\n---\n\n")
		}
		text, err := extractSlideText(zipReader, sf.name)
		if err != nil {
			return "", fmt.Errorf("解析 slide %d 失败: %w", sf.pageNum, err)
		}
		buf.WriteString(text)
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("PPTX 所有 slide 文本为空")
	}
	return result, nil
}

// extractSlideText 从单个 slide XML 中提取 <a:t> 文本节点。
//
// 使用正则匹配兼容不同命名空间前缀的生成器。
func extractSlideText(zipReader *zip.Reader, slidePath string) (string, error) {
	f := findZipFile(zipReader, slidePath)
	if f == nil {
		return "", fmt.Errorf("slide 文件不存在: %s", slidePath)
	}
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("打开 slide 失败: %w", err)
	}
	xmlData, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", fmt.Errorf("读取 slide 失败: %w", err)
	}

	matches := pptxTextRegex.FindAllStringSubmatch(string(xmlData), -1)
	var buf strings.Builder
	for _, m := range matches {
		if m[1] != "" {
			buf.WriteString(m[1])
			buf.WriteByte('\n')
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

// extractPPTXImages 遍历 ZIP 中 ppt/media/ 前缀的文件，提取图片字节。
//
// 返回有序的图片列表，命名 img{N}.{ext}（按原始扩展名）。
func extractPPTXImages(zipReader *zip.Reader) []imageEntry {
	var mediaNames []string
	for _, f := range zipReader.File {
		if strings.HasPrefix(f.Name, "ppt/media/") && !f.FileInfo().IsDir() {
			mediaNames = append(mediaNames, f.Name)
		}
	}
	if len(mediaNames) == 0 {
		return nil
	}
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
		ext = strings.ToLower(ext)
		if ext == "jpeg" {
			ext = "jpg"
		}
		entries = append(entries, imageEntry{
			name: fmt.Sprintf("img%d.%s", i+1, ext),
			data: data,
		})
	}
	return entries
}
