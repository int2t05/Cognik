// Package service 实现站内消息业务逻辑。
//
// MessageService 提供消息查询、标记已读、未读计数功能，
// 以及被 TicketService / KnowledgeService 调用的各类通知方法。
//
// 通知方法命名约定：Notify<触发事件>，由业务 Service 在处理完成后同步调用。
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/gorm"
)

// MessageService 站内消息服务。
type MessageService struct {
	repo        *repository.MessageRepo
	cacheTTL    time.Duration
	unreadMu    sync.RWMutex
	unreadCache map[int64]unreadCountCacheEntry
}

type unreadCountCacheEntry struct {
	count     int64
	expiresAt time.Time
}

const defaultUnreadCountCacheTTL = 15 * time.Second

// NewMessageService 创建 MessageService 实例。
func NewMessageService(repo *repository.MessageRepo) *MessageService {
	return NewMessageServiceWithCacheTTL(repo, defaultUnreadCountCacheTTL)
}

// NewMessageServiceWithCacheTTL 创建 MessageService 实例，并允许测试覆盖未读数缓存 TTL。
func NewMessageServiceWithCacheTTL(repo *repository.MessageRepo, ttl time.Duration) *MessageService {
	return &MessageService{
		repo:        repo,
		cacheTTL:    ttl,
		unreadCache: make(map[int64]unreadCountCacheEntry),
	}
}

// =============================================================================
// 通知方法（被各业务 Service 调用）
// =============================================================================

func (s *MessageService) notify(ctx context.Context, userID int64, title, content, msgType, relatedType string, relatedID int64) error {
	msg := &model.Message{
		UserID: userID, Title: title, Content: content,
		Type: msgType, RelatedType: relatedType, RelatedID: relatedID, IsRead: false,
	}
	if err := s.repo.Create(ctx, msg); err != nil {
		return err
	}
	s.invalidateUnread(userID)
	return nil
}

// NotifySupplement 通知申告人补充信息（TicketService.request_info 调用）。
func (s *MessageService) NotifySupplement(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := "您的申告需要补充更多信息，请尽快登录系统查看并补充相关材料。"
	if ticketTitle != "" {
		content = fmt.Sprintf("您的申告「%s」需要补充更多信息，请尽快登录系统查看并补充相关材料。", ticketTitle)
	}
	return s.notify(ctx, userID, "申告需补充信息", content, "ticket_supplement", "ticket", ticketID)
}

// NotifyTicketResolved 通知申告人申告已解决（TicketService 状态变更为已解决时调用）。
func (s *MessageService) NotifyTicketResolved(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := fmt.Sprintf("您的申告「%s」已被标记为已解决，如有疑问请联系运维人员。", ticketTitle)
	return s.notify(ctx, userID, "申告已解决", content, "ticket_resolved", "ticket", ticketID)
}

// NotifyTicketClosed 通知申告人申告已关闭（TicketService 状态变更为已关闭时调用）。
func (s *MessageService) NotifyTicketClosed(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := fmt.Sprintf("您的申告「%s」已被关闭，如有需要请重新提交申告。", ticketTitle)
	return s.notify(ctx, userID, "申告已关闭", content, "ticket_closed", "ticket", ticketID)
}

// NotifyKnowledgeReviewed 通知文章作者审核结果（KnowledgeService.Review 调用）。
func (s *MessageService) NotifyKnowledgeReviewed(ctx context.Context, articleID int64, articleTitle string, userID int64, approved bool, comment string) error {
	if approved {
		content := fmt.Sprintf("您的文章「%s」已通过审核，可前往发布。", articleTitle)
		return s.notify(ctx, userID, "文章审核通过", content, "knowledge_approved", "knowledge_article", articleID)
	}
	content := fmt.Sprintf("您的文章「%s」已被驳回", articleTitle)
	if comment != "" {
		content += "，原因：" + comment
	}
	return s.notify(ctx, userID, "文章被驳回", content, "knowledge_rejected", "knowledge_article", articleID)
}

// =============================================================================
// 查询和操作
// =============================================================================

// ListMessages 分页查询用户消息列表，支持按 is_read/type 过滤。
func (s *MessageService) ListMessages(ctx context.Context, userID int64, page, pageSize int, filter repository.MessageFilter) ([]model.Message, int64, error) {
	if userID <= 0 {
		return nil, 0, AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	return s.repo.ListByUser(ctx, userID, page, pageSize, filter)
}

// MarkAsRead 将指定用户的消息标记为已读。
//
// 校验消息归属（userID），防止水平越权：用户 A 不能标记用户 B 的消息已读。
// 消息不存在或不属于该用户时返回 AppError{Code: ErrNotFound}。
func (s *MessageService) MarkAsRead(ctx context.Context, id int64, userID int64) error {
	if userID <= 0 {
		return AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	if err := s.repo.MarkAsRead(ctx, id, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "消息不存在"}
		}
		return err
	}
	s.invalidateUnread(userID)
	return nil
}

// MarkAsReadAndCount 标记消息已读并返回最新未读计数。
//
// 合并两次操作减少前端请求数：标记已读后直接返回 unread_count，
// 前端无需额外调用 CountUnread 即可更新未读角标。
func (s *MessageService) MarkAsReadAndCount(ctx context.Context, id int64, userID int64) (int64, error) {
	if err := s.MarkAsRead(ctx, id, userID); err != nil {
		return 0, err
	}
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.setCachedUnread(userID, count)
	return count, nil
}

// MarkAllRead 将用户所有未读消息标记为已读，返回操作影响的条数。
func (s *MessageService) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	affected, err := s.repo.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.invalidateUnread(userID)
	return affected, nil
}

// CountUnread 查询指定用户的未读消息数。
func (s *MessageService) CountUnread(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, AppError{Code: errcode.ErrParam, Message: "无效的用户 ID"}
	}
	if count, ok := s.getCachedUnread(userID); ok {
		return count, nil
	}
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.setCachedUnread(userID, count)
	return count, nil
}

func (s *MessageService) getCachedUnread(userID int64) (int64, bool) {
	if s.cacheTTL <= 0 {
		return 0, false
	}
	s.unreadMu.RLock()
	entry, ok := s.unreadCache[userID]
	s.unreadMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			s.invalidateUnread(userID)
		}
		return 0, false
	}
	return entry.count, true
}

func (s *MessageService) setCachedUnread(userID int64, count int64) {
	if s.cacheTTL <= 0 {
		return
	}
	s.unreadMu.Lock()
	s.unreadCache[userID] = unreadCountCacheEntry{
		count:     count,
		expiresAt: time.Now().Add(s.cacheTTL),
	}
	s.unreadMu.Unlock()
}

func (s *MessageService) invalidateUnread(userID int64) {
	if s.cacheTTL <= 0 {
		return
	}
	s.unreadMu.Lock()
	delete(s.unreadCache, userID)
	s.unreadMu.Unlock()
}
