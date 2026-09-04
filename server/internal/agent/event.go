// Package agent 提供 Agent Loop 基座（事件生产者）。
//
// 自建 ReAct 循环：openai.ChatModel + 统一工具接口 + SSE 事件桥接。
// Agent 领域不直接写 SSE——它只产生 AgentEvent，由 chat/session 层适配为 StreamEvent
// 并发布到 runtime.Gateway 网关（订阅渠道制，对齐 LangGraph Server / Mastra Durable）。
//
// 架构：生产者（AgentRunner → Loop）→ 网关（Gateway）→ 订阅者（writeStream SSE）。
package agent

// AgentEvent Agent 循环产生的流式事件。
// 字段与 chat/session.StreamEvent 对齐，由 chat 层适配（~10 行转换）。
type AgentEvent struct {
	Type    string `json:"type"`              // reasoning | token | tool_call | tool_result | done | error
	Content string `json:"content,omitempty"` // token 文本 / reasoning / 工具参数 / 工具结果
	ID      string `json:"id,omitempty"`      // 工具调用 ID（tool_call/tool_result 配对）
	Label   string `json:"label,omitempty"`   // 工具名
	Error   string `json:"error,omitempty"`   // 错误信息
	TaskID  string `json:"task_id,omitempty"` // 子 Agent 来源 task ID（空=主 Agent；非空=子 Agent 事件，归入对应 tool_call 卡片）
}

// 事件类型常量。
const (
	EventReasoning  = "reasoning"   // 模型思考内容（llama.cpp thinking mode）
	EventToken      = "token"       // 最终回答 token
	EventToolCall   = "tool_call"   // Agent 决定调用工具
	EventToolResult = "tool_result" // 工具执行结果
	EventDone       = "done"        // 循环结束
	EventError      = "error"       // 错误
)

// EventSink 事件出口：工具/子 Agent 通过它推事件到主流。
// 统一事件通道，消除跨层 ctx 注入耦合。
//
// 示例：bash 工具逐行输出 → emit(AgentEvent{Type: EventToken, Content: line})
type EventSink func(AgentEvent)
