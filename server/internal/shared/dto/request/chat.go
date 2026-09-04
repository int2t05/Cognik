// Package request 定义智能问答相关请求 DTO。
package request

// ComputeThresholdsRequest 计算置信度阈值请求。
type ComputeThresholdsRequest struct {
	Days int `json:"days"` // 采样天数，默认 7，范围 [1, 90]
}
