// Package rag 实现自建 RAG 检索引擎。
//
// chunker.go 实现 ChineseRecursiveTextSplitter：
//  1. splitText — 按分隔符优先级递归切分
//  2. mergeSplits — 干净合并至 ≤ chunkSize（不做 overlap）
//  3. addOverlap — 后置追加前后 overlap（前缀+后缀）
//
// 参考：LangChain ChineseRecursiveTextSplitter 核心算法
package rag

import (
	"strings"
	"unicode/utf8"
)

var separators = []string{
	"\n\n", "\n",
	"。", "！", "？",
	".", "!", "?",
	"；", ";",
	"，", ",",
	" ", "",
}

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

func (c *Chunker) Split(text string) []string {
	if len(text) == 0 {
		return nil
	}
	text = normalizeText(text)
	if utf8.RuneCountInString(text) <= c.ChunkSize {
		return []string{text}
	}
	splits := c.splitText(text, separators)
	chunks := c.mergeSplits(splits)
	return c.addOverlap(chunks)
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
// mergeSplits — 干净合并（无 overlap）
// =============================================================================

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
		if len(doc) > 0 && total+n > c.ChunkSize {
			merged = append(merged, strings.Join(doc, ""))
			doc = nil
			total = 0
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
// addOverlap — 前后 overlap 追加
// =============================================================================

func (c *Chunker) addOverlap(chunks []string) []string {
	if c.ChunkOverlap <= 0 || len(chunks) <= 1 {
		return chunks
	}
	result := make([]string, len(chunks))
	for i := range chunks {
		s := chunks[i]
		if i > 0 {
			prev := []rune(chunks[i-1])
			if len(prev) > c.ChunkOverlap {
				s = tail(prev, c.ChunkOverlap) + s
			}
		}
		if i < len(chunks)-1 {
			next := []rune(chunks[i+1])
			if len(next) > c.ChunkOverlap {
				s = s + head(next, c.ChunkOverlap)
			}
		}
		result[i] = s
	}
	return result
}

func tail(runes []rune, n int) string {
	start := len(runes) - n
	for j := start; j > start-n/3 && j > 0; j-- {
		if runes[j] == '\n' {
			start = j + 1
			break
		}
	}
	return string(runes[start:])
}

func head(runes []rune, n int) string {
	end := n
	for j := end; j < end+n/3 && j < len(runes); j++ {
		if runes[j] == '\n' {
			end = j
			break
		}
	}
	return string(runes[:end])
}

// =============================================================================
// normalizeText
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
