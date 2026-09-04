// Package rag 实现自建 RAG 检索引擎。
//
// contextual.go：Contextual Retrieval——索引时为每个 chunk 生成 LLM 上下文摘要 prepend。
// Anthropic 实证：基线失败率 5.7% → +Contextual 2.9% → +Rerank 1.9%（-67%）。
package rag

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// ContextualGenerator 为 chunk 生成上下文摘要（索引时调用，非检索时）。
type ContextualGenerator interface {
	// Generate 为单个 chunk 生成 1-2 句上下文摘要（基于文档全文上下文）。
	Generate(ctx context.Context, docContext, chunk string) (string, error)
}

// LLMContextualGenerator 基于 LLM 的 Contextual Retrieval 实现。
type LLMContextualGenerator struct {
	modelGetter func() *openai.ChatModel
}

// NewLLMContextualGenerator 创建 LLM 上下文生成器。
func NewLLMContextualGenerator(modelGetter func() *openai.ChatModel) *LLMContextualGenerator {
	return &LLMContextualGenerator{modelGetter: modelGetter}
}

// Generate 为 chunk 生成上下文摘要。
func (g *LLMContextualGenerator) Generate(ctx context.Context, docContext, chunk string) (string, error) {
	if g == nil || g.modelGetter == nil {
		return "", nil
	}
	m := g.modelGetter()
	if m == nil {
		return "", fmt.Errorf("ChatModel 未初始化")
	}

	prompt := fmt.Sprintf(`<document>
%s
</document>
Here is the chunk we want to situate within the whole document:
<chunk>
%s
</chunk>
Please give a short succinct context to situate this chunk within the overall document for the purposes of improving search retrieval of the chunk. Answer only with the succinct context and nothing else.`, docContext, chunk)

	msgs := []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage("生成上下文摘要。"),
	}
	resp, err := m.Generate(ctx, msgs)
	if err != nil {
		slog.Warn("Contextual Retrieval 生成失败", "error", err)
		return "", err
	}
	return resp.Content, nil
}

// GenerateContextualPrefixes 为一批 chunk 生成上下文前缀。
// docContext 是文档摘要（截断到 2000 字符）。
// 失败的 chunk 跳过（不影响其他 chunk）。
func GenerateContextualPrefixes(ctx context.Context, gen ContextualGenerator, docContext string, chunks []string) []string {
	if gen == nil {
		return chunks
	}
	// 截断文档上下文避免 token 膨胀
	if len([]rune(docContext)) > 2000 {
		docContext = string([]rune(docContext)[:2000])
	}

	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		prefix, err := gen.Generate(ctx, docContext, chunk)
		if err != nil || prefix == "" {
			result[i] = chunk
			continue
		}
		result[i] = prefix + "\n\n" + chunk
	}
	return result
}
