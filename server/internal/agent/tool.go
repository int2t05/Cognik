// agent/tool.go：统一工具接口（SyncTool/AsyncTool）。
//
// 两种执行语义，由接口类型区分（Go 惯用，同 io.Reader/io.Writer）：
//   - SyncTool  同步阻塞，Call 返回结果（read_file/grep/bash/web_search/web_fetch/kb）
//   - AsyncTool 异步派发，Dispatch 立即返回 Task，后台跑，事件经 emit 流式推送（dispatch_subagent）
//
// Loop 用 type assertion 选择 Call 或 Dispatch，无需 Kind() 元方法（类型即语义）。
// 长输出通过 emit(EventToken) 逐行推，无独立流式返回值，消除 StreamableTool 中间态。
package agent

import (
	"context"

	"cognik/internal/agent/llm"
)

// Tool 工具基接口。所有工具实现 Info，供 LLM 决策调用。
type Tool interface {
	// Info 返回工具元信息（名称/描述/参数 schema）。
	Info() *llm.ToolInfo
}

// SyncTool 同步工具：Call 阻塞返回结果字符串。
// emit 用于推送中间进度（如 bash 长输出的逐行 token、子步骤事件）。
type SyncTool interface {
	Tool
	Call(ctx context.Context, args string, emit EventSink) (string, error)
}

// AsyncTool 异步工具：Dispatch 立即返回 Task，后台 goroutine 执行。
// 事件经 emit 流式推送；完成时 task.markDone 触发 Loop 的 waitForAny 恢复。
type AsyncTool interface {
	Tool
	Dispatch(ctx context.Context, args string, emit EventSink) (*Task, error)
}
