// Package repository 提供数据访问层。
//
// llm_config_repo.go 定义 LLM 配置（llm_configs 表）的 CRUD 操作。
package repository

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// ErrNotFound 导出哨兵供跨包错误比较（如 service_test 中 mock 使用）。
// 保留此导出以确保测试兼容性。
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

// Create 创建 LLM 配置。
func (r *LlmConfigRepo) Create(cfg *model.LlmConfig) error {
	return r.db.Create(cfg).Error
}

// FindByID 按 ID 查询配置。
func (r *LlmConfigRepo) FindByID(id int64) (*model.LlmConfig, error) {
	var cfg model.LlmConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FindDefault 查询默认配置。
func (r *LlmConfigRepo) FindDefault() (*model.LlmConfig, error) {
	// TODO(repository/llm_config): 数据库层应增加唯一约束保证最多一个 is_default=true。
	// 仅靠 Service ClearDefault 在并发请求下仍可能短暂产生多个默认配置。
	var cfg model.LlmConfig
	err := r.db.Where("is_default = ?", true).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// List 列出全部配置（按 id 排序）。
func (r *LlmConfigRepo) List() ([]model.LlmConfig, error) {
	var configs []model.LlmConfig
	err := r.db.Order("id ASC").Find(&configs).Error
	return configs, err
}

// Update 更新配置（主键 + 全字段）。
func (r *LlmConfigRepo) Update(cfg *model.LlmConfig) error {
	return r.db.Save(cfg).Error
}

// Delete 删除配置。
func (r *LlmConfigRepo) Delete(id int64) error {
	return r.db.Delete(&model.LlmConfig{}, id).Error
}

// CountReferencingKBs 统计引用了该配置的知识库数量。
func (r *LlmConfigRepo) CountReferencingKBs(configID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.KnowledgeBase{}).Where("llm_config_id = ?", configID).Count(&count).Error
	return count, err
}

// ClearDefault 清空所有默认标志。
//
// 为什么用批量 UPDATE 而非逐条：
// UPDATE ... SET is_default=false WHERE is_default=true 是单条原子 SQL，
// 比逐条修改更快且无竞态。
func (r *LlmConfigRepo) ClearDefault() error {
	return r.db.Model(&model.LlmConfig{}).Where("is_default = ?", true).Update("is_default", false).Error
}

// 确保导出了 ErrNotFound（兼容 mock 使用）
var _ = ErrNotFound
