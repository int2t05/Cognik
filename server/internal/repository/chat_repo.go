// Package repository 提供问答会话的数据访问层。
package repository

import (
	"context"

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

func (r *ChatRepo) Create(ctx context.Context, session *model.ChatSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *ChatRepo) FindByID(ctx context.Context, id int64) (*model.ChatSession, error) {
	var session model.ChatSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *ChatRepo) UpdateFeedback(ctx context.Context, id int64, feedback int16) error {
	return r.db.WithContext(ctx).Model(&model.ChatSession{}).Where("id = ?", id).
		Update("feedback", feedback).Error
}

func (r *ChatRepo) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]model.ChatSession, int64, error) {
	var sessions []model.ChatSession
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ChatSession{}).Where("user_id = ?", userID)

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

func (r *ChatRepo) CreateBatch(ctx context.Context, messages []model.ChatMessage) error {
	if len(messages) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&messages).Error
}

func (r *ChatRepo) FindMessagesBySession(ctx context.Context, sessionID int64) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("created_at ASC").Limit(50).
		Find(&messages).Error
	return messages, err
}

func (r *ChatRepo) UpdateSession(ctx context.Context, session *model.ChatSession) error {
	return r.db.WithContext(ctx).Model(&model.ChatSession{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
		"answer":      session.Answer,
		"sources":     session.Sources,
		"confidence":  session.Confidence,
		"duration_ms": session.DurationMs,
	}).Error
}

func (r *ChatRepo) DeleteSession(ctx context.Context, id, userID int64) error {
	if err := r.db.WithContext(ctx).Where("session_id = ?", id).Delete(&model.ChatMessage{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.ChatSession{}).Error
}

func (r *ChatRepo) CountMessagesBySession(ctx context.Context, sessionID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

// FindMessageByID 按 ID 和 sessionID 查找单条消息（含会话归属校验）。
func (r *ChatRepo) FindMessageByID(ctx context.Context, messageID, sessionID int64) (*model.ChatMessage, error) {
	var msg model.ChatMessage
	err := r.db.WithContext(ctx).Where("id = ? AND session_id = ?", messageID, sessionID).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// UpdateMessageFeedback 更新单条消息的反馈值。
func (r *ChatRepo) UpdateMessageFeedback(ctx context.Context, messageID int64, feedback int16) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("id = ?", messageID).
		Update("feedback", feedback).Error
}

// CreateMessage 单条写入消息并回填自增 ID。
// 为什么单写：可续传方案要在生成开始时先建一条 generating 的 assistant 消息，
// 拿到 ID 后于完成时再 UpdateMessage 回填内容，区别于一次性 CreateBatch。
func (r *ChatRepo) CreateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// UpdateMessage 按主键全量更新一条消息（含 Status/Content/Sources 等）。
func (r *ChatRepo) UpdateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{ID: m.ID}).
		Select("content", "sources", "pipeline_metrics", "confidence", "status").
		Updates(m).Error
}

// MarkGeneratingFailed 将所有残留 generating 消息标记为 failed。
// 为什么需要：内存 Hub 在服务重启后丢失进行中的生成，避免前端永久卡「生成中」。
func (r *ChatRepo) MarkGeneratingFailed(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("status = ?", model.MessageStatusGenerating).
		Update("status", model.MessageStatusFailed)
	return res.RowsAffected, res.Error
}

func (r *ChatRepo) CountMessagesBySessions(ctx context.Context, sessionIDs []int64) (map[int64]int64, error) {
	if len(sessionIDs) == 0 {
		return map[int64]int64{}, nil
	}
	type row struct {
		SessionID int64
		Count     int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
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
// FindFeedbackSamples 查询最近 N 天内有反馈的消息样本（含用户问题）。
//
// 使用 LATERAL JOIN 为每条有反馈的 assistant 消息匹配最近的前一条 user 消息。
// limitDays=0 时默认 30 天。
func (r *ChatRepo) FindFeedbackSamples(ctx context.Context, limitDays int) ([]model.FeedbackSample, error) {
	if limitDays <= 0 {
		limitDays = 30
	}
	var samples []model.FeedbackSample
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			cm.id AS message_id,
			cm.session_id,
			prev.content AS question,
			cm.content AS answer,
			cm.feedback,
			cm.confidence,
			TO_CHAR(cm.created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at
		FROM chat_messages cm
		CROSS JOIN LATERAL (
			SELECT content FROM chat_messages prev
			WHERE prev.session_id = cm.session_id
			  AND prev.role = 'user'
			  AND prev.id < cm.id
			ORDER BY prev.id DESC
			LIMIT 1
		) prev
		WHERE cm.feedback > 0
		  AND cm.role = 'assistant'
		  AND cm.created_at >= NOW() - ($1::text || ' days')::INTERVAL
		ORDER BY cm.created_at DESC
	`, limitDays).Scan(&samples).Error
	return samples, err
}
