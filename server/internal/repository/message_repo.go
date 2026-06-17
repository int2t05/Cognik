// Package repository 提供站内消息的数据访问层。
//
// MessageRepo 封装 messages 表的 CRUD 操作，供 MessageService 调用。
// 为什么独立 Repo：消息表有特定的查询模式（按用户+已读状态过滤），
// 独立 Repo 更利于职责聚焦。
package repository

import (
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

// Create 创建站内消息。
func (r *MessageRepo) Create(msg *model.Message) error {
	return r.db.Create(msg).Error
}

// MessageFilter 消息列表过滤条件。
//
// 零值表示不过滤（isRead 用指针区分未设置 vs false）。
type MessageFilter struct {
	IsRead *bool
	Type   string
}

// ListByUser 分页查询指定用户的消息列表，支持按 is_read/type 过滤。
//
// 按 created_at DESC 排序（最新在前），返回总数和列表。
func (r *MessageRepo) ListByUser(userID int64, page, pageSize int, filter MessageFilter) ([]model.Message, int64, error) {
	var messages []model.Message
	var total int64

	query := r.db.Model(&model.Message{}).Where("user_id = ?", userID)
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

// MarkAsRead 将指定用户的消息标记为已读。
//
// 同时校验 id 和 user_id，防止用户 A 标记用户 B 的消息已读（水平越权）。
// 为什么用 Update 而非 Save：仅更新 is_read 字段，避免意外覆盖其他列。
// 消息不存在或不属于该用户时返回 gorm.ErrRecordNotFound。
func (r *MessageRepo) MarkAsRead(id int64, userID int64) error {
	result := r.db.Model(&model.Message{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountUnread 查询指定用户的未读消息数。
func (r *MessageRepo) CountUnread(userID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Message{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}
