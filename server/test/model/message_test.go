
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"
)

// TestMessage_Fields 验证 Message 模型字段定义
func TestMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := model.Message{
		UserID:      1,
		Title:       "申告补充信息通知",
		Content:     "您的申告 TK-20250101-0001 需要补充信息",
		Type:        model.MessageTypeTicketSupplement,
		RelatedType: "ticket",
		RelatedID:   100,
		IsRead:      false,
		CreatedAt:   now,
	}
	msg.ID = 1

	if msg.UserID != 1 {
		t.Errorf("UserID = %d, 期望 1", msg.UserID)
	}
	if msg.Title != "申告补充信息通知" {
		t.Errorf("Title = %q, 期望 申告补充信息通知", msg.Title)
	}
	if msg.Type != model.MessageTypeTicketSupplement {
		t.Errorf("Type = %q, 期望 %q", msg.Type, model.MessageTypeTicketSupplement)
	}
	if msg.RelatedType != "ticket" {
		t.Errorf("RelatedType = %q, 期望 ticket", msg.RelatedType)
	}
	if msg.RelatedID != 100 {
		t.Errorf("RelatedID = %d, 期望 100", msg.RelatedID)
	}
	if msg.IsRead {
		t.Error("IsRead = true, 期望 false")
	}
}
