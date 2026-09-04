// Package agent 提供自建 ReAct 循环与工具接口。
//
// compressor.go：五级上下文压缩管线——每步 LLM 调用前对消息历史压缩。
// 参考 Claude Code 思想：从最便宜/最无损到最贵/最有损逐级递进，每级检查上一级是否已降量。
//
// 级别 1 Tool Result Budget（每轮）：单条 tool_result 超限截断为占位。
// 级别 2 Microcompact（每轮）：按 tool_use ID 清理旧 tool_result（保留 tool_use 记录）。
// 级别 3 HeadAndTail（每轮，无损）：保留系统+最近窗口，中间 tool_result 截断。
// 级别 4 去重清理（token>70%，无损）：丢弃重复 tool_result。
// 级别 5 Autocompact（token>85%，有损）：早期消息 LLM 摘要。
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/schema"
)

// 可压缩工具白名单——低价值工具结果可被 Microcompact 清理。
// kb(search)/memory(recall)/dispatch_subagent 不可压缩（决策依据）。
var compactableTools = map[string]bool{
	"read_file": true, "bash": true, "grep": true, "glob": true,
	"list_dir": true, "web_search": true, "web_fetch": true,
	"write_file": true, "edit_file": true,
}

// SummarizeFunc 摘要函数——将早期消息批量摘要为单段文本（autocompact 用）。
type SummarizeFunc func(ctx context.Context, messages []*schema.Message) (string, error)

// Compressor 五级上下文压缩器。
type Compressor struct {
	summarize       SummarizeFunc // autocompact 摘要函数（nil 时跳过）
	dedupThresh     float64       // 去重清理触发阈值，默认 0.70
	compactThresh   float64       // Autocompact 触发阈值，默认 0.85
	maxTokens       int           // 上下文窗口 token 上限
	recentKeep      int           // 保留的最近消息数
	toolResultLimit int           // 单条 tool_result 截断上限（级别 1）
	maxCompactFails int           // 熔断器：连续 autocompact 失败上限
	compactFailCount int          // 连续失败计数
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
		dedupThresh:      0.70,
		compactThresh:    0.85,
		maxTokens:        maxTokens,
		recentKeep:       10,
		toolResultLimit:  2000, // 单条 tool_result 超过 2000 字符截断
		maxCompactFails:  3,    // 熔断器：连续 3 次失败停止重试
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Compress 对消息历史执行五级压缩，返回压缩后的消息。
// 在 Loop 的 drainModelStream 调用前执行。
func (c *Compressor) Compress(ctx context.Context, messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	// 级别 1: Tool Result Budget——单条 tool_result 超限截断为占位
	messages = c.toolResultBudget(messages)

	// 级别 2: Microcompact——按 tool_use ID 清理旧 tool_result（保留 tool_use 记录）
	messages = c.microcompact(messages)

	// 级别 3: HeadAndTail——保留系统+最近窗口，中间 tool_result 截断
	messages = c.headAndTail(messages)

	// 检查是否还需降量
	ratio := c.tokenRatio(messages)

	// 级别 4: 去重清理（token > 70%，无损）
	if ratio > c.dedupThresh {
		messages = c.dedupToolResults(messages)
		ratio = c.tokenRatio(messages) // 重新计算
	}

	// 级别 5: Autocompact（token > 85%，有损）——熔断器保护
	if ratio > c.compactThresh && c.summarize != nil && c.compactFailCount < c.maxCompactFails {
		messages = c.autocompact(ctx, messages)
	}

	return messages
}

// toolResultBudget 单条 tool_result 内容超限时截断为占位（级别 1）。
// 保留 ToolCallID 维持 API 契约，只替换内容。
func (c *Compressor) toolResultBudget(messages []*schema.Message) []*schema.Message {
	result := make([]*schema.Message, len(messages))
	for i, m := range messages {
		if m.Role == schema.Tool && len(m.Content) > c.toolResultLimit {
			truncated := *m
			truncated.Content = m.Content[:c.toolResultLimit] + "\n...[tool result budget 截断]"
			result[i] = &truncated
		} else {
			result[i] = m
		}
	}
	return result
}

// microcompact 按 tool_use ID 清理旧 tool_result（级别 2）。
// 保留最近 recentKeep 条消息中的 tool_result 完整，更早的 tool_result 清空内容但保留 tool_use 记录。
// 模型仍知道调过什么工具，只是看不到旧结果——参考 Claude Code microCompact 思想。
func (c *Compressor) microcompact(messages []*schema.Message) []*schema.Message {
	if len(messages) <= c.recentKeep+1 {
		return messages
	}

	// 系统消息边界
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

	result := make([]*schema.Message, len(messages))
	copy(result, messages)

	// 中间区段的 tool_result 清空内容（仅可压缩工具）
	for i := sysEnd; i < recentStart; i++ {
		m := result[i]
		if m.Role == schema.Tool && isCompactableTool(m.ToolName) && len(m.Content) > 200 {
			cleared := *m
			cleared.Content = "[旧工具结果已清理]"
			result[i] = &cleared
		}
	}
	return result
}

// isCompactableTool 判断工具是否可压缩（决策依据工具不可压缩）。
func isCompactableTool(toolName string) bool {
	return compactableTools[toolName]
}

// headAndTail 保留系统消息 + 最近 recentKeep 条消息完整，中间 tool_result 截断（级别 3）。
func (c *Compressor) headAndTail(messages []*schema.Message) []*schema.Message {
	if len(messages) <= c.recentKeep+1 {
		return messages
	}

	sysEnd := 0
	for i, m := range messages {
		if m.Role != schema.System {
			break
		}
		sysEnd = i + 1
	}

	recentStart := len(messages) - c.recentKeep
	if recentStart < sysEnd {
		return messages
	}

	result := make([]*schema.Message, 0, len(messages))
	result = append(result, messages[:sysEnd]...)
	for _, m := range messages[sysEnd:recentStart] {
		if m.Role == schema.Tool {
			truncated := *m
			if len(truncated.Content) > 200 {
				truncated.Content = truncated.Content[:200] + "\n...[已压缩]"
			}
			result = append(result, &truncated)
		} else {
			result = append(result, m)
		}
	}
	result = append(result, messages[recentStart:]...)
	return result
}

// dedupToolResults 丢弃重复的 tool_result 内容（级别 4，content hash 比对，保留首次出现）。
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

// autocompact 将早期消息批量摘要为单条 system 消息（级别 5，有损）。
// 熔断器：失败时递增计数，达到上限停止后续重试。
func (c *Compressor) autocompact(ctx context.Context, messages []*schema.Message) []*schema.Message {
	if c.summarize == nil || len(messages) <= c.recentKeep+2 {
		return messages
	}

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
		c.compactFailCount++
		slog.Warn("autocompact 摘要失败", "consecutive_fails", c.compactFailCount, "error", err)
		return messages
	}

	// 成功则重置熔断器
	c.compactFailCount = 0

	result := make([]*schema.Message, 0, sysEnd+1+c.recentKeep)
	result = append(result, messages[:sysEnd]...)
	result = append(result, schema.SystemMessage(fmt.Sprintf("[对话历史摘要]\n%s", summary)))
	result = append(result, messages[recentStart:]...)
	return result
}

// TokenRatio 估算当前 token 占用比例。导出供测试与监控用。
func (c *Compressor) TokenRatio(messages []*schema.Message) float64 {
	return c.tokenRatio(messages)
}

// tokenRatio 估算 token 占用比例（rune 近似）。
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
			runes--
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
