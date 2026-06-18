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
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
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
// 同时将 errCode 写入 Gin context 供 Logger 中间件使用，
// 并附加 request_id 便于前端报错与后端日志关联。
func Error(c *gin.Context, code int, message string) {
	c.Set("errCode", code)
	c.JSON(mapHTTPStatus(code), Response{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: c.GetString("requestID"),
	})
}

// SuccessWithPage 返回分页成功响应
func SuccessWithPage(c *gin.Context, data interface{}, total int64, page, pageSize int) {
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
func mapHTTPStatus(code int) int {
	switch code {
	case errcode.ErrAuth:
		return http.StatusUnauthorized
	case errcode.ErrForbidden:
		return http.StatusForbidden
	case errcode.ErrParam, errcode.ErrAlreadyFrozen, errcode.ErrAlreadyActive:
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
