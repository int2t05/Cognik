// Package repository 提供数据访问层。
//
// llm_config_repo.go 定义 LLM 配置（llm_configs 表）的 CRUD 操作。
package repository

import (
	"context"
	"errors"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// ErrNotFound 导出哨兵供跨包错误比较。
var ErrNotFound = gorm.ErrRecordNotFound

// LlmConfigRepo LLM 配置数据访问。
type LlmConfigRepo struct {
	db *gorm.DB
}

// NewLlmConfigRepo 创建 LlmConfigRepo 实例。
func NewLlmConfigRepo(db *gorm.DB) *LlmConfigRepo {
	return &LlmConfigRepo{db: db}
}

// DB 返回底层 *gorm.DB，供 Service 层事务操作使用。
func (r *LlmConfigRepo) DB() *gorm.DB {
	return r.db
}

func (r *LlmConfigRepo) Create(ctx context.Context, cfg *model.LlmConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *LlmConfigRepo) FindByID(ctx context.Context, id int64) (*model.LlmConfig, error) {
	var cfg model.LlmConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FindDefault 查询默认配置。
// 数据库层已有部分唯一索引 idx_llm_configs_default (WHERE is_default=true) 兜底。
// 未找到默认配置时返回 nil, nil（静默降级，不视为错误），
// 避免 GORM 在日志中打印 "record not found" 误导用户。
func (r *LlmConfigRepo) FindDefault(ctx context.Context) (*model.LlmConfig, error) {
	var cfg model.LlmConfig
	err := r.db.WithContext(ctx).Where("is_default = ?", true).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *LlmConfigRepo) List(ctx context.Context) ([]model.LlmConfig, error) {
	var configs []model.LlmConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&configs).Error
	return configs, err
}

func (r *LlmConfigRepo) Update(ctx context.Context, cfg *model.LlmConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *LlmConfigRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.LlmConfig{}, id).Error
}

func (r *LlmConfigRepo) CountReferencingKBs(ctx context.Context, configID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.KnowledgeBase{}).Where("llm_config_id = ?", configID).Count(&count).Error
	return count, err
}

// ClearDefault 清空所有默认标志。
func (r *LlmConfigRepo) ClearDefault(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&model.LlmConfig{}).Where("is_default = ?", true).Update("is_default", false).Error
}

// 确保导出了 ErrNotFound（兼容 mock 使用）
var _ = ErrNotFound
