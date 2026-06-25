// Package request 定义申告管理相关请求 DTO。
//
// 校验规则：标题、描述、手机号为必填；紧急程度 1-3 范围校验。
package request

// CreateTicketRequest 创建申告请求。
//
// Tags 为逗号分隔的标签（如 "网络,VPN,邮箱"），替代原来的 Urgency/ImpactScope/AffectedSystems。
// 与知识库文章 Tags 字段互通——生成知识候选时标签直接复制。
// ChatContext 仅当从智能问答转申告时才有值。
type CreateTicketRequest struct {
	Title        string           `json:"title" binding:"required"`
	Description  string           `json:"description" binding:"required"`
	Tags         []string         `json:"tags"`
	ContactPhone string           `json:"contact_phone" binding:"required"`
	ContactEmail string           `json:"contact_email"`
	ChatContext  *ChatContextData `json:"chat_context"` // 从问答转申告时带入
}

// ChatContextData 申告关联的问答上下文（结构化，替代 JSON 字符串）。
type ChatContextData struct {
	SessionID  int64   `json:"session_id"`
	Question   string  `json:"question"`
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
}

// UpdateTicketRequest 编辑申告请求。
//
// 仅申告人可编辑，仅待处理/处理中状态可编辑。
// 所有字段均可选——仅更新非空字段。
type UpdateTicketRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	ContactPhone string   `json:"contact_phone"`
	ContactEmail string   `json:"contact_email"`
}

// SupplementTicketRequest 补充申告信息请求。
//
// 仅申告人可在"需补充信息"状态下操作。
type SupplementTicketRequest struct {
	Content string `json:"content" binding:"required"`
}

// UpdateTicketStatusRequest 更新申告状态请求。
//
// Action 取值：
//
//	start        — 待处理(1) → 处理中(2)
//	request_info — 处理中(2) → 需补充信息(3)，supplement_count +1
//	resolve      — 处理中(2) → 已解决(4)
//	close        — 任意状态 → 已关闭(5)
//
// ToKnowledgeCandidate 为 true 时，resolve 操作会将此申告标记为知识候选。
type UpdateTicketStatusRequest struct {
	Action                string `json:"action" binding:"required,oneof=start request_info resolve close"`
	Result                string `json:"result"`
	ToKnowledgeCandidate  bool   `json:"to_knowledge_candidate"`
}

// BatchDeleteRequest 批量删除请求（通用，供申告/用户/审计日志复用）。
type BatchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// CreateTicketRecordRequest 创建处理记录请求（不影响状态）。
//
// Detail 为 JSONB 字段，用于存储回访结果等结构化数据。
type CreateTicketRecordRequest struct {
	Action  string `json:"action" binding:"required"`
	Content string `json:"content"`
	Detail  string `json:"detail"` // JSON 字符串
}
