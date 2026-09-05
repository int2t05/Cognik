// Package llmconfig LLM 配置数据访问。
//
// repository.go 管理 llm_configs 表 CRUD。
package llmconfig

import (
	"context"

	"cognik/internal/shared/model"

	"gorm.io/gorm"
)

// LlmConfigRepo LLM 配置数据访问。
type LlmConfigRepo struct {
	db *gorm.DB
}

// NewLlmConfigRepo 创建 LlmConfigRepo 实例。
func NewLlmConfigRepo(db *gorm.DB) *LlmConfigRepo {
	return &LlmConfigRepo{db: db}
}

// DB 返回底层 *gorm.DB，供 Service 事务使用。
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

// FindDefault 查询默认配置（未找到返回 nil, nil，不视为错误）。
func (r *LlmConfigRepo) FindDefault(ctx context.Context) (*model.LlmConfig, error) {
	var cfgs []model.LlmConfig
	if err := r.db.WithContext(ctx).Where("is_default = ?", true).Limit(1).Find(&cfgs).Error; err != nil {
		return nil, err
	}
	if len(cfgs) == 0 {
		return nil, nil
	}
	return &cfgs[0], nil
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
