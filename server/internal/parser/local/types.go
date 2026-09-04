// Package local 实现本地纯 Go 文档解析降级引擎。
//
// 支持 TXT/MD/PDF/DOCX/XLSX/PPTX 的文本与图片提取，
// 以及 doc/xls/ppt 旧格式通过 markitdown 的 Python 子进程解析。
//
// 各格式由独立文件实现，共享 ParseResult / imageEntry 等类型定义于此。
package local

// maxDocumentSize 文档最大解析大小（100MB），防止恶意文件导致 OOM。
const maxDocumentSize = 100 * 1024 * 1024

// ParseResult 解析结果。
//
// Markdown 为正文，Images 为图片名→字节映射。图片引用的最终归一（统一 image/{name} 前缀、
// 内容 hash 命名）由 parser 包出口的 normalizeImagePaths 完成，各引擎只负责提取。
type ParseResult struct {
	Markdown string            // 正文 Markdown（图片引用经 parser 归一为 image/{name}）
	Images   map[string][]byte // 图片名→字节（归一后 name 为 {hash}.{ext}）
}

// imageEntry 图片名与字节的有序对，保证图片命名稳定。
type imageEntry struct {
	name string
	data []byte
}
