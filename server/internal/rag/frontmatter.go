// Package rag 实现自建 RAG 检索引擎。
//
// frontmatter.go：通用 YAML frontmatter 解析（--- 分隔块 → map + 正文）。
// 供 processor 剥离 frontmatter 后分块、knowledge 层解析文章元数据复用。
// 无业务语义，不感知文章类型集合。
package rag

import (
	"strings"
)

// ParseFrontmatter 解析 --- 分隔的 frontmatter，返回字段 map + 正文。
// 无 frontmatter 时返回空 map 与原 content。
// 例：
//
//	---\ntype: guide\ntags: [a, b]\n---\nbody → ({"type":"guide","tags":"[a, b]"}, "body")
func ParseFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)
	// 兼容 \r\n
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return fm, content
	}
	rest := normalized[4:] // 跳过首行 "---\n"
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// 末尾无换行的情况（frontmatter 后直接 EOF）
		if strings.HasSuffix(rest, "\n---") {
			end = len(rest) - 4
		} else {
			return fm, content
		}
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		fm[key] = val
	}
	body := strings.TrimSpace(rest[end+5:]) // 跳过 "\n---\n"
	return fm, body
}

// StripFrontmatter 返回剥离 frontmatter 后的正文（含 # 标题等）。
// processor 分块前调用：chunk 仅作用于正文，frontmatter 元数据不参与向量。
func StripFrontmatter(content string) string {
	_, body := ParseFrontmatter(content)
	return body
}
