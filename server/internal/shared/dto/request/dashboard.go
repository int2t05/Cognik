// Package request 定义数据看板相关请求 DTO。
package request

// TrendRequest 趋势数据查询请求。
//
// Granularity 支持 "day" 或 "week"，MVP 阶段仅实现 "day" 聚合。
// 无效粒度值降级为 "day"。
type TrendRequest struct {
	StartDate   string `json:"start_date" form:"start_date" binding:"required"` // 开始日期 YYYY-MM-DD
	EndDate     string `json:"end_date" form:"end_date" binding:"required"`     // 结束日期 YYYY-MM-DD
	Granularity string `json:"granularity" form:"granularity"`                   // 粒度：day / week
}
