// Package local 实现本地纯 Go 文档解析降级引擎。
//
// xlsx.go 实现 XLSX (Excel) 文件的文本与图片提取。
//
// 文本提取使用 excelize（github.com/xuri/excelize/v2），
// 遍历所有 sheet 的行，单元格值以 Markdown 表格格式输出。
// 图片提取使用 GetPictureCells + GetPictures，按 img{N}.{ext} 命名。
package local

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ParseXLSX 解析 XLSX 文件：遍历所有 sheet 生成 Markdown 表格 + 提取嵌入图片。
func ParseXLSX(reader io.Reader) (*ParseResult, error) {
	b, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX 文件失败: %w", err)
	}
	if len(b) >= maxDocumentSize {
		return nil, fmt.Errorf("XLSX 超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("打开 XLSX 失败: %w", err)
	}
	defer f.Close()

	var buf strings.Builder
	sheets := f.GetSheetList()

	for sheetIdx, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("读取 XLSX sheet %q 失败: %w", sheet, err)
		}

		// sheet 标题
		if sheetIdx > 0 {
			buf.WriteString("\n\n---\n\n")
		}
		buf.WriteString("## ")
		buf.WriteString(sheet)
		buf.WriteByte('\n')

		// 行 → Markdown 表格
		for rowIdx, row := range rows {
			// 跳过全空行
			if isRowEmpty(row) {
				continue
			}
			buf.WriteString("| ")
			buf.WriteString(strings.Join(row, " | "))
			buf.WriteString(" |\n")

			// 第一行数据后输出分隔行（Markdown 表头分隔）
			if rowIdx == 0 {
				buf.WriteString(strings.Repeat("| --- ", len(row)))
				buf.WriteString("|\n")
			}
		}
	}

	result := &ParseResult{Markdown: strings.TrimSpace(buf.String())}

	// 图片提取
	imgEntries := extractXLSXImages(f, sheets)
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

// extractXLSXImages 遍历所有 sheet 的图片单元格，提取图片字节。
//
// 返回有序的图片列表，命名 img{N}.{ext}。
func extractXLSXImages(f *excelize.File, sheets []string) []imageEntry {
	var entries []imageEntry
	imgNum := 0
	for _, sheet := range sheets {
		cells, err := f.GetPictureCells(sheet)
		if err != nil {
			continue
		}
		for _, cell := range cells {
			pics, err := f.GetPictures(sheet, cell)
			if err != nil {
				continue
			}
			for _, pic := range pics {
				if len(pic.File) == 0 {
					continue
				}
				imgNum++
				ext := strings.ToLower(strings.TrimSpace(pic.Extension))
				if ext == "" {
					ext = "png"
				}
				if ext == "jpeg" {
					ext = "jpg"
				}
				entries = append(entries, imageEntry{
					name: fmt.Sprintf("img%d.%s", imgNum, ext),
					data: pic.File,
				})
			}
		}
	}
	return entries
}

// isRowEmpty 判断行是否全为空字符串。
func isRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
