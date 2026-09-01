package model

import (
	"time"

	"gorm.io/datatypes"
)

// AuditLog 审计日志表
type AuditLog struct {
	ID         int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	OperatorID int64          `gorm:"column:operator_id;index:idx_audit_operator" json:"operator_id"`
	Action     string         `gorm:"type:varchar(64);not null;index:idx_audit_action" json:"action"`
	TargetType string         `gorm:"type:varchar(32);column:target_type" json:"target_type"`
	TargetID   int64          `gorm:"column:target_id" json:"target_id"`
	Detail     datatypes.JSON `gorm:"type:jsonb" json:"detail"`
	IPAddress  string         `gorm:"type:varchar(45);column:ip_address" json:"ip_address"`
	CreatedAt  time.Time      `gorm:"not null;index:idx_audit_created_at" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
