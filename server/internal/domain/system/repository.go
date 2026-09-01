// Package system 聚合系统管理领域（审计日志、系统配置、数据看板、站内消息）的
// Handler / Service / Repository 三层实现。
//
// repository.go 合并原 audit_repo / config_repo / dashboard_repo 三个数据访问实现，
// 封装 audit_logs、system_configs 表及看板聚合查询。
package system

import (
	"context"
	"strings"
	"time"

	"opsmind/internal/shared/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// =============================================================================
// 审计日志 AuditRepo
// =============================================================================

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

// Create 写入一条审计日志。
func (r *AuditRepo) Create(ctx context.Context, log interface{}) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// List 分页查询审计日志（LEFT JOIN users 获取操作人姓名），支持多维过滤。
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

// =============================================================================
// 系统配置 ConfigRepo
// =============================================================================

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

// =============================================================================
// 数据看板 DashboardRepo
// =============================================================================

// DashboardRepo 看板数据访问。
type DashboardRepo struct {
	db *gorm.DB
}

// NewDashboardRepo 创建 DashboardRepo 实例。
func NewDashboardRepo(db *gorm.DB) *DashboardRepo {
	return &DashboardRepo{db: db}
}

func (r *DashboardRepo) CountTodayTickets(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM tickets WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&count).Error
	return count, err
}

func (r *DashboardRepo) CountByStatus(ctx context.Context, status int16) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM tickets WHERE status = ?", status).Scan(&count).Error
	return count, err
}

func (r *DashboardRepo) CountTodayChats(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM chat_sessions WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&count).Error
	return count, err
}

func (r *DashboardRepo) AvgTodayConfidence(ctx context.Context) (float64, error) {
	var avg float64
	err := r.db.WithContext(ctx).Raw(
		"SELECT COALESCE(AVG(confidence), 0) FROM chat_sessions WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&avg).Error
	return avg, err
}

// CountFeedbackByType 按反馈类型统计 chat_messages 表中的反馈数。
// feedbackType: 1=有帮助, 2=无帮助。
func (r *DashboardRepo) CountFeedbackByType(ctx context.Context, feedbackType int16) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM chat_messages WHERE feedback = ?", feedbackType,
	).Scan(&count).Error
	return count, err
}

func (r *DashboardRepo) CountKnowledgeArticles(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM knowledge_articles").Scan(&count).Error
	return count, err
}

// TrendPoint 趋势数据点。
type TrendPoint struct {
	Date  string
	Count int64
}

// GetTicketTrends 按天/周聚合申告创建趋势。
// 使用 CTE 先计算 date_trunc 再 GROUP BY，避免 TO_CHAR 与 CASE 表达式不一致
// 导致的 PostgreSQL 42803 错误（column must appear in GROUP BY clause）。
func (r *DashboardRepo) GetTicketTrends(ctx context.Context, startDate, endDate, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	err := r.db.WithContext(ctx).Raw(
		`WITH raw AS (
  SELECT CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END AS truncated
  FROM tickets
  WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
)
SELECT TO_CHAR(truncated, 'YYYY-MM-DD') AS date, COUNT(*) AS count
FROM raw
GROUP BY truncated
ORDER BY truncated`,
		trunc, startDate, endDate,
	).Scan(&points).Error
	return points, err
}

// GetChatTrends 按天/周聚合问答趋势。
func (r *DashboardRepo) GetChatTrends(ctx context.Context, startDate, endDate string, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	err := r.db.WithContext(ctx).Raw(
		`WITH raw AS (
  SELECT CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END AS truncated
  FROM chat_sessions
  WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
)
SELECT TO_CHAR(truncated, 'YYYY-MM-DD') AS date, COUNT(*) AS count
FROM raw
GROUP BY truncated
ORDER BY truncated`,
		trunc, startDate, endDate,
	).Scan(&points).Error
	return points, err
}
