// Package service 实现系统配置管理业务逻辑。
//
// ConfigService 提供系统配置的获取和更新功能。
// 支持白名单内的配置键读写，拒绝未知 key。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// configKeyMeta 定义配置键的元信息：期望类型和用途说明。
type configKeyMeta struct {
	ValueType   string // "string" | "number" | "bool"
	Description string // 配置项说明，写入 system_configs.description
}

// validConfigKeys 配置键白名单。
//
// 为什么用白名单而非自由 key-value：
// 自由 key-value 允许调用方写入任意键名，拼写错误导致静默创建无用配置项，
// 且前端无法区分「配置不存在」和「配置类型不符」。
// validConfigKeys 配置键白名单。
//
// 为什么用白名单而非自由 key-value：
// 自由 key-value 允许调用方写入任意键名，拼写错误导致静默创建无用配置项，
// 且前端无法区分「配置不存在」和「配置类型不符」。
var validConfigKeys = map[string]configKeyMeta{
	"app_name":    {ValueType: "string", Description: "应用名称，显示在页面标题和系统通知中"},
	"ai.top_k":    {ValueType: "number", Description: "RAG 默认检索 Top K"},
	"ai.threshold": {ValueType: "number", Description: "AI 置信度阈值"},
}

// ConfigService 系统配置管理服务。
type ConfigService struct {
	repo      *repository.ConfigRepo
	auditRepo *repository.AuditRepo
}

// NewConfigService 创建 ConfigService 实例。
func NewConfigService(repo *repository.ConfigRepo, auditRepo *repository.AuditRepo) *ConfigService {
	return &ConfigService{repo: repo, auditRepo: auditRepo}
}

// GetConfig 获取指定 key 的配置值。
func (s *ConfigService) GetConfig(ctx context.Context, key string) (interface{}, error) {
	if _, ok := validConfigKeys[key]; !ok {
		return nil, AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}

	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(cfg.Value, &value); err != nil {
		return nil, fmt.Errorf("解析配置值失败: %w", err)
	}

	return value, nil
}

// UpdateConfig 更新或创建系统配置。
//
// value 会被序列化为 JSONB 存储，nil 被拒绝。
// 仅允许白名单内的 key 写入，同时写入白名单中对应的 description。
func (s *ConfigService) UpdateConfig(ctx context.Context, key string, value interface{}, updatedBy int64) error {
	meta, ok := validConfigKeys[key]
	if !ok {
		return AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}
	if value == nil {
		return AppError{Code: errcode.ErrParam, Message: "配置值不能为 nil"}
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}

	if err := s.repo.Upsert(ctx, key, meta.Description, datatypes.JSON(jsonBytes), updatedBy); err != nil {
		return err
	}
	s.auditRepo.Create(ctx, &model.AuditLog{
		OperatorID: updatedBy, Action: "config.update",
		TargetType: "config", TargetID: 0,
		Detail: datatypes.JSON(jsonBytes),
	})
	return nil
}
