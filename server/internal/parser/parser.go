// Package parser 实现多格式文档解析，支持文本和图片提取。
// 双引擎：MinerU 云端高精度优先，本地纯 Go 库兜底降级。
// 子包：mineru/（云端）、local/（本地降级含旧格式 markitdown 子进程）。
package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"cognos/internal/parser/local"
	"cognos/internal/parser/mineru"
)

// maxDocumentSize 文档最大解析大小（100MB），防止恶意文件导致 OOM。
const maxDocumentSize = 100 * 1024 * 1024

// ParseResult 解析结果（local.ParseResult 的类型别名，保持 parser.ParseResult 向后兼容）。
type ParseResult = local.ParseResult

// Parser 多格式文档解析器。
//
// mineru 为可选的 MinerU 云端引擎，nil 时直接走本地解析。
type Parser struct {
	mineru *mineru.Engine
}

// ParserOption Parser 构造选项。
type ParserOption func(*Parser)

// WithMinerU 注入 MinerU 云端引擎（nil 表示不启用云端解析）。
func WithMinerU(engine *mineru.Engine) ParserOption {
	return func(p *Parser) { p.mineru = engine }
}

// NewParser 创建文档解析器实例。
//
// 可选注入 MinerU 引擎：parser.NewParser(parser.WithMinerU(engine))
func NewParser(opts ...ParserOption) *Parser {
	p := &Parser{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Parse 根据文件类型解析文档，返回 Markdown + 图片。
//
// 解析顺序：
//  1. MinerU 云端引擎（若已配置且 API Key 可用）—— 成功直接返回
//  2. 本地纯 Go 解析（MinerU 失败时降级）
//
// reader 在解析完成后不会关闭，由调用方负责关闭。
// fileType 支持：pdf / docx / xlsx / pptx / md / txt / doc / xls / ppt（大小写不敏感）。
func (p *Parser) Parse(reader io.Reader, fileType string) (*ParseResult, error) {
	// 缓冲全部字节：MinerU 失败后降级到本地需要复用原始数据
	data, err := io.ReadAll(io.LimitReader(reader, maxDocumentSize))
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}
	if len(data) >= maxDocumentSize {
		return nil, fmt.Errorf("文档超过大小上限 %dMB", maxDocumentSize/(1024*1024))
	}

	// 1. MinerU 云端解析（仅富文档走云端，txt/md 纯文本直接本地解析避免无谓往返）
	ft := strings.ToLower(fileType)
	if p.mineru != nil && ft != "txt" && ft != "md" && ft != "markdown" {
		fileName := "document." + ft
		result, err := p.mineru.Parse(bytes.NewReader(data), fileName)
		if err == nil {
			normalizeImagePaths(result)
			return result, nil
		}
		slog.Warn("MinerU 解析失败，降级到本地解析", "file_type", fileType, "error", err)
	}

	// 2. 本地降级
	var result *ParseResult
	switch ft {
	case "txt", "md", "markdown":
		result, err = local.ParseTxt(bytes.NewReader(data))
	case "docx":
		result, err = local.ParseDocx(bytes.NewReader(data))
	case "pdf":
		result, err = local.ParsePDF(bytes.NewReader(data))
	case "xlsx":
		result, err = local.ParseXLSX(bytes.NewReader(data))
	case "pptx":
		result, err = local.ParsePPTX(bytes.NewReader(data))
	case "doc", "xls", "ppt":
		result, err = local.ParseLegacy(bytes.NewReader(data), fileType)
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s（支持: pdf/docx/xlsx/pptx/md/txt/doc/xls/ppt）", fileType)
	}
	if err != nil {
		return nil, err
	}
	normalizeImagePaths(result)
	return result, nil
}

// imageRefRe 匹配 markdown 图片语法 ![](path)，捕获 alt 与 src。
var imageRefRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

// htmlImgSrcRe 匹配 HTML <img> 标签的 src 属性值（兼容单/双引号，RE2 不支持反向引用故用字符类）。
// 捕获：prefix（含开引号）、src 值、suffix（含闭引号与标签尾）。MinerU 等引擎把图片以 HTML <img>
// 形式嵌入（尤其表格内），需与 markdown ![]() 同等归一。
var htmlImgSrcRe = regexp.MustCompile(`(<img\b[^>]*\bsrc=["'])([^"'>]+)(["'][^>]*>)`)

// imageRelPrefix 是图片在文章 markdown 中的相对引用前缀。
// markdown 存于 kb-{kbID}/{draft|published}/article-{id}.md，图片存于桶根 image/ 目录，
// 故相对路径为 ../../image/（上两级回到桶根）。
const imageRelPrefix = "../../image/"

// normalizeImagePaths 归一图片引用与 Images map key，收敛双引擎差异。
//
// 不变式（归一后任意引擎均满足）：markdown 图片引用（![]() 与 HTML <img>）均为 imageRelPrefix{name}，Images[name] 存在。
//   - Images map key → 内容 sha256 前 16 hex + 原扩展名（内容寻址去重 + 全局唯一）。
//   - markdown ![]() 与 HTML <img src> 非 http(s)/data: 引用 → 改写为 imageRelPrefix{新hash名}（按旧名映射）。
//   - 网络/base64 引用不动；MinerU 未导出引用保留原样。
func normalizeImagePaths(r *local.ParseResult) {
	if r == nil || len(r.Images) == 0 {
		return
	}
	// 1. map key 归一为内容 hash + 扩展名；记录 旧 basename → 新名 映射供 markdown 改写
	norm := make(map[string][]byte, len(r.Images))
	rename := make(map[string]string, len(r.Images)) // 旧 basename → 新 hash 名
	for oldKey, data := range r.Images {
		name := hashedName(oldKey, data)
		if _, exists := norm[name]; !exists {
			norm[name] = data
		}
		rename[filepath.Base(oldKey)] = name
	}
	r.Images = norm
	// 2. markdown 图片引用归一（![]() 与 HTML <img> 两类语法均覆盖）
	r.Markdown = rewriteImageRefs(r.Markdown, rename)
}

// hashedName 按图片字节内容生成稳定文件名：sha256 前 16 hex + 原扩展名。
func hashedName(oldKey string, data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8]) + filepath.Ext(filepath.Base(oldKey))
}

// rewriteImageRefs 把 markdown 中指向已知图片的引用统一为 imageRelPrefix{新名}。
// 覆盖两类语法：markdown ![](path) 与 HTML <img src="path">；http(s)/data: 与未命中引用保留原样。
func rewriteImageRefs(markdown string, rename map[string]string) string {
	// 第一遍：markdown ![]() 语法
	markdown = imageRefRe.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := imageRefRe.FindStringSubmatch(match)
		alt, src := sub[1], sub[2]
		if isExternalSrc(src) {
			return match
		}
		if newName, ok := rename[filepath.Base(src)]; ok {
			return fmt.Sprintf("![%s](%s%s)", alt, imageRelPrefix, newName)
		}
		return match
	})
	// 第二遍：HTML <img src="..."> 语法（MinerU 表格内图片常用）
	markdown = htmlImgSrcRe.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := htmlImgSrcRe.FindStringSubmatch(match)
		prefix, src, suffix := sub[1], sub[2], sub[3]
		if isExternalSrc(src) {
			return match
		}
		if newName, ok := rename[filepath.Base(src)]; ok {
			return fmt.Sprintf("%s%s%s%s", prefix, imageRelPrefix, newName, suffix)
		}
		return match
	})
	return markdown
}

// isExternalSrc 判断图片 src 是否为外部引用（网络图床/base64），这类引用不经存储、前端透传。
func isExternalSrc(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "data:")
}
