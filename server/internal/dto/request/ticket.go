// Package request 定义申告管理相关请求 DTO。
//
// 校验规则：标题、描述、手机号为必填；紧急程度 1-3 范围校验。
package request

// CreateTicketRequest 创建申告请求。
//
// ChatContext 和 AffectedSystems 为可选字段，
// 仅当从智能问答转申告时 ChatContext 才有值。
type CreateTicketRequest struct {
	Title           string   `json:"title" binding:"required"`
	Description     string   `json:"description" binding:"required"`
	Urgency         int      `json:"urgency" binding:"required"`
	ImpactScope     int      `json:"impact_scope"`
	AffectedSystems []string `json:"affected_systems"`
	ContactPhone    string   `json:"contact_phone" binding:"required"`
	ContactEmail    string          `json:"contact_email"`
	ChatContext     *ChatContextData `json:"chat_context"` // 从问答转申告时带入
}

// ChatContextData 申告关联的问答上下文（结构化，替代 JSON 字符串）。
type ChatContextData struct {
	SessionID  int64   `json:"session_id"`
	Question   string  `json:"question"`
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
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

// CreateTicketRecordRequest 创建处理记录请求（不影响状态）。
//
// Detail 为 JSONB 字段，用于存储回访结果等结构化数据。
type CreateTicketRecordRequest struct {
	Action  string `json:"action" binding:"required"`
	Content string `json:"content"`
	Detail  string `json:"detail"` // JSON 字符串
}
