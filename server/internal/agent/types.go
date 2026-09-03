// Package agent 提供 Agent Loop 基座（事件生产者）。
//
// 基于 Eino 框架的 ReAct 循环：Eino ChatModel + ReactAgent + 内置工具 + SSE 事件桥接。
// Agent 领域不直接写 SSE——它只产生 AgentEvent，由 chat/session 层适配为 StreamEvent
// 并发布到 runtime.Gateway 网关（订阅渠道制，对齐 LangGraph Server / Mastra Durable）。
//
// 架构：生产者（AgentRunner）→ 网关（Gateway）→ 订阅者（writeStream SSE）。
package agent

// AgentEvent Agent 循环产生的流式事件。
// 字段与 chat/session.StreamEvent 对齐，由 chat 层适配（~10 行转换）。
type AgentEvent struct {
	Type    string `json:"type"`              // reasoning | token | tool_call | tool_result | done | error
	Content string `json:"content,omitempty"` // token 文本 / reasoning / 工具参数 / 工具结果
	ID      string `json:"id,omitempty"`       // 工具调用 ID（tool_call/tool_result 配对）
	Label   string `json:"label,omitempty"`    // 工具名
	Error   string `json:"error,omitempty"`   // 错误信息
}

// 事件类型常量。
const (
	EventReasoning   = "reasoning"   // 模型思考内容（llama.cpp thinking mode）
	EventToken       = "token"       // 最终回答 token
	EventToolCall    = "tool_call"    // Agent 决定调用工具
	EventToolResult  = "tool_result" // 工具执行结果
	EventDone        = "done"        // 循环结束
	EventError       = "error"       // 错误
)
