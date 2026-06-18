// Package repository 提供数据访问层。
//
// dashboard_repo.go 定义看板聚合查询的数据访问方法。
package repository

import (
	"context"

	"gorm.io/gorm"
)

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

func (r *DashboardRepo) GetTicketTrends(ctx context.Context, startDate, endDate, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	// 使用 CASE WHEN 替代字符串拼接，避免 SQL 注入风险。
	err := r.db.WithContext(ctx).Raw(
		`SELECT TO_CHAR(CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END, 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM tickets
		 WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
		 GROUP BY CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END
		 ORDER BY CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END`,
		trunc, startDate, endDate, trunc, trunc,
	).Scan(&points).Error
	return points, err
}

func (r *DashboardRepo) GetChatTrends(ctx context.Context, startDate, endDate string, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	err := r.db.WithContext(ctx).Raw(
		`SELECT TO_CHAR(CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END, 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM chat_sessions
		 WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
		 GROUP BY CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END
		 ORDER BY CASE WHEN ? = 'week' THEN date_trunc('week', created_at) ELSE date_trunc('day', created_at) END`,
		trunc, startDate, endDate, trunc, trunc,
	).Scan(&points).Error
	return points, err
}
