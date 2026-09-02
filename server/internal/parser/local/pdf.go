// Package local 实现本地纯 Go 文档解析降级引擎。
//
// pdf.go 实现 PDF 文件的文本与图片提取。
//
// 文本提取使用 ledongthuc/pdf（纯 Go，无 CGO 依赖），逐页 GetPlainText。
// 图片提取使用 pdfcpu（github.com/pdfcpu/pdfcpu），支持 JPEG/PNG/TIFF/JPX/JBIG2 等。
// 图片提取失败不阻塞文本提取——降级为仅返回文本。
package local

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// ParsePDF 解析 PDF 文件：逐页提取文本 + pdfcpu 提取嵌入图片。
func ParsePDF(reader io.Reader) (*ParseResult, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取 PDF 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return nil, fmt.Errorf("PDF 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	text, err := extractPDFText(b)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{Markdown: text}

	// 图片提取（失败不阻塞文本，降级为仅文本）
	imgEntries := extractPDFImages(b)
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

// extractPDFText 使用 ledongthuc/pdf 逐页提取文本。
//
// 使用 bytes.NewReader 而非 strings.NewReader(string(b))，
// 避免非 UTF-8 二进制字节在 Go string 往返中损坏。
func extractPDFText(b []byte) (string, error) {
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

// extractPDFImages 使用 pdfcpu 提取 PDF 中的嵌入图片。
//
// 返回有序的图片列表，命名 img{N}.{ext}（N 从 1）。
// 提取失败时返回 nil，仅记录 warning，不阻塞文本提取。
func extractPDFImages(b []byte) []imageEntry {
	// pdfcpu ExtractImagesRaw：selectedPages=nil 表示全部页，conf=nil 使用默认配置
	pageImages, err := api.ExtractImagesRaw(bytes.NewReader(b), nil, nil)
	if err != nil {
		// pdfcpu 在 skip 模式下可能返回部分结果 + UnsupportedResourceError
		if len(pageImages) == 0 {
			slog.Warn("PDF 图片提取失败，跳过图片", "error", err)
			return nil
		}
		slog.Warn("PDF 图片提取部分失败，使用已提取的图片", "error", err)
	}

	var entries []imageEntry
	imgNum := 0
	for _, pageMap := range pageImages {
		// 按 objNr 排序保证命名稳定
		objNrs := make([]int, 0, len(pageMap))
		for objNr := range pageMap {
			objNrs = append(objNrs, objNr)
		}
		sort.Ints(objNrs)

		for _, objNr := range objNrs {
			img := pageMap[objNr]
			if img.Reader == nil {
				continue
			}
			data, err := io.ReadAll(img.Reader)
			if err != nil || len(data) == 0 {
				slog.Warn("读取 PDF 图片字节失败", "obj_nr", objNr, "error", err)
				continue
			}
			imgNum++
			ext := pdfImageExt(img.FileType)
			entries = append(entries, imageEntry{
				name: fmt.Sprintf("img%d.%s", imgNum, ext),
				data: data,
			})
		}
	}

	return entries
}

// pdfImageExt 将 pdfcpu 的 FileType 归一化为文件扩展名。
func pdfImageExt(fileType string) string {
	ft := strings.ToLower(strings.TrimSpace(fileType))
	switch ft {
	case "jpeg":
		return "jpg"
	case "":
		return "png"
	default:
		return ft
	}
}
