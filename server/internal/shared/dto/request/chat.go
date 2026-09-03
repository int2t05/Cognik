// Package request 定义智能问答相关请求 DTO。
package request

// CreateSessionRequest 创建问答会话请求。
type CreateSessionRequest struct {
	KBID  int64  `json:"kb_id" binding:"required"` // 目标知识库 ID
	Title string `json:"title"`                    // 会话标题（可选，默认"新会话"）
}

// SendMessageRequest 发送消息请求（SSE 流式端点）。
type SendMessageRequest struct {
	Question string `json:"question" binding:"required,max=2000"` // 用户问题（限制 2000 字符防滥用）
}

// SubmitFeedbackRequest 问答反馈请求。
type SubmitFeedbackRequest struct {
	Feedback int16 `json:"feedback" binding:"required"` // 反馈值（如 1=已解决, 2=未解决）
}

// ComputeThresholdsRequest 计算置信度阈值请求。
type ComputeThresholdsRequest struct {
	Days int `json:"days"` // 采样天数，默认 7，范围 [1, 90]
}
