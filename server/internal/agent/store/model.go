// Package store 提供 Agent 对话数据的独立存储（SQLite）。
//
// 与业务 PostgreSQL 隔离：agent 对话数据（threads/messages）存 SQLite（data/agent.db），
// 不混入业务库。对标 agent-starter ChatStore 接口 + AI SDK UIMessage parts 模型。
package store

import "time"

// Thread Agent 对话线程（对标 AI SDK ChatThread）。
type Thread struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"index:idx_thread_user;not null" json:"user_id"`
	Title     string    `gorm:"type:text;not null;default:'新对话'" json:"title"`
	CreatedAt time.Time `gorm:"not null;index:idx_thread_created" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (Thread) TableName() string { return "threads" }

// 消息状态常量。
const (
	MessageStatusGenerating = "generating"
	MessageStatusCompleted  = "completed"
	MessageStatusFailed     = "failed"
	MessageStatusCancelled  = "cancelled"
)

// Message Agent 对话消息（parts 数组模型，对标 AI SDK UIMessage）。
// Parts 存 JSON 数组：[{type:"text",content:"..."},{type:"reasoning",...},{type:"tool_call",...}]
type Message struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ThreadID  int64     `gorm:"not null;index:idx_msg_thread" json:"thread_id"`
	Role      string    `gorm:"type:varchar(16);not null" json:"role"`     // user / assistant
	Parts     string    `gorm:"type:text;not null;default:'[]'" json:"parts"` // JSON 数组
	Status    string    `gorm:"type:varchar(16);not null;default:completed" json:"status"`
	Error     string    `gorm:"type:text" json:"error"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (Message) TableName() string { return "messages" }

// MessagePart 消息部件（辨别联合，对标 AI SDK UIMessage.parts + assistant-ui types/message.ts）。
type MessagePart struct {
	Type    string `json:"type"`              // text / reasoning / tool_call / tool_result
	Content string `json:"content,omitempty"` // 文本内容 / 工具参数 / 工具结果
	ID      string `json:"id,omitempty"`      // 工具调用 ID（tool_call/tool_result 配对）
	Label   string `json:"label,omitempty"`   // 工具名
	Status  string `json:"status,omitempty"`  // 工具调用状态：running / done / error
}

// Part 类型常量。
const (
	PartText       = "text"
	PartReasoning  = "reasoning"
	PartToolCall   = "tool_call"
	PartToolResult = "tool_result"
)
