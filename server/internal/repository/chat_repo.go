// Package repository 提供问答会话的数据访问层。
//
// ChatRepo 封装 chat_sessions 和 chat_messages 表的 CRUD 操作，供 ChatService 调用。
// 为什么独立于 UserRepo：问答涉及会话查询、消息批量写入、反馈更新等独立操作，
// 独立 Repo 更利于聚焦和维护。
package repository

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// ChatRepo 问答数据访问
type ChatRepo struct {
	db *gorm.DB
}

// NewChatRepo 创建 ChatRepo 实例
func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

// =============================================================================
// ChatSession
// =============================================================================

// Create 创建问答会话。
//
// 创建后 session.ID 会被 GORM 自动填充。
func (r *ChatRepo) Create(session *model.ChatSession) error {
	return r.db.Create(session).Error
}

// FindByID 按 ID 查询问答会话。
func (r *ChatRepo) FindByID(id int64) (*model.ChatSession, error) {
	// TODO(repository/chat): FindByID 应支持 userID 条件，用于门户端防止水平越权。
	// 只按 session id 查询会把授权判断推到更上层且容易遗漏。
	var session model.ChatSession
	err := r.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateFeedback 更新问答会话的反馈状态。
//
// feedback: 0=未评价, 1=已解决, 2=未解决。
// 为什么只更新 feedback 字段：反馈是独立操作，只改单个字段避免全量 Save
// 意外覆盖其他列。
func (r *ChatRepo) UpdateFeedback(id int64, feedback int16) error {
	return r.db.Model(&model.ChatSession{}).Where("id = ?", id).
		Update("feedback", feedback).Error
}

// ListByUser 分页查询指定用户的问答会话列表。
//
// 按 created_at DESC 排序（最新在前），返回总数和列表。
func (r *ChatRepo) ListByUser(userID int64, page, pageSize int) ([]model.ChatSession, int64, error) {
	var sessions []model.ChatSession
	var total int64

	query := r.db.Model(&model.ChatSession{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// =============================================================================
// ChatMessage
// =============================================================================

// CreateBatch 批量创建对话消息。
//
// 为什么用批量插入：一次问答产生 2 条消息（用户问题 + AI 回答），
// 批量插入减少网络往返。GORM Create 支持切片参数。
// 空切片调用不会报错（GORM 跳过空切片插入）。
func (r *ChatRepo) CreateBatch(messages []model.ChatMessage) error {
	if len(messages) == 0 {
		return nil
	}
	return r.db.Create(&messages).Error
}

// FindMessagesBySession 查询会话的全部消息（按时间正序）。
//
// 用于多轮对话——加载历史消息作为 LLM 上下文。
// 限制最多返回 50 条，避免超长历史撑爆 prompt。
func (r *ChatRepo) FindMessagesBySession(sessionID int64) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).
		Order("created_at ASC").Limit(50).
		Find(&messages).Error
	return messages, err
}

// UpdateSession 更新会话的答案/置信度/耗时字段。
//
// 流式完成后，用完整答案回填预创建的 session。
// 只更新特定字段，避免 Save 覆盖其他并发写入。
func (r *ChatRepo) UpdateSession(session *model.ChatSession) error {
	return r.db.Model(&model.ChatSession{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
		"answer":      session.Answer,
		"sources":     session.Sources,
		"confidence":  session.Confidence,
		"duration_ms": session.DurationMs,
	}).Error
}

// DeleteSession 删除会话及其全部消息。
//
// 必须同时传入 userID 防止水平越权——仅允许删除自己的会话。
// 先删消息再删会话，保证外键一致性（若将来有 FK 约束）。
func (r *ChatRepo) DeleteSession(id, userID int64) error {
	if err := r.db.Where("session_id = ?", id).Delete(&model.ChatMessage{}).Error; err != nil {
		return err
	}
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ChatSession{}).Error
}

// CountMessagesBySession 统计会话的消息数量。
func (r *ChatRepo) CountMessagesBySession(sessionID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.ChatMessage{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

// CountMessagesBySessions 批量统计多个会话的消息数量，避免 N+1 查询。
func (r *ChatRepo) CountMessagesBySessions(sessionIDs []int64) (map[int64]int64, error) {
	if len(sessionIDs) == 0 {
		return map[int64]int64{}, nil
	}
	type row struct {
		SessionID int64
		Count     int64
	}
	var rows []row
	err := r.db.Model(&model.ChatMessage{}).
		Select("session_id, COUNT(*) as count").
		Where("session_id IN ?", sessionIDs).
		Group("session_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(rows))
	for _, r := range rows {
		m[r.SessionID] = r.Count
	}
	return m, nil
}
