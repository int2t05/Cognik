// Package service 实现系统配置管理业务逻辑。
//
// ConfigService 提供系统配置的获取和更新功能。
// 支持白名单内的配置键读写，拒绝未知 key。
package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// validConfigKeys 配置键白名单，每个 key 的期望类型。
//
// 为什么用白名单而非自由 key-value：
// 自由 key-value 允许调用方写入任意键名，拼写错误导致静默创建无用配置项，
// 且前端无法区分「配置不存在」和「配置类型不符」。
var validConfigKeys = map[string]string{
	"app_name": "string",
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
func (s *ConfigService) GetConfig(key string) (interface{}, error) {
	if _, ok := validConfigKeys[key]; !ok {
		return nil, AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}

	cfg, err := s.repo.GetByKey(key)
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
// 仅允许白名单内的 key 写入。
func (s *ConfigService) UpdateConfig(key string, value interface{}, updatedBy int64) error {
	if _, ok := validConfigKeys[key]; !ok {
		return AppError{Code: errcode.ErrNotFound, Message: fmt.Sprintf("配置项 %s 不存在", key)}
	}
	if value == nil {
		return AppError{Code: errcode.ErrParam, Message: "配置值不能为 nil"}
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}

	if err := s.repo.Upsert(key, datatypes.JSON(jsonBytes), updatedBy); err != nil {
		return err
	}
	s.auditRepo.Create(&model.AuditLog{
		OperatorID: updatedBy, Action: "config.update",
		TargetType: "config", TargetID: 0,
		Detail: datatypes.JSON(jsonBytes),
	})
	return nil
}
