// Package repository 提供站内消息的数据访问层。
//
// MessageRepo 封装 messages 表的 CRUD 操作，供 MessageService 调用。
package repository

import (
	"context"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// MessageRepo 站内消息数据访问
type MessageRepo struct {
	db *gorm.DB
}

// NewMessageRepo 创建 MessageRepo 实例
func NewMessageRepo(db *gorm.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) Create(ctx context.Context, msg *model.Message) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// MessageFilter 消息列表过滤条件。
type MessageFilter struct {
	IsRead *bool
	Type   string
}

func (r *MessageRepo) ListByUser(ctx context.Context, userID int64, page, pageSize int, filter MessageFilter) ([]model.Message, int64, error) {
	var messages []model.Message
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Message{}).Where("user_id = ?", userID)
	if filter.IsRead != nil {
		query = query.Where("is_read = ?", *filter.IsRead)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *MessageRepo) MarkAsRead(ctx context.Context, id int64, userID int64) error {
	result := r.db.WithContext(ctx).Model(&model.Message{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *MessageRepo) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true)
	return res.RowsAffected, res.Error
}

func (r *MessageRepo) CountUnread(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}
