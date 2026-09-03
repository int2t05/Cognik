// Package audit 审计日志数据访问。
//
// repository.go 管理 audit_logs 表读写。
package audit

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// AuditFilter 审计日志查询过滤条件。
type AuditFilter struct {
	OperatorID int64
	Action     string
	TargetType string
	TargetID   int64
	Keyword    string
	DateFrom   string
	DateTo     string
	Page       int
	PageSize   int
}

// escapeLike 转义 LIKE/ILIKE 模式中的通配符（%、_、\），配合 ESCAPE '\\' 子句实现字面量搜索。
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// AuditLogRow 审计日志查询结果行（含 LEFT JOIN users 的操作人姓名）。
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

// Create 写入一条审计日志。
func (r *AuditRepo) Create(ctx context.Context, log interface{}) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// List 分页查询审计日志（LEFT JOIN users，支持多维过滤）。
func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]AuditLogRow, int64, error) {
	query := r.db.WithContext(ctx).Table("audit_logs").
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
	if f.Keyword != "" {
		like := "%" + escapeLike(f.Keyword) + "%"
		query = query.Where("(audit_logs.action ILIKE ? ESCAPE '\\' OR audit_logs.target_type ILIKE ? ESCAPE '\\' OR audit_logs.detail::text ILIKE ? ESCAPE '\\')", like, like, like)
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

// BatchDelete 批量删除审计日志。
func (r *AuditRepo) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Table("audit_logs").Where("id IN ?", ids).Delete(nil)
	return res.RowsAffected, res.Error
}
