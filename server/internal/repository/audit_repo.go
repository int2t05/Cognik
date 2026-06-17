// Package repository 提供审计日志的数据访问层。
//
// AuditRepo 封装 audit_logs 表的写入和查询操作。
// 审计日志写入由各 Service 层在关键操作完成后同步调用。
package repository

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AuditFilter 审计日志查询过滤条件。
type AuditFilter struct {
	OperatorID int64
	Action     string
	TargetType string
	TargetID   int64
	DateFrom   string
	DateTo     string
	Page       int
	PageSize   int
}

// AuditRepo 审计日志数据访问。
type AuditRepo struct {
	db *gorm.DB
}

// NewAuditRepo 创建 AuditRepo 实例。
func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Create 写入一条审计日志。写入失败返回 error。
func (r *AuditRepo) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// List 分页查询审计日志，支持多维过滤。
func (r *AuditRepo) List(f AuditFilter) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	query := r.db.Model(&model.AuditLog{})
	if f.OperatorID > 0 {
		query = query.Where("operator_id = ?", f.OperatorID)
	}
	if f.Action != "" {
		query = query.Where("action = ?", f.Action)
	}
	if f.TargetType != "" {
		query = query.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID > 0 {
		query = query.Where("target_id = ?", f.TargetID)
	}
	if f.DateFrom != "" {
		query = query.Where("created_at >= ?::date", f.DateFrom)
	}
	if f.DateTo != "" {
		query = query.Where("created_at < (?::date + INTERVAL '1 day')", f.DateTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PageSize
	if err := query.Offset(offset).Limit(f.PageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
