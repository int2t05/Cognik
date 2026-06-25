// Package request 定义智能问答相关请求 DTO。
package request

// CreateSessionRequest 创建问答会话请求。
//
// 仅创建会话容器，不触发 LLM 调用。消息通过 SendMessageRequest 在流式端点发送。
type CreateSessionRequest struct {
	KBID  int64  `json:"kb_id" binding:"required"` // 目标知识库 ID
	Title string `json:"title"`                    // 会话标题（可选，默认"新会话"）
}

// SendMessageRequest 在已有会话中发送消息请求。
//
// 用于 POST /api/v1/portal/chat-sessions/:id/stream（SSE 流式）。
type SendMessageRequest struct {
	Question   string `json:"question" binding:"required,max=2000"` // 用户问题（限制 2000 字符防滥用）
	RouteCount int    `json:"route_count"`                          // 多路检索子查询数（0=使用默认值 3）
	RerankCount int   `json:"rerank_count"`                         // 重排序截断数（0=使用默认值 5）
}

// SubmitFeedbackRequest 问答反馈请求。
type SubmitFeedbackRequest struct {
	Feedback int16 `json:"feedback" binding:"required"` // 反馈值（如 1=已解决, 2=未解决）
}

// ComputeThresholdsRequest 计算置信度阈值请求。
type ComputeThresholdsRequest struct {
	Days int `json:"days"` // 采样天数，默认 7，范围 [1, 90]
}
