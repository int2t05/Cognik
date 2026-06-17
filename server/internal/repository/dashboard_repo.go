// Package repository 提供数据访问层。
//
// dashboard_repo.go 定义看板聚合查询的数据访问方法。
package repository

import (
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

// CountTodayTickets 今日新增申告数（使用范围查询保持索引可用）。
func (r *DashboardRepo) CountTodayTickets() (int64, error) {
	var count int64
	err := r.db.Raw(
		"SELECT COUNT(*) FROM tickets WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&count).Error
	return count, err
}

// CountByStatus 按状态统计申告数。
func (r *DashboardRepo) CountByStatus(status int16) (int64, error) {
	var count int64
	err := r.db.Raw("SELECT COUNT(*) FROM tickets WHERE status = ?", status).Scan(&count).Error
	return count, err
}

// CountTodayChats 今日问答数（使用范围查询保持索引可用）。
func (r *DashboardRepo) CountTodayChats() (int64, error) {
	var count int64
	err := r.db.Raw(
		"SELECT COUNT(*) FROM chat_sessions WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&count).Error
	return count, err
}

// AvgTodayConfidence 今日平均置信度（使用范围查询保持索引可用）。
func (r *DashboardRepo) AvgTodayConfidence() (float64, error) {
	var avg float64
	err := r.db.Raw(
		"SELECT COALESCE(AVG(confidence), 0) FROM chat_sessions WHERE created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'",
	).Scan(&avg).Error
	return avg, err
}

// CountKnowledgeArticles 知识条目总数。
func (r *DashboardRepo) CountKnowledgeArticles() (int64, error) {
	var count int64
	err := r.db.Raw("SELECT COUNT(*) FROM knowledge_articles").Scan(&count).Error
	return count, err
}

// TrendPoint 趋势数据点。
type TrendPoint struct {
	Date  string
	Count int64
}

// GetTicketTrends 获取指定日期范围内的每日/每周申告数（使用范围查询保持索引可用）。
func (r *DashboardRepo) GetTicketTrends(startDate, endDate string, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	err := r.db.Raw(
		`SELECT TO_CHAR(date_trunc('`+trunc+`', created_at), 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM tickets
		 WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
		 GROUP BY date_trunc('`+trunc+`', created_at)
		 ORDER BY date_trunc('`+trunc+`', created_at)`,
		startDate, endDate,
	).Scan(&points).Error
	return points, err
}

// GetChatTrends 获取指定日期范围内的每日/每周问答数。
func (r *DashboardRepo) GetChatTrends(startDate, endDate string, granularity string) ([]TrendPoint, error) {
	var points []TrendPoint
	trunc := "day"
	if granularity == "week" {
		trunc = "week"
	}
	err := r.db.Raw(
		`SELECT TO_CHAR(date_trunc('`+trunc+`', created_at), 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM chat_sessions
		 WHERE created_at >= ?::date AND created_at < (?::date + INTERVAL '1 day')
		 GROUP BY date_trunc('`+trunc+`', created_at)
		 ORDER BY date_trunc('`+trunc+`', created_at)`,
		startDate, endDate,
	).Scan(&points).Error
	return points, err
}
