// Package response 提供统一的 JSON 响应格式封装。
//
// 所有 API 响应使用统一格式：{"code": 0, "message": "success", "data": {...}}
// 错误响应根据错误码自动映射 HTTP 状态码，映射规则见 mapHTTPStatus 函数。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"opsmind/pkg/errcode"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// PageResponse 分页响应结构
type PageResponse struct {
	Code     int         `json:"code"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// Success 返回成功响应，HTTP 状态码 200
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    errcode.Success,
		Message: "success",
		Data:    data,
	})
}

// Error 返回错误响应，根据错误码自动映射 HTTP 状态码。
//
// 同时将 errCode 写入 Gin context，供 Logger 中间件写入日志行。
func Error(c *gin.Context, code int, message string) {
	c.Set("errCode", code)
	c.JSON(mapHTTPStatus(code), Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// SuccessWithPage 返回分页成功响应
func SuccessWithPage(c *gin.Context, data interface{}, total int64, page, pageSize int) {
	// TODO(response): 分页响应当前把 total/page/page_size 放在顶层，而部分前端类型期望 data.items/data.total。
	// 应统一一种分页契约，减少视图里 (res as any).data || res 的兼容代码。
	c.JSON(http.StatusOK, PageResponse{
		Code:     errcode.Success,
		Message:  "success",
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// mapHTTPStatus 将业务错误码映射为 HTTP 状态码。
//
// AI/RAG/Storage 服务不可用时返回 503，客户端可据此实现重试策略。
func mapHTTPStatus(code int) int {
	switch code {
	case errcode.ErrAuth:
		return http.StatusUnauthorized
	case errcode.ErrForbidden:
		return http.StatusForbidden
	case errcode.ErrParam:
		return http.StatusBadRequest
	case errcode.ErrNotFound:
		return http.StatusNotFound
	case errcode.ErrConflict:
		return http.StatusConflict
	case errcode.ErrAIUnavailable, errcode.ErrRAGUnavailable, errcode.ErrStorageUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
