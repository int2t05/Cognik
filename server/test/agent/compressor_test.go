// Package agent_test 验证 Agent 上下文压缩器（Compressor）的核心行为。
// 三级管线：HeadAndTail（无损）→ 去重清理（无损）→ Autocompact（有损）。
package agent_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"opsmind/internal/agent"
)

// newCompressor 创建测试用压缩器。
func newCompressor(maxTokens int, summarize agent.SummarizeFunc) *agent.Compressor {
	return agent.NewCompressor(maxTokens,
		agent.WithSummarize(summarize),
		agent.WithMaxTokens(maxTokens),
	)
}

// makeMessages 构造 N 条消息（1 条 system + N-1 条 user）。
func makeMessages(n int) []*schema.Message {
	msgs := []*schema.Message{schema.SystemMessage("你是运维助手")}
	for i := 1; i < n; i++ {
		msgs = append(msgs, schema.UserMessage(strings.Repeat("内容", 10)))
	}
	return msgs
}

// --- HeadAndTail ---

func TestCompressor_HeadAndTailShortMessages(t *testing.T) {
	c := newCompressor(100000, nil)
	msgs := makeMessages(5) // 5 条消息，不超 recentKeep
	result := c.Compress(context.Background(), msgs)
	if len(result) != len(msgs) {
		t.Errorf("短消息不应压缩，得到 %d 条（原 %d）", len(result), len(msgs))
	}
}

func TestCompressor_HeadAndTailLongMessages(t *testing.T) {
	c := newCompressor(100000, nil)
	msgs := makeMessages(30) // 超过 recentKeep(10)
	result := c.Compress(context.Background(), msgs)

	// 系统消息应保留
	if result[0].Role != schema.System {
		t.Error("系统消息应保留在开头")
	}
	// 消息数不变（HeadAndTail 无损，只截断 tool_result 内容）
	if len(result) != len(msgs) {
		t.Errorf("HeadAndTail 无损，消息数应不变，得到 %d（原 %d）", len(result), len(msgs))
	}
}

func TestCompressor_HeadAndTailTruncatesToolResults(t *testing.T) {
	c := newCompressor(100000, nil)
	// 构造 15 条消息，中间含 tool_result
	msgs := []*schema.Message{
		schema.SystemMessage("系统提示"),
		schema.UserMessage("用户问题1"),
		schema.AssistantMessage("助手回答1", nil),
		schema.ToolMessage("很长的工具结果"+strings.Repeat("x", 300), "tc-1"),
		schema.UserMessage("用户问题2"),
		schema.AssistantMessage("助手回答2", nil),
		schema.ToolMessage("另一个长结果"+strings.Repeat("y", 300), "tc-2"),
		schema.UserMessage("用户问题3"),
		schema.AssistantMessage("助手回答3", nil),
		schema.ToolMessage("第三个工具结果"+strings.Repeat("z", 300), "tc-3"),
		schema.UserMessage("用户问题4"),
		schema.AssistantMessage("助手回答4", nil),
		schema.UserMessage("用户问题5"),
		schema.AssistantMessage("助手回答5", nil),
		schema.UserMessage("用户问题6"),
	}
	result := c.Compress(context.Background(), msgs)

	// 验证中间 tool_result 被截断
	truncatedFound := false
	for _, m := range result {
		if m.Role == schema.Tool && strings.Contains(m.Content, "已压缩") {
			truncatedFound = true
			break
		}
	}
	if !truncatedFound {
		t.Error("中间 tool_result 应被截断为带 [已压缩] 标记")
	}

	// 最近窗口的 tool_result 不应被截断
	lastTool := result[len(result)-1]
	for i := len(result) - 1; i >= 0; i-- {
		if result[i].Role == schema.Tool {
			lastTool = result[i]
			break
		}
	}
	if strings.Contains(lastTool.Content, "已压缩") {
		t.Error("最近窗口的 tool_result 不应被截断")
	}
}

// --- 去重清理 ---

func TestCompressor_DedupToolResults(t *testing.T) {
	c := newCompressor(30, nil) // 极小 maxTokens 触发 >70%

	dupContent := "重复的工具结果内容"
	msgs := []*schema.Message{
		schema.SystemMessage("系统"),
		schema.AssistantMessage("回答", nil),
		schema.ToolMessage(dupContent, "tc-1"),
		schema.UserMessage("问题2"),
		schema.AssistantMessage("回答2", nil),
		schema.ToolMessage(dupContent, "tc-2"), // 重复
	}
	result := c.Compress(context.Background(), msgs)

	// 第二个重复 tool_result 应被替换为 "[重复内容已省略]"
	dupFound := false
	for _, m := range result {
		if m.Role == schema.Tool && m.Content == "[重复内容已省略]" {
			dupFound = true
		}
	}
	if !dupFound {
		t.Error("重复 tool_result 应被替换为 [重复内容已省略]")
	}
}

// --- Autocompact ---

func TestCompressor_Autocompact(t *testing.T) {
	summarizeCalled := false
	summarize := func(ctx context.Context, msgs []*schema.Message) (string, error) {
		summarizeCalled = true
		return "对话历史摘要：用户询问了 PG CPU 排障", nil
	}
	c := newCompressor(100, summarize) // 极小 maxTokens 触发 >85%

	msgs := makeMessages(20)
	result := c.Compress(context.Background(), msgs)

	if !summarizeCalled {
		t.Error("autocompact 应被触发调用 summarize")
	}
	// 应含摘要 system 消息
	hasSummary := false
	for _, m := range result {
		if m.Role == schema.System && strings.Contains(m.Content, "对话历史摘要") {
			hasSummary = true
		}
	}
	if !hasSummary {
		t.Error("压缩后应含对话历史摘要 system 消息")
	}
	// 结果应比原始消息少（早期消息被摘要替换）
	if len(result) >= len(msgs) {
		t.Errorf("autocompact 应减少消息数，得到 %d（原 %d）", len(result), len(msgs))
	}
}

func TestCompressor_AutocompactNoSummarize(t *testing.T) {
	c := newCompressor(100, nil) // 无 summarize 函数
	msgs := makeMessages(20)
	result := c.Compress(context.Background(), msgs)
	// 无 summarize 时 autocompact 跳过，不报错
	if result == nil {
		t.Error("无 summarize 时应返回消息不报错")
	}
}

// --- tokenRatio ---

func TestCompressor_TokenRatio(t *testing.T) {
	c := newCompressor(1000, nil)
	msgs := []*schema.Message{
		schema.SystemMessage(strings.Repeat("a", 500)), // ~125 tokens
		schema.UserMessage(strings.Repeat("中", 300)),  // ~300 tokens
	}
	// 总 token 约 425，maxTokens=1000，ratio ≈ 0.425
	ratio := c.TokenRatio(msgs)
	if ratio <= 0 || ratio > 1 {
		t.Errorf("tokenRatio 应在 (0,1]，得到 %.3f", ratio)
	}
}

// --- 空消息 ---

func TestCompressor_EmptyMessages(t *testing.T) {
	c := newCompressor(1000, nil)
	result := c.Compress(context.Background(), nil)
	if result != nil {
		t.Errorf("空消息应返回 nil，得到 %d 条", len(result))
	}
}
