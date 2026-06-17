// Package service 实现数据看板统计业务逻辑。
//
// DashboardService 提供看板统计和趋势数据查询功能。
// 聚合查询 SQL 封装在 repository.DashboardRepo 中，遵循 3 层架构。
package service

import (
	"fmt"
	"sync"
	"time"

	"opsmind/internal/dto/request"
	"opsmind/internal/dto/response"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"
)

const maxTrendDays = 90

// DashboardService 数据看板服务。
type DashboardService struct {
	repo *repository.DashboardRepo
}

// NewDashboardService 创建 DashboardService 实例。
func NewDashboardService(repo *repository.DashboardRepo) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetStats 获取看板统计数据（7 项查询并行执行）。
func (s *DashboardService) GetStats() (*response.StatsResponse, error) {
	var (
		resp         response.StatsResponse
		mu           sync.Mutex
		wg           sync.WaitGroup
		firstErr     error
		once         sync.Once
	)

	setErr := func(err error) {
		once.Do(func() { firstErr = err })
	}

	wg.Add(7)
	go func() { defer wg.Done(); c, e := s.repo.CountTodayTickets(); mu.Lock(); resp.TodayTickets = c; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); c, e := s.repo.CountByStatus(1); mu.Lock(); resp.PendingTickets = c; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); c, e := s.repo.CountByStatus(2); mu.Lock(); resp.ProcessingTickets = c; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); c, e := s.repo.CountByStatus(4); mu.Lock(); resp.ResolvedTickets = c; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); c, e := s.repo.CountTodayChats(); mu.Lock(); resp.TodayChats = c; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); a, e := s.repo.AvgTodayConfidence(); mu.Lock(); resp.AvgConfidence = a; mu.Unlock(); if e != nil { setErr(e) } }()
	go func() { defer wg.Done(); c, e := s.repo.CountKnowledgeArticles(); mu.Lock(); resp.KnowledgeCount = c; mu.Unlock(); if e != nil { setErr(e) } }()

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return &resp, nil
}

// GetTrends 获取趋势数据（支持 day/week 粒度，上限 90 天）。
func (s *DashboardService) GetTrends(req request.TrendRequest) (*response.TrendResponse, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "开始日期格式错误，格式应为 YYYY-MM-DD"}
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "结束日期格式错误，格式应为 YYYY-MM-DD"}
	}
	if endDate.Before(startDate) {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "结束日期不能早于开始日期"}
	}
	if endDate.Sub(startDate) > maxTrendDays*24*time.Hour {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: fmt.Sprintf("日期范围不能超过 %d 天", maxTrendDays)}
	}

	// 生成日期序列（按 day 或 week 粒度）
	granularity := req.Granularity
	if granularity != "week" {
		granularity = "day"
	}

	var labels []string
	if granularity == "week" {
		// 对齐到周一
		cur := startDate
		for cur.Weekday() != time.Monday {
			cur = cur.AddDate(0, 0, -1)
		}
		for !cur.After(endDate) {
			labels = append(labels, cur.Format("2006-01-02"))
			cur = cur.AddDate(0, 0, 7)
		}
	} else {
		cur := startDate
		for !cur.After(endDate) {
			labels = append(labels, cur.Format("2006-01-02"))
			cur = cur.AddDate(0, 0, 1)
		}
	}

	dataPoints := make([]response.DataPoint, len(labels))
	for i, d := range labels {
		dataPoints[i] = response.DataPoint{Date: d, TicketCount: 0, ChatCount: 0}
	}

	// 查询趋势（已支持 granularity 参数）
	ticketCounts, err := s.repo.GetTicketTrends(req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日申告数失败: %w", err)
	}
	ticketMap := make(map[string]int64, len(ticketCounts))
	for _, tc := range ticketCounts {
		ticketMap[tc.Date] = tc.Count
	}

	chatCounts, err := s.repo.GetChatTrends(req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日问答数失败: %w", err)
	}
	chatMap := make(map[string]int64, len(chatCounts))
	for _, cc := range chatCounts {
		chatMap[cc.Date] = cc.Count
	}

	// O(n) 填充（替代 O(n²) 双重循环）
	for i, dp := range dataPoints {
		dataPoints[i].TicketCount = ticketMap[dp.Date]
		dataPoints[i].ChatCount = chatMap[dp.Date]
	}

	return &response.TrendResponse{DataPoints: dataPoints}, nil
}
