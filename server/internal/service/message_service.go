// Package service 实现站内消息业务逻辑。
//
// MessageService 提供消息查询、标记已读、未读计数功能，以及
// 被 TicketService 调用的 NotifySupplement 方法。
//
// 为什么 NotifySupplement 在 MessageService 而非 TicketService 中：
// 消息的格式化和写入属于消息领域的职责，TicketService 只需调用
// NotifySupplement 即可，无需关心消息内容。
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
// NotifySupplement（被 TicketService 调用）
// =============================================================================

// NotifySupplement 通知申告人补充信息。
//
// 当 TicketService 执行 request_info 操作时同步调用此方法，
// 写入一条 type=ticket_supplement 的站内消息。
// 为什么同步调用而非异步：消息写入是轻量操作（单条 INSERT），
// 同步执行可保证事务一致性——如果消息创建失败，申告操作也告失败。
func (s *MessageService) NotifySupplement(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	content := "您的申告需要补充更多信息，请尽快登录系统查看并补充相关材料。"
	if ticketTitle != "" {
		content = fmt.Sprintf("您的申告「%s」需要补充更多信息，请尽快登录系统查看并补充相关材料。", ticketTitle)
	}
	msg := &model.Message{
		UserID:      userID,
		Title:       "申告需补充信息",
		Content:     content,
		Type:        "ticket_supplement",
		RelatedType: "ticket",
		RelatedID:   ticketID,
		IsRead:      false,
	}
	if err := s.repo.Create(ctx, msg); err != nil {
		return err
	}
	s.invalidateUnread(userID)
	return nil
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
