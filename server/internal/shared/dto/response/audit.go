// Package response 定义审计日志相关响应 DTO。
package response

// AuditLogItem 单条审计日志（含操作人姓名）。
//
// 为什么包含 OperatorName 而非仅 OperatorID：
// 前端直接展示姓名无需二次查询用户表，减少请求数。
type AuditLogItem struct {
	ID           int64  `json:"id"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"` // 操作人姓名（JOIN users 表）
	Action       string `json:"action"`
	TargetType   string `json:"target_type"`
	TargetID     int64  `json:"target_id"`
	Detail       string `json:"detail"`       // JSON 字符串
	IPAddress    string `json:"ip_address"`
	CreatedAt    string `json:"created_at"`    // 格式：2006-01-02 15:04:05
}

// AuditLogListResponse 审计日志列表响应（分页）。
type AuditLogListResponse struct {
	Items    []AuditLogItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}
