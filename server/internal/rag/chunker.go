// Package rag 实现自建 RAG 检索引擎。
//
// chunker.go 实现 RecursiveCharacterTextSplitter（递归字符文本分割器）。
//
// 分割策略按优先级依次尝试：
//
//	\n\n (段落) → \n (行) → 。(中文句号) → . (英文句号) → 空格 → 字符级
//
// 为什么不用固定大小切分：
// 固定大小切分会在句子中间截断，破坏语义完整性。
// 递归分割优先在自然边界（段落、句子）处切分，
// 在无法找到合适分隔符时才降级到字符级硬切分，
// 这样能最大程度保持分块的语义独立性。
//
// chunk_size=1000 / overlap=200 的选择依据：
// 1000 字符约为 400-500 个中文 token，在 bge-m3 的 512 token 上下文窗口内。
// overlap=200（20%）保证相邻分块之间的上下文连续性，避免关键信息落在边界处丢失。
package rag

import (
	"strings"
	"unicode/utf8"
)

// Chunker 递归字符文本分割器。
type Chunker struct {
	ChunkSize    int // 目标分块大小（字符数）
	ChunkOverlap int // 相邻分块重叠大小（字符数）
}

// NewChunker 创建分块器实例。
//
// chunkSize 为目标分块大小（字符数），chunkOverlap 为重叠量。
// 如果 chunkOverlap ≥ chunkSize，将自动 clamp 到 chunkSize/2，
// 避免产生 O(N²) 级别的海量重叠分块。
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 1000 // 默认分块大小
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 2
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

// Split 将文本按递归优先级分割为分块列表。
//
// 空字符串返回空切片。分割前对文本做归一化处理（全角→半角、多余空白合并），
// 提升 BM25 分词品质和 embedding 稳定性。
func (c *Chunker) Split(text string) []string {
	if len(text) == 0 {
		return nil
	}

	text = normalizeText(text)

	// 文本不超过 chunkSize 时直接返回
	if utf8.RuneCountInString(text) <= c.ChunkSize {
		return []string{text}
	}

	// 按分隔符优先级递归分割
	separators := []string{"\n\n", "\n", "。", ".", " ", ""}
	chunks := c.splitRecursive(text, separators)
	return chunks
}

// normalizeText 对文本做归一化处理。
//
// 为什么需要归一化：
// 全角字符在半角环境下分词效果差，连续空白浪费 token 预算，
// 统一处理后 BM25 分词更一致，embedding 向量更稳定。
func normalizeText(text string) string {
	// 统一换行符：\r\n → \n，\r → \n
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 合并连续空白行（>2 个连续换行缩减为 2 个）
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	// 合并连续水平空白（空格/制表符缩为单个空格）
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			lines[i] = strings.Join(fields, " ")
		} else {
			lines[i] = ""
		}
	}
	text = strings.Join(lines, "\n")

	// 全角字母数字→半角（中文环境常见复制粘贴问题）
	text = normalizeFullwidth(text)

	return strings.TrimSpace(text)
}

// normalizeFullwidth 将全角 ASCII 字符转换为半角。
//
// 全角范围 FF01-FF5E 对应半角 21-7E（偏移 0xFEE0）。
// 全角空格 3000 → 半角空格 0020。
func normalizeFullwidth(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == '　':
			runes[i] = ' '
		case r >= '！' && r <= '～':
			runes[i] = r - 0xFEE0
		}
	}
	return string(runes)
}

// splitRecursive 按分隔符递归分割文本。
//
// 对给定分隔符列表依次尝试，找到能分割出至少 2 段的第一个分隔符，
// 然后对每一段递归处理（使用该分隔符及其后续分隔符）。
// 最后一个分隔符 "" 表示字符级硬切分。
func (c *Chunker) splitRecursive(text string, separators []string) []string {
	if len(separators) == 0 {
		return []string{text}
	}

	sep := separators[0]
	remainingSeps := separators[1:]

	var splits []string
	if sep == "" {
		splits = c.splitByRunes(text)
	} else {
		parts := strings.Split(text, sep)
		if len(parts) == 1 {
			return c.splitRecursive(text, remainingSeps)
		}
		for _, part := range parts {
			if len(part) == 0 {
				continue
			}
			if utf8.RuneCountInString(part) <= c.ChunkSize {
				splits = append(splits, part)
			} else {
				splits = append(splits, c.splitRecursive(part, remainingSeps)...)
			}
		}
	}

	return c.mergeSplits(splits)
}

// splitByRunes 按字符硬切分，保证 overlap 重叠。
func (c *Chunker) splitByRunes(text string) []string {
	runes := []rune(text)
	if len(runes) <= c.ChunkSize {
		return []string{text}
	}

	var chunks []string
	step := c.ChunkSize - c.ChunkOverlap
	if step <= 0 {
		step = 1
	}

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

// mergeSplits 将递归分割得到的小片段合并到目标大小附近。
//
// 为什么需要合并：递归分割可能产生很多小块（特别是按句号分割时），
// 合并后可以控制在 chunkSize 附近，减少 embedding API 调用次数。
//
// 合并时保留 chunkOverlap：每个合并块尾部与下一块的头部有重叠，
// 从上一块的尾部截取 overlap 长度的字符作为下一块的前缀。
func (c *Chunker) mergeSplits(splits []string) []string {
	if len(splits) <= 1 {
		return splits
	}

	var merged []string
	current := ""

	for _, s := range splits {
		if current == "" {
			current = s
			continue
		}

		combined := current + s
		if utf8.RuneCountInString(combined) <= c.ChunkSize {
			current = combined
		} else {
			merged = append(merged, current)

			// 从当前块尾部截取 overlap 作为下一块的前缀
			if c.ChunkOverlap > 0 {
				runes := []rune(current)
				if len(runes) > c.ChunkOverlap {
					current = string(runes[len(runes)-c.ChunkOverlap:]) + s
					// 重叠拼接后可能超过 chunkSize，截断防止超大块
					if utf8.RuneCountInString(current) > c.ChunkSize {
						cr := []rune(current)
						current = string(cr[:c.ChunkSize])
					}
					continue
				}
			}
			current = s
		}
	}

	if current != "" {
		merged = append(merged, current)
	}

	// 如果合并后只有 1 个分块但长度仍超限，退回原始分割
	if len(merged) == 1 && utf8.RuneCountInString(merged[0]) > c.ChunkSize {
		return splits
	}

	return merged
}
