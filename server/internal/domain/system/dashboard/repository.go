// Package dashboard 封装数据看板领域的数据访问层。
//
// repository.go 执行看板聚合查询：今日申告数、状态分布、问答趋势等。
package dashboard

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

// TrendPoint 趋势数据点。
type TrendPoint struct {
	Date  string
	Count int64
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

// GetTicketTrends 按天/周聚合申告创建趋势。
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
