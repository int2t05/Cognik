package model

import (
	"time"

	"gorm.io/datatypes"
)

// SystemConfig 系统配置表。
//
// MVP 阶段仅需 key-value 存储，不设 value_type/editable/validation_schema 元数据字段。
// 配置项白名单（validConfigKeys）在 ConfigService 中维护。
type SystemConfig struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Key         string         `gorm:"type:varchar(128);uniqueIndex;not null" json:"key"`
	Value       datatypes.JSON `gorm:"type:jsonb" json:"value"`
	Description string         `gorm:"type:varchar(255)" json:"description"`
	UpdatedBy   int64          `gorm:"column:updated_by" json:"updated_by"`
	UpdatedAt   time.Time      `gorm:"not null" json:"updated_at"`
}

func (SystemConfig) TableName() string { return "system_configs" }
