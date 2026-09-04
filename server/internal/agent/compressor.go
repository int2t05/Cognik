// Package agent 提供自建 ReAct 循环与工具接口。
//
// compressor.go：三级上下文压缩管线——每步 LLM 调用前对消息历史压缩，控制上下文膨胀。
// 上下文窗口是 L1 cache，不是 RAM——O(n²) 注意力导致窗口越大性能越差。
//
// 级别 1 HeadAndTail（每轮，无损）：保留系统消息 + 最近 N 轮完整，中间 tool_result 截断为摘要行。
// 级别 2 去重清理（token>70%，无损）：丢弃重复 tool_result 内容（content hash 比对）。
// 级别 3 Autocompact（token>85%，有损）：早期消息批量送 LLM 摘要，替换为单条 system 消息。
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// SummarizeFunc 摘要函数——将早期消息批量摘要为单段文本（autocompact 用）。
// 注入 LLM 调用，避免 compressor 直接依赖 ChatModel。
type SummarizeFunc func(ctx context.Context, messages []*schema.Message) (string, error)

// Compressor 三级上下文压缩器。
type Compressor struct {
	summarize     SummarizeFunc        // autocompact 摘要函数（nil 时跳过 autocompact）
	dedupThresh   float64              // 去重清理触发阈值，默认 0.70
	compactThresh float64              // Autocompact 触发阈值，默认 0.85
	maxTokens     int                  // 上下文窗口 token 上限
	recentKeep    int                  // HeadAndTail 保留的最近消息数
}

// CompressorOption 函数选项模式。
type CompressorOption func(*Compressor)

// WithSummarize 设置 autocompact 摘要函数。
func WithSummarize(fn SummarizeFunc) CompressorOption {
	return func(c *Compressor) { c.summarize = fn }
}

// WithMaxTokens 设置上下文窗口 token 上限。
func WithMaxTokens(n int) CompressorOption {
	return func(c *Compressor) { c.maxTokens = n }
}

// NewCompressor 创建压缩器。
func NewCompressor(maxTokens int, opts ...CompressorOption) *Compressor {
	c := &Compressor{
		dedupThresh:   0.70,
		compactThresh: 0.85,
		maxTokens:     maxTokens,
		recentKeep:    10, // 保留最近 10 条消息完整
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Compress 对消息历史执行三级压缩，返回压缩后的消息。
// 在 Loop 的 drainModelStream 调用前执行。
func (c *Compressor) Compress(ctx context.Context, messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	// 级别 1: HeadAndTail（每轮，无损）——保留系统+最近窗口，中间 tool_result 截断
	messages = c.headAndTail(messages)

	// 级别 2: 去重清理（token > 70%，无损）
	ratio := c.tokenRatio(messages)
	if ratio > c.dedupThresh {
		messages = c.dedupToolResults(messages)
	}

	// 级别 3: Autocompact（token > 85%，有损）
	if ratio > c.compactThresh && c.summarize != nil {
		messages = c.autocompact(ctx, messages)
	}

	return messages
}

// headAndTail 保留系统消息 + 最近 recentKeep 条消息完整，中间 tool_result 截断为摘要行。
func (c *Compressor) headAndTail(messages []*schema.Message) []*schema.Message {
	if len(messages) <= c.recentKeep+1 {
		return messages
	}

	// 找到系统消息边界（开头连续的 system 消息）
	sysEnd := 0
	for i, m := range messages {
		if m.Role != schema.System {
			break
		}
		sysEnd = i + 1
	}

	// 中间区段（系统消息之后、最近窗口之前）
	recentStart := len(messages) - c.recentKeep
	if recentStart < sysEnd {
		return messages // 没有中间区段可压缩
	}

	result := make([]*schema.Message, 0, len(messages))
	// 保留系统消息
	result = append(result, messages[:sysEnd]...)
	// 压缩中间区段的 tool_result
	for _, m := range messages[sysEnd:recentStart] {
		if m.Role == schema.Tool {
			// tool_result 截断为摘要行（保留 ToolCallID 以维持 API 契约）
			truncated := *m
			if len(truncated.Content) > 200 {
				truncated.Content = truncated.Content[:200] + "\n...[已压缩]"
			}
			result = append(result, &truncated)
		} else {
			result = append(result, m)
		}
	}
	// 保留最近窗口
	result = append(result, messages[recentStart:]...)
	return result
}

// dedupToolResults 丢弃重复的 tool_result 内容（content hash 比对，保留首次出现）。
func (c *Compressor) dedupToolResults(messages []*schema.Message) []*schema.Message {
	seen := make(map[string]bool)
	result := make([]*schema.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != schema.Tool {
			result = append(result, m)
			continue
		}
		hash := contentHash(m.Content)
		if seen[hash] {
			// 重复 tool_result 替换为短引用（保留 ToolCallID 维持契约）
			dup := *m
			dup.Content = "[重复内容已省略]"
			result = append(result, &dup)
		} else {
			seen[hash] = true
			result = append(result, m)
		}
	}
	return result
}

// autocompact 将早期消息批量摘要为单条 system 消息，保留最近窗口。
func (c *Compressor) autocompact(ctx context.Context, messages []*schema.Message) []*schema.Message {
	if c.summarize == nil || len(messages) <= c.recentKeep+2 {
		return messages
	}

	// 保留系统消息 + 最近窗口，中间区段送摘要
	sysEnd := 0
	for i, m := range messages {
		if m.Role != schema.System {
			break
		}
		sysEnd = i + 1
	}
	recentStart := len(messages) - c.recentKeep
	if recentStart <= sysEnd {
		return messages
	}

	earlyMsgs := messages[sysEnd:recentStart]
	summary, err := c.summarize(ctx, earlyMsgs)
	if err != nil {
		return messages // 摘要失败不阻塞，返回原消息
	}

	result := make([]*schema.Message, 0, sysEnd+1+c.recentKeep)
	result = append(result, messages[:sysEnd]...)
	result = append(result, schema.SystemMessage(fmt.Sprintf("[对话历史摘要]\n%s", summary)))
	result = append(result, messages[recentStart:]...)
	return result
}

// TokenRatio 估算当前 token 占用比例（rune 近似）。导出供测试与监控用。
func (c *Compressor) TokenRatio(messages []*schema.Message) float64 {
	return c.tokenRatio(messages)
}

// tokenRatio 估算当前 token 占用比例（rune 近似）。
func (c *Compressor) tokenRatio(messages []*schema.Message) float64 {
	if c.maxTokens <= 0 {
		return 0
	}
	tokens := 0
	for _, m := range messages {
		tokens += estimateTokens(m)
	}
	return float64(tokens) / float64(c.maxTokens)
}

// estimateTokens 估算单条消息的 token 数（rune 近似：CJK 1 rune≈1 token，ASCII 4 char≈1 token）。
func estimateTokens(m *schema.Message) int {
	if m == nil {
		return 0
	}
	runes := 0
	for _, r := range m.Content {
		runes++
		if r < 128 {
			runes-- // ASCII 字符减回，用 4:1 近似
		}
	}
	asciiChars := len([]byte(m.Content)) - runes
	return runes + asciiChars/4
}

// contentHash 计算 content 的 SHA256 哈希（去重用）。
func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
