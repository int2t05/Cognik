// Package repository 提供审计日志的数据访问层。
//
// AuditRepo 封装 audit_logs 表的写入和查询操作。
// 审计日志写入由各 Service 层在关键操作完成后同步调用。
package repository

import (
	"strings"

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

// AuditLogRow 审计日志查询结果行，包含 LEFT JOIN users 得到的操作人姓名。
type AuditLogRow struct {
	ID           int64  `json:"id"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Action       string `json:"action"`
	TargetType   string `json:"target_type"`
	TargetID     int64  `json:"target_id"`
	Detail       string `json:"detail"`
	IPAddress    string `json:"ip_address"`
	CreatedAt    string `json:"created_at"`
}

// AuditRepo 审计日志数据访问。
type AuditRepo struct {
	db *gorm.DB
}

// NewAuditRepo 创建 AuditRepo 实例。
func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Create 写入一条审计日志。写入失败返回 error，由调用方决定是否阻断主流程。
func (r *AuditRepo) Create(log interface{}) error {
	return r.db.Create(log).Error
}

// List 分页查询审计日志（LEFT JOIN users 获取操作人姓名），支持多维过滤。
func (r *AuditRepo) List(f AuditFilter) ([]AuditLogRow, int64, error) {
	// audit_logs LEFT JOIN users，一次查询获取操作人姓名
	query := r.db.Table("audit_logs").
		Select(`audit_logs.id, audit_logs.operator_id, audit_logs.action,
			audit_logs.target_type, audit_logs.target_id,
			COALESCE(users.real_name, '') AS operator_name,
			audit_logs.detail::text AS detail,
			audit_logs.ip_address,
			TO_CHAR(audit_logs.created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at`).
		Joins("LEFT JOIN users ON audit_logs.operator_id = users.id")

	if f.OperatorID > 0 {
		query = query.Where("audit_logs.operator_id = ?", f.OperatorID)
	}
	if f.Action != "" {
		if strings.HasSuffix(f.Action, "*") {
			query = query.Where("audit_logs.action LIKE ?", strings.TrimSuffix(f.Action, "*")+"%")
		} else {
			query = query.Where("audit_logs.action = ?", f.Action)
		}
	}
	if f.TargetType != "" {
		query = query.Where("audit_logs.target_type = ?", f.TargetType)
	}
	if f.TargetID > 0 {
		query = query.Where("audit_logs.target_id = ?", f.TargetID)
	}
	if f.DateFrom != "" {
		query = query.Where("audit_logs.created_at >= ?::date", f.DateFrom)
	}
	if f.DateTo != "" {
		query = query.Where("audit_logs.created_at < (?::date + INTERVAL '1 day')", f.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AuditLogRow
	offset := (f.Page - 1) * f.PageSize
	if err := query.Offset(offset).Limit(f.PageSize).Order("audit_logs.created_at DESC").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
