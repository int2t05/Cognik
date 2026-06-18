// Package repository 提供系统配置的数据访问层。
//
// ConfigRepo 封装 SystemConfig 表的 CRUD 操作，供 ConfigService 调用。
// 为什么单独建包而非放在 model 层：Repository 层负责数据访问细节
// （如 Upsert 的 ON CONFLICT 逻辑），model 层只定义结构体和表名。
package repository

import (
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

// GetByKey 按 key 查询配置。
//
// key 具有唯一索引，查询不到时返回 gorm.ErrRecordNotFound。
func (r *ConfigRepo) GetByKey(key string) (*model.SystemConfig, error) {
	var cfg model.SystemConfig
	err := r.db.Where("key = ?", key).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert 更新或插入配置。
//
// 为什么用 ON CONFLICT 而非先查后改：
// ON CONFLICT 是原子操作，避免并发场景下的竞态条件。
// GORM 的 clause.OnConflict 配合 Clauses 可以在单条 SQL 中完成。
func (r *ConfigRepo) Upsert(key string, value datatypes.JSON, updatedBy int64) error {
		cfg := model.SystemConfig{
		Key:       key,
		Value:     value,
		UpdatedBy: updatedBy,
		UpdatedAt: time.Now(),
	}

	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_by", "updated_at"}),
	}).Create(&cfg).Error
}

// List 列出全部配置。
//
// 为什么返回空切片而非 nil：调用方可以直接 range 遍历，无需额外 nil 判断。
func (r *ConfigRepo) List() ([]model.SystemConfig, error) {
	var configs []model.SystemConfig
	err := r.db.Find(&configs).Error
	if err != nil {
		return nil, err
	}
	if configs == nil {
		configs = []model.SystemConfig{}
	}
	return configs, nil
}
