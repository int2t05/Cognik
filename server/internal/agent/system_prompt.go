// Package agent 提供自建 ReAct 循环与工具接口。
//
// system_prompt.go：系统提示词——静态原则 + 动态注入（KB 摘要 + 全局记忆）。
// 对标 Claude Code：系统提示词 = 静态行为规范 + 动态上下文（记忆 + 知识库摘要）。
// 渐进式披露：系统提示词只注入 KB 摘要（不注入完整文章列表），Agent 按需调 kb(action=list) 获取。
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// staticPrompt 静态行为规范——通用原则，不随运行时变化。
// 工具列表通过 Eino WithTools() 动态绑定给 LLM，不在 prompt 里硬编码。
const staticPrompt = `You are Cognos, a knowledge management assistant for teams and individuals.

## Core Principles

1. **Knowledge first** — When users ask about any topic, search the knowledge base before answering. Only use your own knowledge if the KB has no relevant articles.

2. **Search → Fetch → Write loop** — If the KB doesn't have relevant content, use web_search to find answers, then use kb(action=create) to write findings back for future retrieval.

3. **Cite sources** — Reference the source file when answering from KB: "According to [filename]...". Use kb(action=get, article_id) to read a full article, or kb(action=get, article_ids=[...]) for batch summaries.

4. **Memory** — Use memory(action=recall) to check session/global memory before searching the KB. Use memory(action=remember) to save important findings.

5. **Concise & accurate** — Answer in the user's language. Use code blocks for technical examples. Admit when you don't know.

## Retrieval Priority

memory(recall, session) → memory(recall, global) → kb(search) → web_search → kb(create)

## Progressive Disclosure

- System prompt contains KB summaries only (not full article lists).
- Use kb(action=list, kb_id, limit, offset) to browse articles (paginated, default 20 per page).
- Use kb(action=search, kb_id, query) to find relevant chunks via RAG.
- Use kb(action=get, article_id) to read one full article.
- Use kb(action=get, article_ids=[1,2,3]) to read batch summaries (first 500 chars each).`

// KBSummary 知识库摘要（注入系统提示词）。
type KBSummary struct {
	ID           int64
	Name         string
	ArticleCount int64
	TypeCounts   map[string]int // type → count
}

// BuildSystemPrompt 构建完整系统提示词 = 静态原则 + 动态 KB 摘要 + 全局记忆。
func BuildSystemPrompt(summaries []KBSummary, memoryRoot string) string {
	var sb strings.Builder
	sb.WriteString(staticPrompt)

	// 动态注入：知识库摘要（不注入完整文章列表，Agent 按需调 kb(list)）
	if len(summaries) > 0 {
		sb.WriteString("\n\n## Knowledge Bases\n")
		for _, kb := range summaries {
			sb.WriteString(fmt.Sprintf("- kb_id=%d: %s (%d articles", kb.ID, kb.Name, kb.ArticleCount))
			if len(kb.TypeCounts) > 0 {
				sb.WriteString(", types: ")
				first := true
				for t, c := range kb.TypeCounts {
					if !first {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%s=%d", t, c))
					first = false
				}
			}
			sb.WriteString(")\n")
		}
		sb.WriteString("\nUse kb(action=list, kb_id, limit, offset) to browse articles.")
	}

	// 动态注入：全局记忆索引（MEMORY.md）
	memoryPath := filepath.Join(memoryRoot, "memory/global/MEMORY.md")
	if data, err := os.ReadFile(memoryPath); err == nil && len(data) > 0 {
		sb.WriteString("\n## Global Memory\n")
		sb.Write(data)
	}

	return sb.String()
}
