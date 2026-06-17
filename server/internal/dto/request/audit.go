// Package request 定义审计日志相关请求 DTO。
package request

// AuditLogListRequest 审计日志列表查询请求。
type AuditLogListRequest struct {
	OperatorID int64  `form:"operator_id"`  // 操作人 ID（0=全部）
	Action     string `form:"action"`       // 操作类型（空=全部）
	TargetType string `form:"target_type"`  // 目标类型（空=全部，如 user/role/knowledge/ticket）
	TargetID   int64  `form:"target_id"`    // 目标 ID（0=全部）
	DateFrom   string `form:"date_from"`    // 起始日期（YYYY-MM-DD，可选）
	DateTo     string `form:"date_to"`      // 结束日期（YYYY-MM-DD，可选）
}
