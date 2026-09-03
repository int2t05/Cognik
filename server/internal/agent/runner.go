// Package agent 提供 Agent Loop 基座。
// runner.go：AgentRunner 跑 ReAct 循环 → 事件 channel（生产者）。
//
// Stream 返回 <-chan AgentEvent。内部 detached goroutine 跑 ReAct 循环：
//   - 主流（agent.Stream 的 StreamReader）→ 最终回答 token + reasoning
//   - MessageFuture 的 GetMessageStreams → 中间消息（thinking/tool_call/tool_result）
//
// 生产者与交付渠道（runtime.Gateway）解耦：runner 只产出事件，由 chat 层订阅并 Publish 到网关。
// detached ctx 保证客户端断开后生成继续跑完并落库。
package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// AgentRunner 跑 ReAct 循环并产出事件流（生产者）。
type AgentRunner struct {
	factory *AgentFactory
}

// NewAgentRunner 创建 runner。
func NewAgentRunner(factory *AgentFactory) *AgentRunner {
	return &AgentRunner{factory: factory}
}

// Stream 跑 ReAct 循环，返回事件 channel。
// ctx 应为 detached（带超时）— 客户端断开不停止生成。
func (r *AgentRunner) Stream(ctx context.Context, input []*schema.Message) (<-chan AgentEvent, error) {
	ag, err := r.factory.NewAgent(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan AgentEvent, 256)
	opt, future := react.WithMessageFuture() // 中间消息 future

	// 生产者 1：排空 MessageFuture 的流式中间消息 → thinking/tool_call/tool_result。
	// 注意：agent.Stream 用 GetMessageStreams（返回 *schema.StreamReader 的迭代器）。
	go func() {
		streamIter := future.GetMessageStreams()
		for {
			sr, ok, err := streamIter.Next()
			if err != nil {
				slog.Warn("MessageFuture 流读取错误", "error", err)
				return
			}
			if !ok {
				return // 中间消息流结束
			}
			r.drainMessageStream(sr, out)
		}
	}()

	// 生产者 2：排空主流 StreamReader → 最终回答 token + reasoning。
	go func() {
		defer close(out)
		reader, err := ag.Stream(ctx, input, opt)
		if err != nil {
			emit(out, AgentEvent{Type: EventError, Error: err.Error()})
			return
		}
		defer reader.Close()
		for {
			msg, err := reader.Recv()
			if errors.Is(err, io.EOF) {
				emit(out, AgentEvent{Type: EventDone})
				return
			}
			if err != nil {
				emit(out, AgentEvent{Type: EventError, Error: err.Error()})
				return
			}
			if msg.ReasoningContent != "" {
				emit(out, AgentEvent{Type: EventReasoning, Content: msg.ReasoningContent})
			}
			if msg.Content != "" {
				emit(out, AgentEvent{Type: EventToken, Content: msg.Content})
			}
		}
	}()

	return out, nil
}

// drainMessageStream 排空一个中间消息流，按角色/字段分发为 tool_call/tool_result/reasoning 事件。
func (r *AgentRunner) drainMessageStream(sr *schema.StreamReader[*schema.Message], out chan<- AgentEvent) {
	defer sr.Close()
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			slog.Warn("中间消息流读取错误", "error", err)
			return
		}
		// tool_result：assistant 收到工具执行结果（Role=Tool）
		if msg.Role == schema.Tool {
			if msg.Content != "" {
				emit(out, AgentEvent{Type: EventToolResult, Content: msg.Content, ID: msg.ToolCallID, Label: msg.ToolName})
			}
			continue
		}
		// tool_call：agent 决定调用工具（Assistant 消息含 ToolCalls）
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				emit(out, AgentEvent{Type: EventToolCall, ID: tc.ID, Label: tc.Function.Name, Content: tc.Function.Arguments})
			}
			continue
		}
		// reasoning：思考内容
		if msg.ReasoningContent != "" {
			emit(out, AgentEvent{Type: EventReasoning, Content: msg.ReasoningContent})
		}
	}
}

// emit 非阻塞发送：channel 满则丢弃（慢消费者），生成 goroutine 始终跑完 + 落库。
// 与 runtime.Gateway.Publish 的非阻塞扇出语义一致（慢订阅者丢弃可重连补回）。
func emit(ch chan<- AgentEvent, evt AgentEvent) {
	select {
	case ch <- evt:
	default:
	}
}
