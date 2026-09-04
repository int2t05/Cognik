// Package handler 提供 Handler 共享的工具函数。
package handler

import (
	"strconv"

	"cognos/internal/shared/pkg/errcode"
	"cognos/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// parsePagination 解析分页参数，默认 page=1 pageSize=10，上限 100。
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

// parseID 解析路径参数 ID，失败时自动返回错误响应，调用方应直接 return。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 获取 context 中的用户 ID，未认证返回 exists=false。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// mustCurrentUserID 获取用户 ID，未认证时返回 401。
func mustCurrentUserID(c *gin.Context) (int64, bool) {
	id, ok := getCurrentUserID(c)
	if !ok {
		response.Error(c, errcode.ErrAuth, "未登录或令牌已过期")
	}
	return id, ok
}
