// Package service 实现系统配置管理业务逻辑。
//
// ConfigService 提供系统配置的获取和更新功能。
//
// 为什么 ConfigService.GetConfig 返回 interface{} 而非具体类型：
// 系统配置项的值类型多样（字符串、数字、JSON 对象），使用 interface{}
// 由调用方按需断言，兼顾灵活性和类型安全。
package service

import (
	"errors"
	"encoding/json"
	"fmt"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
//
// 从 JSONB 反序列化后返回原始值（string、float64、map 等）。
// key 不存在时返回 AppError code=10004。
func (s *ConfigService) GetConfig(key string) (interface{}, error) {
	// TODO(service/config): 增加允许的配置 key 白名单和类型定义。
	// 当前任意 key 都可读写，调用方只能在运行时发现类型不符合预期。
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
// value 会被序列化为 JSONB 存储。value 为 nil 时拒绝更新。
// 为什么 nil 值被拒绝：JSONB 列存储 null 在语义上等同于配置不存在，
// 如果允许存储 null，GetConfig 会返回 JSON 的 null 而非 AppError，
// 导致调用方无法区分「配置不存在」和「配置值为 null」。
func (s *ConfigService) UpdateConfig(key string, value interface{}, updatedBy int64) error {
	// TODO(service/config): 更新 ai.default_top_k/ai.confidence_threshold 后没有同步到 ChatService 运行时配置。
	// 需要引入配置观察者或把 AI 配置也纳入 atomic manager。
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
