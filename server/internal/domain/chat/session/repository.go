// Package session 封装智能问答会话领域的数据访问层。
//
// repository.go 管理 chat_sessions 与 chat_messages 两张表的 CRUD。
// 问答会话生命周期：创建 → 流式消息 → 完成/失败/取消。
package session

import (
	"context"

	"opsmind/internal/shared/model"

	"gorm.io/gorm"
)

// ErrNotFound 导出哨兵供跨包错误比较。
var ErrNotFound = gorm.ErrRecordNotFound

// ChatRepo 问答数据访问。
type ChatRepo struct {
	db *gorm.DB
}

// NewChatRepo 创建 ChatRepo 实例。
func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

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

func (r *ChatRepo) UpdateSessionMeta(ctx context.Context, sessionID int64, question string, kbID int64) error {
	updates := map[string]interface{}{}
	if question != "" {
		updates["question"] = question
	}
	if kbID > 0 {
		updates["kb_id"] = kbID
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.ChatSession{}).Where("id = ?", sessionID).Updates(updates).Error
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

func (r *ChatRepo) FindMessageByID(ctx context.Context, messageID, sessionID int64) (*model.ChatMessage, error) {
	var msg model.ChatMessage
	err := r.db.WithContext(ctx).Where("id = ? AND session_id = ?", messageID, sessionID).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *ChatRepo) UpdateMessageFeedback(ctx context.Context, messageID int64, feedback int16) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("id = ?", messageID).
		Update("feedback", feedback).Error
}

func (r *ChatRepo) CreateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *ChatRepo) DeleteMessage(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ChatMessage{}, id).Error
}

// CleanFailedMessages 删除会话中所有失败的 assistant 消息及其配对 user 消息。
func (r *ChatRepo) CleanFailedMessages(ctx context.Context, sessionID int64) (int64, error) {
	var failed []model.ChatMessage
	if err := r.db.WithContext(ctx).
		Where("session_id = ? AND role = ? AND status = ?", sessionID, "assistant", model.MessageStatusFailed).
		Order("id ASC").
		Find(&failed).Error; err != nil {
		return 0, err
	}
	if len(failed) == 0 {
		return 0, nil
	}

	var toDelete []int64
	for _, f := range failed {
		var user model.ChatMessage
		err := r.db.WithContext(ctx).
			Where("session_id = ? AND role = ? AND id < ?", sessionID, "user", f.ID).
			Order("id DESC").
			First(&user).Error
		if err == nil {
			toDelete = append(toDelete, user.ID)
		}
		toDelete = append(toDelete, f.ID)
	}

	if len(toDelete) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Delete(&model.ChatMessage{}, toDelete)
	return res.RowsAffected, res.Error
}

func (r *ChatRepo) UpdateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{ID: m.ID}).
		Select("content", "sources", "pipeline_metrics", "confidence_raw", "status").
		Updates(m).Error
}

// MarkGeneratingFailed 将所有残留 generating 消息标记为 failed。
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

// QueryRawScores 查询最近 N 天内 assistant 消息的原始置信度分数。
func (r *ChatRepo) QueryRawScores(ctx context.Context, days int) ([]float64, error) {
	if days <= 0 {
		days = 7
	}
	var scores []float64
	err := r.db.WithContext(ctx).Raw(`
		SELECT confidence_raw FROM chat_messages
		WHERE role = 'assistant'
		  AND status = 'completed'
		  AND confidence_raw IS NOT NULL
		  AND content != ''
		  AND created_at >= NOW() - make_interval(days => $1)
		ORDER BY confidence_raw`, days).Scan(&scores).Error
	return scores, err
}

// FindFeedbackSamples 查询最近 N 天内有反馈的消息样本。
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
			cm.confidence_raw AS confidence,
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
		  AND cm.created_at >= NOW() - make_interval(days => $1)
		ORDER BY cm.created_at DESC
	`, limitDays).Scan(&samples).Error
	return samples, err
}
