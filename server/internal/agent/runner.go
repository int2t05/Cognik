// agent/runner.go：AgentRunner 跑 ReAct 循环 → 事件 channel（生产者）。
//
// 一个 goroutine 跑 Loop.Run，emit 转发到 <-chan AgentEvent。
//
// 生产者与交付渠道（runtime.Gateway）解耦：runner 只产出事件，
// 由 chat 层订阅并 Publish 到网关。独立 ctx 保证客户端断开后生成继续跑完并落库。
package agent

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// AgentRunner 跑 ReAct 循环并产出事件流（生产者）。
type AgentRunner struct {
	loop *Loop
}

// NewAgentRunner 创建 runner。
func NewAgentRunner(loop *Loop) *AgentRunner {
	return &AgentRunner{loop: loop}
}

// Stream 跑 ReAct 循环，返回事件 channel。
// ctx 应为 detached（带超时）— 客户端断开不停止生成。
func (r *AgentRunner) Stream(ctx context.Context, input []*schema.Message) (<-chan AgentEvent, error) {
	out := make(chan AgentEvent, 256)

	// 生产者：Loop.Run 内部通过 emit 推 reasoning/token/tool_call/tool_result 事件。
	// Loop.Run 返回后 emit done（或 error），然后关闭 channel。
	go func() {
		defer close(out)
		emit := func(evt AgentEvent) {
			select {
			case out <- evt:
			default: // channel 满（慢消费者）丢弃，生成 goroutine 始终跑完
			}
		}
		result, err := r.loop.Run(ctx, input, emit)
		if err != nil {
			emit(AgentEvent{Type: EventError, Error: err.Error()})
			return
		}
		// 最终回答已通过 emit(EventToken) 流式推送；done 事件触发 chat 层落库。
		_ = result
		emit(AgentEvent{Type: EventDone})
	}()

	return out, nil
}
