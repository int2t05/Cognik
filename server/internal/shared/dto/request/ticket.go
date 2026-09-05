// Package request 定义工单管理相关请求 DTO。
package request

import "time"

// CreateTicketRequest 创建工单请求。
type CreateTicketRequest struct {
	Title        string           `json:"title" binding:"required"`
	Description  string           `json:"description" binding:"required"`
	Tags         []string         `json:"tags"`
	ContactPhone string           `json:"contact_phone" binding:"required"`
	ContactEmail string           `json:"contact_email"`
	DeadlineAt   *time.Time       `json:"deadline_at"` // 处理时限，可空，创建时设置
	ChatContext  *ChatContextData `json:"chat_context"` // 从问答转工单时带入
}

// ChatContextData 工单关联的问答上下文（结构化，替代 JSON 字符串）。
type ChatContextData struct {
	SessionID  int64   `json:"session_id"`
	Question   string  `json:"question"`
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
}

// SystemTicketRequest 系统自动创建的复核工单（无联系电话，Source=KnowledgeReview）。
// 由知识库服务在上传文件 / LLM 元数据补全时调用，标记需人工复核。
type SystemTicketRequest struct {
	Title            string   `json:"title" binding:"required"`
	Description      string   `json:"description" binding:"required"`
	Tags             []string `json:"tags"`
	RelatedArticleID *int64   `json:"related_article_id,omitempty"` // 关联文章，可空
	RelatedKBID      *int64   `json:"related_kb_id,omitempty"`       // 关联知识库，可空
	Reason           string   `json:"reason"`                        // uploaded_document / metadata_auto_completed
}

// UpdateTicketRequest 编辑工单请求，仅更新非空字段。
type UpdateTicketRequest struct {
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Tags         []string   `json:"tags"`
	ContactPhone string     `json:"contact_phone"`
	ContactEmail string     `json:"contact_email"`
	DeadlineAt   *time.Time `json:"deadline_at"` // 处理时限，nil 不更新，非 nil 设置
}

// SupplementTicketRequest 补充工单信息请求。
type SupplementTicketRequest struct {
	Content string `json:"content" binding:"required"`
}

// UpdateTicketStatusRequest 更新工单状态请求。
type UpdateTicketStatusRequest struct {
	Action               string `json:"action" binding:"required,oneof=start request_info resolve close"`
	Result               string `json:"result"`
	ToKnowledgeCandidate bool   `json:"to_knowledge_candidate"`
}

// BatchDeleteRequest 批量操作请求（通用，供工单批量删除/批量关闭复用）。
type BatchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// CreateTicketRecordRequest 创建处理记录请求（不影响状态）。
type CreateTicketRecordRequest struct {
	Action  string `json:"action" binding:"required"`
	Content string `json:"content"`
	Detail  string `json:"detail"` // JSON 字符串
}
