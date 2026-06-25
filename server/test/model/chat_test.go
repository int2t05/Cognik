
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestChatSession_Fields 验证 ChatSession 模型字段定义
func TestChatSession_Fields(t *testing.T) {
	now := time.Now()
	sources := datatypes.JSON(`[{"title":"OA文档","url":"http://example.com"}]`)
	sess := model.ChatSession{
		UserID:     1,
		KBID:       1,
		Question:   "OA系统无法登录",
		Answer:     "请清除浏览器缓存",
		Sources:    sources,
		Confidence: 0.85,
		Feedback:   1,
		DurationMs: 1200,
		CreatedAt:  now,
	}
	sess.ID = 1

	if sess.UserID != 1 {
		t.Errorf("UserID = %d, 期望 1", sess.UserID)
	}
	if sess.KBID != 1 {
		t.Errorf("KBID = %d, 期望 1", sess.KBID)
	}
	if sess.Confidence != 0.85 {
		t.Errorf("Confidence = %f, 期望 0.85", sess.Confidence)
	}
	if sess.Feedback != 1 {
		t.Errorf("Feedback = %d, 期望 1", sess.Feedback)
	}
	if sess.DurationMs != 1200 {
		t.Errorf("DurationMs = %d, 期望 1200", sess.DurationMs)
	}
	if sess.Sources == nil {
		t.Error("Sources 为 nil, 期望有值")
	}
}

// TestChatMessage_Fields 验证 ChatMessage 模型字段定义
func TestChatMessage_Fields(t *testing.T) {
	now := time.Now()
	sources := datatypes.JSON(`[{"title":"OA文档"}]`)
	msg := model.ChatMessage{
		SessionID:  1,
		Role:       "assistant",
		Content:    "请清除浏览器缓存",
		Sources:    sources,
		ConfidenceRaw: 0.9,
		CreatedAt:  now,
	}
	msg.ID = 1

	if msg.SessionID != 1 {
		t.Errorf("SessionID = %d, 期望 1", msg.SessionID)
	}
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, 期望 %q", msg.Role, "assistant")
	}
	if msg.ConfidenceRaw != 0.9 {
		t.Errorf("Confidence = %f, 期望 0.9", msg.ConfidenceRaw)
	}
}
