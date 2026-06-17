package model

import (
	"time"

	"gorm.io/datatypes"
)

// ChatSession 问答会话表
type ChatSession struct {
	ID         int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64          `gorm:"column:user_id;index:idx_chat_user_id" json:"user_id"`
	KBID       int64          `gorm:"column:kb_id" json:"kb_id"`
	Question   string         `gorm:"type:text;not null" json:"question"`
	Answer     string         `gorm:"type:text" json:"answer"`
	Sources    datatypes.JSON `gorm:"type:jsonb" json:"sources"`
	// TODO(model/chat): 增加 pipeline_metrics JSONB 字段持久化 RAG 管道各步骤耗时。
	Confidence float64        `json:"confidence"`
	Feedback   int16          `json:"feedback"`
	DurationMs int            `gorm:"column:duration_ms" json:"duration_ms"`
	CreatedAt  time.Time      `gorm:"not null;index:idx_chat_created_at" json:"created_at"`
}

func (ChatSession) TableName() string { return "chat_sessions" }

// ChatMessage 对话消息表
type ChatMessage struct {
	ID         int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID  int64          `gorm:"not null;column:session_id;index:idx_chat_messages_session" json:"session_id"`
	Role       string         `gorm:"type:varchar(16);not null" json:"role"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	Sources    datatypes.JSON `gorm:"type:jsonb" json:"sources"`
	Confidence float64        `json:"confidence"`
	CreatedAt  time.Time      `gorm:"not null" json:"created_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }
