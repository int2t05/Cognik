// Package chat 聚合智能问答领域（会话管理、RAG+LLM 编排、LLM 配置）的
// Handler / Service / Repository 三层实现。
//
// repository.go 合并原 chat_repo / llm_config_repo / message_repo 三个数据访问实现，
// 封装 chat_sessions、chat_messages、llm_configs、messages 四张表的 CRUD 操作。
package chat

import (
	"context"

	"opsmind/internal/shared/model"

	"gorm.io/gorm"
)

// ErrNotFound 导出哨兵供跨包错误比较。
var ErrNotFound = gorm.ErrRecordNotFound

// =============================================================================
// ChatRepo — 问答会话与消息数据访问
// =============================================================================

// ChatRepo 问答数据访问
type ChatRepo struct {
	db *gorm.DB
}

// NewChatRepo 创建 ChatRepo 实例
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

// UpdateSessionMeta 更新会话元数据（标题 + 知识库），仅会话所有者可调用。
// 与 UpdateSession 分离的原因：元数据由前端主动编辑，answer/sources 由流式生成自动写入，职责不同。
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

// DeleteMessage 按主键删除单条消息。
func (r *ChatRepo) DeleteMessage(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ChatMessage{}, id).Error
}

// CleanFailedMessages 删除会话中所有失败的 assistant 消息及其配对 user 消息。
//
// 配对规则：failed assistant 的配对 user 是同一会话中 ID 小于 assistant 的最近一条 user 消息。
// 这样进入会话页面时自动清理残留的失败消息对，前端无需感知"失败"状态。
//
// 返回删除的消息对数（每对含 1 user + 1 assistant）。
func (r *ChatRepo) CleanFailedMessages(ctx context.Context, sessionID int64) (int64, error) {
	// 1. 查所有 failed assistant 消息
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

	// 2. 为每个 failed assistant 查找其配对的 user 消息
	//    规则：同一会话中 ID 小于 assistant 的最近一条 user 消息
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

	// 3. 批量删除
	if len(toDelete) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Delete(&model.ChatMessage{}, toDelete)
	return res.RowsAffected, res.Error
}

// UpdateMessage 按主键全量更新一条消息（含 Status/Content/Sources 等）。
func (r *ChatRepo) UpdateMessage(ctx context.Context, m *model.ChatMessage) error {
	return r.db.WithContext(ctx).Model(&model.ChatMessage{ID: m.ID}).
		Select("content", "sources", "pipeline_metrics", "confidence_raw", "status").
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

// QueryRawScores 查询最近 N 天内 assistant 消息的原始置信度分数。
//
// 用于分位数计算，不过滤 confidence_raw=0（低分本身是有效信号）。
// days 为 0 或负数时默认 7 天。
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

// =============================================================================
// LlmConfigRepo — LLM 配置数据访问
// =============================================================================

// LlmConfigRepo LLM 配置数据访问。
type LlmConfigRepo struct {
	db *gorm.DB
}

// NewLlmConfigRepo 创建 LlmConfigRepo 实例。
func NewLlmConfigRepo(db *gorm.DB) *LlmConfigRepo {
	return &LlmConfigRepo{db: db}
}

// DB 返回底层 *gorm.DB，供 Service 层事务操作使用。
func (r *LlmConfigRepo) DB() *gorm.DB {
	return r.db
}

func (r *LlmConfigRepo) Create(ctx context.Context, cfg *model.LlmConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *LlmConfigRepo) FindByID(ctx context.Context, id int64) (*model.LlmConfig, error) {
	var cfg model.LlmConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FindDefault 查询默认配置。
// 数据库层已有部分唯一索引 idx_llm_configs_default (WHERE is_default=true) 兜底。
// 未找到默认配置时返回 nil, nil（静默降级，不视为错误）。
//
// 为什么用 Limit(1).Find 而非 First：
// GORM 的 First 在无记录时会返回 ErrRecordNotFound 并在日志打印 "record not found"，
// 对用户产生误导——以为初始化失败。Limit(1).Find 对空结果不报错，静默返回空切片。
func (r *LlmConfigRepo) FindDefault(ctx context.Context) (*model.LlmConfig, error) {
	var cfgs []model.LlmConfig
	if err := r.db.WithContext(ctx).Where("is_default = ?", true).Limit(1).Find(&cfgs).Error; err != nil {
		return nil, err
	}
	if len(cfgs) == 0 {
		return nil, nil
	}
	return &cfgs[0], nil
}

func (r *LlmConfigRepo) List(ctx context.Context) ([]model.LlmConfig, error) {
	var configs []model.LlmConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&configs).Error
	return configs, err
}

func (r *LlmConfigRepo) Update(ctx context.Context, cfg *model.LlmConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *LlmConfigRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.LlmConfig{}, id).Error
}

func (r *LlmConfigRepo) CountReferencingKBs(ctx context.Context, configID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.KnowledgeBase{}).Where("llm_config_id = ?", configID).Count(&count).Error
	return count, err
}

// ClearDefault 清空所有默认标志。
func (r *LlmConfigRepo) ClearDefault(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&model.LlmConfig{}).Where("is_default = ?", true).Update("is_default", false).Error
}

// =============================================================================
// MessageRepo — 站内消息数据访问
// =============================================================================

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
