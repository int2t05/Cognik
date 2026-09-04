// Package rag 实现自建 RAG 检索引擎。
//
// chunker.go 实现递归字符文本分块（参考 LangChain ChineseRecursiveTextSplitter）。
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

// Split 将文本分块。先按 Markdown 标题边界切分（保留结构），超长 section 走递归字符分割。
// 分隔符优先级：\n\n → \n → 句号 → … → 空串（rune 滑动窗口）。
func (c *Chunker) Split(text string) []string {
	if len(text) == 0 {
		return nil
	}
	text = normalizeText(text)
	if utf8.RuneCountInString(text) <= c.ChunkSize {
		return []string{text}
	}
	// 先按 Markdown 标题切分，每个 section 带父标题上下文
	sections := splitByMarkdownHeadings(text)
	var chunks []string
	for _, sec := range sections {
		if utf8.RuneCountInString(sec.content) <= c.ChunkSize {
			chunks = append(chunks, sec.contextualized())
			continue
		}
		// 超长 section 走递归字符分割，保留父标题上下文 prepend
		splits := c.splitText(sec.content, separators)
		merged := c.mergeSplits(splits)
		for _, m := range merged {
			chunks = append(chunks, sec.prependContext(m))
		}
	}
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
// Markdown 标题感知分块
// =============================================================================

// markdownSection Markdown 文档的一个标题段落，携带父标题路径作为上下文。
type markdownSection struct {
	headings []string // 父标题路径（如 ["# PostgreSQL 高 CPU", "## 排查步骤"]）
	content  string   // 段落正文（不含标题行）
}

// contextualized 将 headings + content 拼接为完整文本（短 section 直接返回）。
func (s markdownSection) contextualized() string {
	if len(s.headings) == 0 {
		return s.content
	}
	return strings.Join(s.headings, "\n") + "\n" + s.content
}

// prependContext 将父标题路径 prepend 到 chunk 前，保留结构上下文。
func (s markdownSection) prependContext(chunk string) string {
	if len(s.headings) == 0 {
		return chunk
	}
	return strings.Join(s.headings, "\n") + "\n" + chunk
}

// splitByMarkdownHeadings 按 # / ## / ### 标题行切分文档，每个 section 携带父标题路径。
// 无标题的文档返回单个 section（headings 为空）。
func splitByMarkdownHeadings(text string) []markdownSection {
	lines := strings.Split(text, "\n")
	var sections []markdownSection
	var headings []string
	var content []string

	flush := func() {
		if len(content) == 0 && len(sections) > 0 {
			return
		}
		sections = append(sections, markdownSection{
			headings: append([]string{}, headings...),
			content:  strings.TrimSpace(strings.Join(content, "\n")),
		})
		content = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isMarkdownHeading(trimmed) {
			flush()
			level := headingLevel(trimmed)
			title := strings.TrimSpace(trimmed[level:])
			// 标题路径截断到当前层级（# 是 level 1，## 是 level 2）
			if level <= len(headings) {
				headings = headings[:level-1]
			}
			headings = append(headings, strings.Repeat("#", level)+" "+title)
		} else {
			content = append(content, line)
		}
	}
	flush()

	if len(sections) == 0 {
		return []markdownSection{{content: text}}
	}
	return sections
}

// isMarkdownHeading 判断一行是否是 Markdown 标题（1-6 个 # 开头，后跟空格）。
func isMarkdownHeading(line string) bool {
	if line == "" || !strings.HasPrefix(line, "#") {
		return false
	}
	return headingLevel(line) > 0
}

// headingLevel 返回标题层级（1-6），非标题返回 0。
func headingLevel(line string) int {
	level := 0
	for _, c := range line {
		if c == '#' {
			level++
			if level > 6 {
				return 0
			}
		} else {
			break
		}
	}
	if level == 0 || level > len(line) || line[level] != ' ' {
		return 0
	}
	return level
}

// normalizeText 预处理文本：CRLF 归一化、压缩多余空行、全角转半角。
// 保留代码块缩进——围栏 ``` 内不做行内空白压缩，避免破坏代码语义。
func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// 仅压缩 3+ 连续换行为 2 个，保留标题层级间的段落分隔
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	lines := strings.Split(text, "\n")
	inCodeBlock := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 检测围栏代码块开关（``` 或 ~~~）
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue // 代码块内保留原样（含缩进）
		}
		// 代码块外：全角 CJK 标点转半角（BM25 精确匹配用）
		runes := []rune(line)
		for j, r := range runes {
			switch {
			case r == '　':
				runes[j] = ' '
			case r >= '！' && r <= '～':
				runes[j] = r - 0xFEE0
			}
		}
		lines[i] = string(runes)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
