// Package request 定义数据看板相关请求 DTO。
package request

// TrendRequest 趋势数据查询请求。
type TrendRequest struct {
	StartDate   string `json:"start_date" form:"start_date" binding:"required"` // 开始日期 YYYY-MM-DD
	EndDate     string `json:"end_date" form:"end_date" binding:"required"`     // 结束日期 YYYY-MM-DD
	Granularity string `json:"granularity" form:"granularity"`                  // 粒度：day / week
}
