// Package service 实现后台调度器。
//
// Scheduler 提供定时任务管理功能，当前包含：
// - TicketAutoCloseJob：每小时检查，关闭超过 7 天的申告
package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ticketAutoCloseService Scheduler 需要的 TicketService 方法子集（消费者定义接口）。
type ticketAutoCloseService interface {
	AutoClose(ctx context.Context, olderThan time.Time) (int64, error)
}

// Scheduler 后台调度器。
type Scheduler struct {
	ticketSvc ticketAutoCloseService
	once      sync.Once
	cancel    context.CancelFunc
}

// NewScheduler 创建 Scheduler 实例。
func NewScheduler(svc ticketAutoCloseService) *Scheduler {
	return &Scheduler{ticketSvc: svc}
}

// Start 启动调度器（幂等——重复调用无副作用）。
//
// 启动时立即执行一次 AutoClose，避免频繁重启时长期不清理超期申告。
// 随后每小时执行一次。
func (s *Scheduler) Start(ctx context.Context) {
	s.once.Do(func() {
		ctx, s.cancel = context.WithCancel(ctx)
		go s.runAutoCloseLoop(ctx)
		slog.Info("后台调度器已启动")
	})
}

// Stop 停止调度器，取消所有 ticker goroutine。
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
		slog.Info("后台调度器已停止")
	}
}

// runAutoCloseLoop 启动时立即执行一次，随后每小时执行 AutoClose。
func (s *Scheduler) runAutoCloseLoop(ctx context.Context) {
	// 启动时立即执行一次，防止频繁重启导致超期工单堆积
	s.doAutoClose()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.doAutoClose()
		}
	}
}

func (s *Scheduler) doAutoClose() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	closed, err := s.ticketSvc.AutoClose(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		slog.Error("自动关闭申告失败", "error", err)
	} else if closed > 0 {
		slog.Info("自动关闭申告完成", "count", closed)
	}
}

// RunAutoClose 手动关闭超期申告（暴露给测试使用）。
func (s *Scheduler) RunAutoClose(olderThan time.Time) (int64, error) {
	return s.ticketSvc.AutoClose(context.Background(), olderThan)
}
