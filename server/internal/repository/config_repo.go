// Package repository 提供系统配置的数据访问层。
//
// ConfigRepo 封装 SystemConfig 表的 CRUD 操作，供 ConfigService 调用。
package repository

import (
	"context"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConfigRepo 系统配置数据访问
type ConfigRepo struct {
	db *gorm.DB
}

// NewConfigRepo 创建 ConfigRepo 实例
func NewConfigRepo(db *gorm.DB) *ConfigRepo {
	return &ConfigRepo{db: db}
}

func (r *ConfigRepo) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var cfg model.SystemConfig
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert 更新或插入配置，同时写入 description。
func (r *ConfigRepo) Upsert(ctx context.Context, key, description string, value datatypes.JSON, updatedBy int64) error {
	cfg := model.SystemConfig{
		Key:         key,
		Value:       value,
		Description: description,
		UpdatedBy:   updatedBy,
		UpdatedAt:   time.Now(),
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "description", "updated_by", "updated_at"}),
	}).Create(&cfg).Error
}

func (r *ConfigRepo) List(ctx context.Context) ([]model.SystemConfig, error) {
	var configs []model.SystemConfig
	err := r.db.WithContext(ctx).Find(&configs).Error
	if err != nil {
		return nil, err
	}
	if configs == nil {
		configs = []model.SystemConfig{}
	}
	return configs, nil
}
