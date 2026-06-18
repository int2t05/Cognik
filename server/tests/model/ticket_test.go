
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestTicket_Fields 验证 Ticket 模型字段定义
func TestTicket_Fields(t *testing.T) {
	now := time.Now()
	context := datatypes.JSON(`{"session_id": 1}`)
	systems := datatypes.JSON(`["OA系统","ERP系统"]`)
	tk := model.Ticket{
		TicketNo:        "TK-20250101-0001",
		UserID:          1,
		Title:           "OA系统无法登录",
		Description:     "点击登录后页面空白",
		Urgency:         model.TicketUrgencyHigh,
		ImpactScope:     model.ImpactDept,
		AffectedSystems: systems,
		ContactPhone:    "13800138000",
		ContactEmail:    "user@example.com",
		Status:          model.TicketStatusPending,
		SupplementCount: 0,
		ChatContext:     context,
		Source:          model.TicketSourcePortal,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	tk.ID = 1

	if tk.TicketNo != "TK-20250101-0001" {
		t.Errorf("TicketNo = %q, 期望 TK-20250101-0001", tk.TicketNo)
	}
	if tk.UserID != 1 {
		t.Errorf("UserID = %d, 期望 1", tk.UserID)
	}
	if tk.Urgency != model.TicketUrgencyHigh {
		t.Errorf("Urgency = %d, 期望 %d", tk.Urgency, model.TicketUrgencyHigh)
	}
	if tk.ImpactScope != model.ImpactDept {
		t.Errorf("ImpactScope = %d, 期望 %d", tk.ImpactScope, model.ImpactDept)
	}
	if tk.Status != model.TicketStatusPending {
		t.Errorf("Status = %d, 期望 %d", tk.Status, model.TicketStatusPending)
	}
	if tk.Source != model.TicketSourcePortal {
		t.Errorf("Source = %d, 期望 %d", tk.Source, model.TicketSourcePortal)
	}
	if tk.SupplementCount != 0 {
		t.Errorf("SupplementCount = %d, 期望 0", tk.SupplementCount)
	}
	if tk.AffectedSystems == nil {
		t.Error("AffectedSystems 为 nil, 期望有值")
	}
	if tk.ChatContext == nil {
		t.Error("ChatContext 为 nil, 期望有值")
	}
}

// TestTicketRecord_Fields 验证 TicketRecord 模型字段定义
func TestTicketRecord_Fields(t *testing.T) {
	now := time.Now()
	detail := datatypes.JSON(`{"callback_method":"phone","is_resolved":true}`)
	tr := model.TicketRecord{
		TicketID:   1,
		OperatorID: 2,
		Action:     model.TicketActionResolve,
		Content:    "已电话回访，问题解决",
		Detail:     detail,
		CreatedAt:  now,
	}
	tr.ID = 1

	if tr.TicketID != 1 {
		t.Errorf("TicketID = %d, 期望 1", tr.TicketID)
	}
	if tr.OperatorID != 2 {
		t.Errorf("OperatorID = %d, 期望 2", tr.OperatorID)
	}
	if tr.Action != model.TicketActionResolve {
		t.Errorf("Action = %q, 期望 %q", tr.Action, model.TicketActionResolve)
	}
	if tr.Detail == nil {
		t.Error("Detail 为 nil, 期望有值")
	}
}
