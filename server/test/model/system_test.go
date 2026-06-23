
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestSystemConfig_Fields 验证 SystemConfig 模型字段定义
func TestSystemConfig_Fields(t *testing.T) {
	now := time.Now()
	value := datatypes.JSON(`{"threshold":0.6}`)
	cfg := model.SystemConfig{
		Key:       "ai.confidence_threshold",
		Value:     value,
		Description: "AI 置信度阈值",
		UpdatedBy: 1,
		UpdatedAt: now,
	}
	cfg.ID = 1

	if cfg.Key != "ai.confidence_threshold" {
		t.Errorf("Key = %q, 期望 ai.confidence_threshold", cfg.Key)
	}
	if cfg.Value == nil {
		t.Error("Value 为 nil, 期望有值")
	}
	if cfg.Description != "AI 置信度阈值" {
		t.Errorf("Description = %q, 期望 AI 置信度阈值", cfg.Description)
	}
	if cfg.UpdatedBy != 1 {
		t.Errorf("UpdatedBy = %d, 期望 1", cfg.UpdatedBy)
	}
}
