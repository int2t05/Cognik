// Package task 提供 Agent 异步任务管理。
// 任务在后台执行（goroutine + detached ctx），不阻塞对话。
// 任务状态持久化到 SQLite（与 agent 对话数据同一库）。

package task

import "time"

// Task 异步任务模型（SQLite 存储）。
type Task struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ThreadID  int64     `gorm:"index:idx_task_thread" json:"thread_id"`
	UserID    int64     `gorm:"index:idx_task_user" json:"user_id"`
	Type      string    `gorm:"type:varchar(32);not null" json:"type"`     // agent_run / custom
	Status    string    `gorm:"type:varchar(16);not null;default:pending" json:"status"` // pending/running/completed/failed/cancelled
	Input     string    `gorm:"type:text;not null;default:'{}'" json:"input"` // JSON: 任务输入
	Output    string    `gorm:"type:text" json:"output"`                    // JSON: 任务输出
	Error     string    `gorm:"type:text" json:"error"`                    // 错误信息
	CreatedAt time.Time `gorm:"not null;index:idx_task_created" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

// 任务状态常量。
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// 任务类型常量。
const (
	TypeAgentRun = "agent_run" // Agent 执行任务
	TypeCustom   = "custom"    // 自定义任务
)
