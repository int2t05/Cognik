// Package response 定义数据看板相关响应 DTO。
package response

// StatsResponse 看板统计数据。
type StatsResponse struct {
	TodayTickets      int64   `json:"today_tickets"`      // 今日新增申告数
	PendingTickets    int64   `json:"pending_tickets"`    // 待处理申告数（status=1）
	ProcessingTickets int64   `json:"processing_tickets"` // 处理中申告数（status=2）
	ResolvedTickets   int64   `json:"resolved_tickets"`   // 已解决申告数（status=4）
	TodayChats        int64   `json:"today_chats"`        // 今日问答次数
	AvgConfidence     float64 `json:"avg_confidence"`      // 今日平均置信度
	KnowledgeCount    int64   `json:"knowledge_count"`     // 知识条目总数
	HelpfulFeedback   int64   `json:"helpful_feedback"`   // 累计"有帮助"反馈数
	UnhelpfulFeedback int64   `json:"unhelpful_feedback"` // 累计"无帮助"反馈数
}

// TrendResponse 趋势数据响应。
type TrendResponse struct {
	DataPoints []DataPoint `json:"data_points"` // 趋势数据点列表
}

// DataPoint 单个数据点（按天或按周聚合）。
type DataPoint struct {
	Date        string `json:"date"`         // 日期 YYYY-MM-DD
	TicketCount int64  `json:"ticket_count"` // 该日新增申告数
	ChatCount   int64  `json:"chat_count"`   // 该日问答数
}
