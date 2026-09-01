package model

import "time"

// Message 站内消息表
type Message struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"not null;column:user_id;index:idx_messages_user_id" json:"user_id"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	Type        string    `gorm:"type:varchar(32);not null" json:"type"`
	RelatedType string    `gorm:"type:varchar(32);column:related_type" json:"related_type"`
	RelatedID   int64     `gorm:"column:related_id" json:"related_id"`
	IsRead      bool      `gorm:"not null;default:false;column:is_read;index:idx_messages_is_read" json:"is_read"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

func (Message) TableName() string { return "messages" }
