package model

import (
	"time"

	"gorm.io/datatypes"
)

// Ticket 申告工单表
type Ticket struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketNo        string         `gorm:"type:varchar(32);uniqueIndex;not null;column:ticket_no" json:"ticket_no"`
	UserID          int64          `gorm:"not null;column:user_id;index:idx_tickets_user_id" json:"user_id"`
	User            User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title           string         `gorm:"type:varchar(255);not null" json:"title"`
	Description     string         `gorm:"type:text;not null" json:"description"`
	Urgency         int16          `gorm:"not null" json:"urgency"`
	ImpactScope     int16          `gorm:"column:impact_scope" json:"impact_scope"`
	AffectedSystems datatypes.JSON `gorm:"type:jsonb;column:affected_systems" json:"affected_systems"`
	ContactPhone    string         `gorm:"type:varchar(11);not null;column:contact_phone" json:"contact_phone"`
	// TODO(model/ticket): contact_phone 长度固定 11 假设中国大陆手机号。
	// 如果企业支持分机或国际号码，应放宽并在 Service 层做格式化校验。
	ContactEmail    string         `gorm:"type:varchar(128);column:contact_email" json:"contact_email"`
	Status          int16          `gorm:"not null;default:1;index:idx_tickets_status" json:"status"`
	SupplementCount int16          `gorm:"not null;default:0;column:supplement_count" json:"supplement_count"`
	ChatContext     datatypes.JSON `gorm:"type:jsonb;column:chat_context" json:"chat_context"`
	Source          int16          `gorm:"not null;default:1" json:"source"`
	TicketRecords   []TicketRecord `gorm:"foreignKey:TicketID" json:"ticket_records,omitempty"`
	CreatedAt       time.Time      `gorm:"not null;index:idx_tickets_created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null" json:"updated_at"`
}

func (Ticket) TableName() string { return "tickets" }

// TicketRecord 申告处理记录表
type TicketRecord struct {
	ID         int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID   int64          `gorm:"not null;column:ticket_id;index:idx_ticket_records_ticket_id" json:"ticket_id"`
	OperatorID int64          `gorm:"not null;column:operator_id" json:"operator_id"` // 0=系统自动操作，无 FK 约束以避免冲突
	Action     string         `gorm:"type:varchar(32);not null" json:"action"`
	Content    string         `gorm:"type:text" json:"content"`
	Detail     datatypes.JSON `gorm:"type:jsonb" json:"detail"`
	CreatedAt  time.Time      `gorm:"not null" json:"created_at"`
}

func (TicketRecord) TableName() string { return "ticket_records" }
