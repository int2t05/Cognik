// Package dashboard 数据看板业务逻辑。
package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"opsmind/internal/shared/dto/request"
	respDto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/pkg/errcode"
)

const maxTrendDays = 90

// dashboardRepo 看板仓库接口（消费者定义）。
type dashboardRepo interface {
	CountTodayTickets(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status int16) (int64, error)
	CountTodayChats(ctx context.Context) (int64, error)
	AvgTodayConfidence(ctx context.Context) (float64, error)
	CountKnowledgeArticles(ctx context.Context) (int64, error)
	CountFeedbackByType(ctx context.Context, feedbackType int16) (int64, error)
	GetTicketTrends(ctx context.Context, startDate, endDate, granularity string) ([]TrendPoint, error)
	GetChatTrends(ctx context.Context, startDate, endDate string, granularity string) ([]TrendPoint, error)
}

// DashboardService 数据看板服务。
type DashboardService struct {
	repo dashboardRepo
}

// NewDashboardService 创建 DashboardService 实例。
func NewDashboardService(repo dashboardRepo) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetStats 获取看板统计数据（9 项查询并行执行）。
func (s *DashboardService) GetStats(ctx context.Context) (*respDto.StatsResponse, error) {
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		result   respDto.StatsResponse
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		once     sync.Once
	)

	setErr := func(err error) {
		once.Do(func() {
			firstErr = err
			cancel()
		})
	}

	wg.Add(9)
	go func() {
		defer wg.Done()
		count, err := s.repo.CountTodayTickets(queryCtx)
		mu.Lock()
		result.TodayTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 1)
		mu.Lock()
		result.PendingTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 2)
		mu.Lock()
		result.ProcessingTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountByStatus(queryCtx, 4)
		mu.Lock()
		result.ResolvedTickets = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountTodayChats(queryCtx)
		mu.Lock()
		result.TodayChats = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		avg, err := s.repo.AvgTodayConfidence(queryCtx)
		mu.Lock()
		result.AvgConfidence = avg
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountKnowledgeArticles(queryCtx)
		mu.Lock()
		result.KnowledgeCount = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountFeedbackByType(queryCtx, 1)
		mu.Lock()
		result.HelpfulFeedback = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		count, err := s.repo.CountFeedbackByType(queryCtx, 2)
		mu.Lock()
		result.UnhelpfulFeedback = count
		mu.Unlock()
		if err != nil {
			setErr(err)
		}
	}()

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return &result, nil
}

// GetTrends 获取趋势数据（day/week 粒度，上限 90 天）。
func (s *DashboardService) GetTrends(ctx context.Context, req request.TrendRequest) (*respDto.TrendResponse, error) {
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

	granularity := req.Granularity
	if granularity != "week" {
		granularity = "day"
	}

	var labels []string
	if granularity == "week" {
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

	dataPoints := make([]respDto.DataPoint, len(labels))
	for i, d := range labels {
		dataPoints[i] = respDto.DataPoint{Date: d, TicketCount: 0, ChatCount: 0}
	}

	ticketCounts, err := s.repo.GetTicketTrends(ctx, req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日申告数失败: %w", err)
	}
	ticketMap := make(map[string]int64, len(ticketCounts))
	for _, tc := range ticketCounts {
		ticketMap[tc.Date] = tc.Count
	}

	chatCounts, err := s.repo.GetChatTrends(ctx, req.StartDate, req.EndDate, granularity)
	if err != nil {
		return nil, fmt.Errorf("查询每日问答数失败: %w", err)
	}
	chatMap := make(map[string]int64, len(chatCounts))
	for _, cc := range chatCounts {
		chatMap[cc.Date] = cc.Count
	}

	for i, dp := range dataPoints {
		dataPoints[i].TicketCount = ticketMap[dp.Date]
		dataPoints[i].ChatCount = chatMap[dp.Date]
	}

	return &respDto.TrendResponse{DataPoints: dataPoints}, nil
}
