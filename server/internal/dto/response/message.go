// Package response 定义消息模块的 HTTP 响应结构。
//
// 将 handler 层中使用的匿名 gin.H / map 替换为类型化结构体，
// 提升类型安全性和文档自描述能力。
package response

// MarkAsReadResponse 标记消息已读的响应。
type MarkAsReadResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

// MarkAllReadResponse 全部标记已读的响应。
type MarkAllReadResponse struct {
	Affected int64 `json:"affected"`
}

// UnreadCountResponse 未读消息数的响应。
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
