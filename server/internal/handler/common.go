// Package handler 实现 HTTP 请求处理。
//
// common.go 提供所有 Handler 共享的工具函数。
// 这些函数原本分散在各个 handler 文件中（分页参数解析、ID 解析等），
// 集中到这里以减少重复、统一行为。
package handler

import (
	"strconv"

	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// parsePagination 从查询参数中解析分页参数（page, pageSize）。
//
// 默认值：page=1, pageSize=10。上限：pageSize≤100。
// 为什么集中而非各 handler 自行解析：
// 6 个 handler 原本各自实现相同的 5 行逻辑，集中后分页策略（默认值、上限）
// 只需在一处修改。
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}

// parseID 从路径参数中解析 int64 ID，解析失败时自动返回错误响应。
//
// 返回值 ok=false 表示解析失败，调用方应直接 return。
// 为什么在 parseID 内部处理响应而非让调用方处理：
// 每个调用方都写一样的错误响应是重复路径，此处统一处理保证错误信息一致。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 从 Gin context 中获取当前用户 ID。
//
// JWTAuth 中间件将当前用户 ID 以 int64 类型写入 context，key 为 "userID"。
// 返回 (userID, exists)，exists=false 表示未认证。
// 调用方如需强制认证，使用 mustCurrentUserID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// mustCurrentUserID 获取当前用户 ID，未认证时直接返回 401 错误。
//
// 用于 admin 端写操作——必须已登录才能执行。
// portal 端读操作（如 ListByUser）继续使用 getCurrentUserID 并通过 Service 层校验归属。
func mustCurrentUserID(c *gin.Context) (int64, bool) {
	id, ok := getCurrentUserID(c)
	if !ok {
		response.Error(c, errcode.ErrAuth, "未登录或令牌已过期")
	}
	return id, ok
}
