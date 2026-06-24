// Package rag 实现自建 RAG 检索引擎。
//
// chunker.go 实现 ChineseRecursiveTextSplitter：
//
//  1. _split_text — 按分隔符优先级递归切分，收集 ≤ chunkSize 的片段
//  2. _merge_splits — 滑动窗口合并至 chunkSize，左侧弹出至 overlap
//
// 参考：LangChain ChineseRecursiveTextSplitter 核心算法
package rag

import (
	"strings"
	"unicode/utf8"
)

// separators 递归分割分隔符优先级（中文优化）。
var separators = []string{
	"\n\n", "\n",
	"。", "！", "？",
	".", "!", "?",
	"；", ";",
	"，", ",",
	" ", "",
}

// Chunker 中文递归文本分割器。
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
}

func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 2
	}
	return &Chunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap}
}

// Split 归一化 → 递归分割 → 滑动窗口合并。
func (c *Chunker) Split(text string) []string {
	if len(text) == 0 {
		return nil
	}
	text = normalizeText(text)
	if utf8.RuneCountInString(text) <= c.ChunkSize {
		return []string{text}
	}
	splits := c.splitText(text, separators)
	return c.mergeSplits(splits)
}

// =============================================================================
// splitText — 递归分割
// =============================================================================

func (c *Chunker) splitText(text string, seps []string) []string {
	if len(seps) == 0 {
		return []string{text}
	}

	sep := seps[0]
	remaining := seps[1:]

	if sep == "" {
		return c.splitByRunes(text)
	}

	parts := strings.Split(text, sep)
	if len(parts) == 1 {
		return c.splitText(text, remaining)
	}

	var good []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if utf8.RuneCountInString(p) <= c.ChunkSize {
			good = append(good, p)
		} else {
			good = append(good, c.splitText(p, remaining)...)
		}
	}
	return good
}

// splitByRunes 字符级硬切分（最后兜底）。
func (c *Chunker) splitByRunes(text string) []string {
	runes := []rune(text)
	if len(runes) <= c.ChunkSize {
		return []string{text}
	}
	step := c.ChunkSize - c.ChunkOverlap
	if step <= 0 {
		step = 1
	}
	var chunks []string
	for i := 0; i < len(runes); i += step {
		end := i + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}

// =============================================================================
// mergeSplits — 滑动窗口合并（核心算法）
// =============================================================================

// mergeSplits 将片段合并到 ≤ chunkSize，并通过左侧弹出产生 overlap。
//
// 算法：
//
//	total = 0
//	doc = []
//	for each split:
//	    if doc not empty AND total + len(split) > chunkSize:
//	        merged.append("".join(doc))
//	        while total > chunkOverlap:
//	            total -= len(doc.pop(0))
//	    doc.append(split); total += len(split)
func (c *Chunker) mergeSplits(splits []string) []string {
	if len(splits) == 0 {
		return nil
	}

	var merged []string
	var doc []string
	total := 0

	for _, s := range splits {
		n := utf8.RuneCountInString(s)
		if n == 0 {
			continue
		}

		// 加这个片段会超 → 先封口当前块，再弹左侧保留 overlap
		if len(doc) > 0 && total+n > c.ChunkSize {
			merged = append(merged, strings.Join(doc, ""))
			for len(doc) > 0 && total > c.ChunkOverlap {
				total -= utf8.RuneCountInString(doc[0])
				doc = doc[1:]
			}
		}

		doc = append(doc, s)
		total += n
	}

	if len(doc) > 0 {
		merged = append(merged, strings.Join(doc, ""))
	}

	return merged
}

// =============================================================================
// normalizeText — 文本归一化
// =============================================================================

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if f := strings.Fields(line); len(f) > 0 {
			lines[i] = strings.Join(f, " ")
		} else {
			lines[i] = ""
		}
	}
	text = strings.Join(lines, "\n")
	// 全角→半角
	runes := []rune(text)
	for i, r := range runes {
		switch {
		case r == '　':
			runes[i] = ' '
		case r >= '！' && r <= '～':
			runes[i] = r - 0xFEE0
		}
	}
	return strings.TrimSpace(string(runes))
}
