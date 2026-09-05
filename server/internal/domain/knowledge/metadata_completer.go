// Package knowledge 知识库领域业务逻辑、数据访问与 HTTP 处理。
//
// metadata_completer.go：发布时 LLM 补全文章元数据（type/tags）。
// 复用 rag.LLMContextualGenerator 的 modelGetter 模式（ChatModel，零锁热切换）。
// 仅补全字段，不修改正文——用户明确要求"不要 LLM 补全文章内容，只补全字段"。
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"cognos/internal/agent/llm"
)

// MetadataCompleter 发布时补全文章元数据（type/tags），不修改正文。
type MetadataCompleter interface {
	// Complete 根据标题+正文推断缺失字段，与 existing 合并返回。已完整则不调 LLM。
	Complete(ctx context.Context, title, content string, existing ArticleMeta) (ArticleMeta, error)
}

// LLMMetadataCompleter 基于 ChatModel 的元数据补全实现。
// modelGetter 复用 agent.ChatModelFactory.GetModel（热切换零锁读）。
type LLMMetadataCompleter struct {
	modelGetter func() *llm.ChatModel
}

// NewLLMMetadataCompleter 创建 LLM 元数据补全器。
func NewLLMMetadataCompleter(modelGetter func() *llm.ChatModel) *LLMMetadataCompleter {
	return &LLMMetadataCompleter{modelGetter: modelGetter}
}

// completionResult LLM 返回的 JSON 结构。
type completionResult struct {
	Type string   `json:"type"`
	Tags []string `json:"tags"`
}

// Complete 推断缺失 type/tags 并与 existing 合并。LLM 不可用/失败时返回 existing（调用方降级 guide）。
func (c *LLMMetadataCompleter) Complete(ctx context.Context, title, content string, existing ArticleMeta) (ArticleMeta, error) {
	// 已完整（type 合法 + tags 非空）则跳过 LLM
	if IsAllowedType(existing.Type) && len(existing.Tags) > 0 {
		return existing, nil
	}
	if c == nil || c.modelGetter == nil {
		return existing, nil
	}
	m := c.modelGetter()
	if m == nil {
		return existing, nil
	}

	// 截断正文避免 token 膨胀
	bodyRunes := []rune(content)
	if len(bodyRunes) > 2000 {
		content = string(bodyRunes[:2000])
	}

	prompt := fmt.Sprintf(`你是文章分类器。根据标题和正文，从 [%s] 中选最合适的类型，并推荐 1-5 个标签。
仅返回 JSON，不要任何解释或代码块标记：{"type":"...","tags":["..."]}
标题：%s
正文：%s`, strings.Join(AllowedArticleTypes, "/"), title, content)

	resp, err := m.Generate(ctx, []*llm.Message{
		llm.SystemMessage(prompt),
		llm.UserMessage("输出 JSON"),
	})
	if err != nil {
		slog.Warn("元数据补全 LLM 调用失败", "title", title, "error", err)
		return existing, err
	}

	inferred := parseCompletionJSON(resp.Content)
	result := existing
	if !IsAllowedType(result.Type) {
		result.Type = inferred.Type
	}
	if len(result.Tags) == 0 {
		result.Tags = inferred.Tags
	}
	// LLM 仍返回非法 type → 最终降级 guide（调用方兜底）
	if !IsAllowedType(result.Type) {
		result.Type = "guide"
	}
	return result, nil
}

// parseCompletionJSON 从 LLM 输出提取 JSON（兼容代码块/散文包裹）。
func parseCompletionJSON(raw string) completionResult {
	var r completionResult
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return r
	}
	jsonStr := raw[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), &r); err != nil {
		slog.Warn("元数据补全 JSON 解析失败", "raw", raw, "error", err)
		return completionResult{}
	}
	return r
}
