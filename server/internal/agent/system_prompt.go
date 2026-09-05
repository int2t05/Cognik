// Package agent 提供自建 ReAct 循环与工具接口。
//
// system_prompt.go：系统提示词——静态原则 + 动态注入（KB 列表 + INDEX.md 内容）。
// 对标 Claude Code：系统提示词 = 静态行为规范 + 动态上下文（记忆 + 知识库索引）。
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

3. **Cite sources** — Reference the source file when answering from KB: "According to [filename]...". Each KB has an INDEX.md listing all articles. Use kb(action=get) to read the full article.

4. **Memory** — Use memory(action=recall) to check session/global memory before searching the KB. Use memory(action=remember) to save important findings.

5. **Concise & accurate** — Answer in the user's language. Use code blocks for technical examples. Admit when you don't know.

## Retrieval Priority

memory(recall, session) → memory(recall, global) → kb(search) → web_search → kb(create)

## How to find full articles

Each knowledge base has an INDEX.md (auto-generated) listing all published articles with their slug (filename) and tags. The kb(search) results include article_id in the source field — use kb(action=get, article_id) to read the full article.`

// KBContext 知识库上下文——动态注入到系统提示词。
type KBContext struct {
	ID    int64
	Name  string
	Index string // INDEX.md 内容（文章标题 + slug + tags 列表）
}

// BuildSystemPrompt 构建完整系统提示词 = 静态原则 + 动态 KB 索引 + 全局记忆。
func BuildSystemPrompt(kbs []KBContext, memoryRoot string) string {
	var sb strings.Builder
	sb.WriteString(staticPrompt)

	// 动态注入：知识库列表 + INDEX.md 内容
	if len(kbs) > 0 {
		sb.WriteString("\n\n## Knowledge Bases\n")
		for _, kb := range kbs {
			sb.WriteString(fmt.Sprintf("\n### KB %d: %s\n", kb.ID, kb.Name))
			if kb.Index != "" {
				sb.WriteString(kb.Index)
				sb.WriteString("\n")
			}
		}
	}

	// 动态注入：全局记忆索引（MEMORY.md）
	memoryPath := filepath.Join(memoryRoot, "memory/global/MEMORY.md")
	if data, err := os.ReadFile(memoryPath); err == nil && len(data) > 0 {
		sb.WriteString("\n## Global Memory\n")
		sb.Write(data)
	}

	return sb.String()
}

// LoadKBContext 从文件系统加载 KB 的 INDEX.md 内容。
// indexDir 是 kb-{id} 目录的路径。
func LoadKBContext(kbID int64, kbName, storageRoot, bucket string) KBContext {
	ctx := KBContext{ID: kbID, Name: kbName}
	indexPath := filepath.Join(storageRoot, bucket, fmt.Sprintf("kb-%d", kbID), "INDEX.md")
	if data, err := os.ReadFile(indexPath); err == nil {
		ctx.Index = string(data)
	}
	return ctx
}
