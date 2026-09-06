// Package config 系统配置业务逻辑（配置键白名单、值校验）。
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cognik/internal/domain/system/audit"
	"cognik/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// configKeyMeta 配置键元信息：期望类型和说明。
type configKeyMeta struct {
	ValueType   string // "string" | "number" | "bool"
	Description string // 配置项说明
}

// validConfigKeys 配置键白名单。
var validConfigKeys = map[string]configKeyMeta{
	"app_name": {ValueType: "string", Description: "应用名称，显示在页面标题和系统通知中"},
}

// ConfigService 系统配置管理服务。
type ConfigService struct {
	repo        *ConfigRepo
	auditWriter audit.AuditWriter
}

// NewConfigService 创建 ConfigService 实例。
func NewConfigService(repo *ConfigRepo, auditWriter audit.AuditWriter) *ConfigService {
	return &ConfigService{repo: repo, auditWriter: auditWriter}
}

// GetInt 读取整数配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetInt(ctx context.Context, key string) (int, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// GetFloat 读取浮点配置，不存在或类型不匹配返回 (0, false)。
func (s *ConfigService) GetFloat(ctx context.Context, key string) (float64, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return 0, false
	}
	if n, ok := v.(float64); ok {
		return n, true
	}
	return 0, false
}

// GetBool 读取布尔配置，不存在或类型不匹配返回 (false, false)。
func (s *ConfigService) GetBool(ctx context.Context, key string) (bool, bool) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == nil {
		return false, false
	}
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

// GetConfig 获取指定 key 的配置值。
func (s *ConfigService) GetConfig(ctx context.Context, key string) (interface{}, error) {
	if _, ok := validConfigKeys[key]; !ok {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}

	cfg, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 有效 key 但尚未初始化：返回 null 而非报错
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
func (s *ConfigService) UpdateConfig(ctx context.Context, key string, value interface{}, updatedBy int64) error {
	meta, ok := validConfigKeys[key]
	if !ok {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}
	if value == nil {
		return errcode.AppError{Code: errcode.ErrParam, Message: "配置值不能为 nil"}
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}

	if err := s.repo.Upsert(ctx, key, meta.Description, datatypes.JSON(jsonBytes), updatedBy); err != nil {
		return err
	}
	s.auditWriter.Write(ctx, updatedBy, "config.update", "config", 0, string(jsonBytes))
	return nil
}

// IsPublicKey 判断指定 key 是否为无需认证的公开配置项。
func (s *ConfigService) IsPublicKey(key string) bool {
	return key == "app_name"
}
