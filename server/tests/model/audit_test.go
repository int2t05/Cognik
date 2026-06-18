
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestAuditLog_Fields 验证 AuditLog 模型字段定义
func TestAuditLog_Fields(t *testing.T) {
	now := time.Now()
	detail := datatypes.JSON(`{"old_status":1,"new_status":2}`)
	log := model.AuditLog{
		OperatorID: 1,
		Action:     "ticket.status_change",
		TargetType: "ticket",
		TargetID:   100,
		Detail:     detail,
		IPAddress:  "192.168.1.1",
		CreatedAt:  now,
	}
	log.ID = 1

	if log.OperatorID != 1 {
		t.Errorf("OperatorID = %d, 期望 1", log.OperatorID)
	}
	if log.Action != "ticket.status_change" {
		t.Errorf("Action = %q, 期望 ticket.status_change", log.Action)
	}
	if log.TargetType != "ticket" {
		t.Errorf("TargetType = %q, 期望 ticket", log.TargetType)
	}
	if log.TargetID != 100 {
		t.Errorf("TargetID = %d, 期望 100", log.TargetID)
	}
	if log.IPAddress != "192.168.1.1" {
		t.Errorf("IPAddress = %q, 期望 192.168.1.1", log.IPAddress)
	}
	if log.Detail == nil {
		t.Error("Detail 为 nil, 期望有值")
	}
}
