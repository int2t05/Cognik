// Package handler 实现 HTTP 请求处理。
//
// config.go 提供系统配置管理接口。
package handler

import (
	"encoding/json"

	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// ConfigHandler 系统配置管理接口。
type ConfigHandler struct {
	svc *service.ConfigService
}

// NewConfigHandler 创建 ConfigHandler 实例。
func NewConfigHandler(svc *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// Get 获取指定 key 的配置值。
//
// GET /api/v1/admin/configs/:key
func (h *ConfigHandler) Get(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	val, err := h.svc.GetConfig(c.Request.Context(), key)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, val)
}

// Update 更新或创建系统配置。
//
// PUT /api/v1/admin/configs/:key
//
// 使用 json.RawMessage 检查 "value" 键是否存在，
// 避免 binding:"required" 将 false/0/"" 等合法值误判为缺失。
func (h *ConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	// 先读取原始 JSON 检查 "value" 键是否存在
	raw, err := c.GetRawData()
	if err != nil {
		response.Error(c, errcode.ErrParam, "读取请求体失败")
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		response.Error(c, errcode.ErrParam, "请求体不是合法 JSON")
		return
	}
	valRaw, ok := m["value"]
	if !ok {
		response.Error(c, errcode.ErrParam, "缺少 value 字段")
		return
	}

	// 反序列化 value 为任意类型
	var val interface{}
	if err := json.Unmarshal(valRaw, &val); err != nil {
		response.Error(c, errcode.ErrParam, "value 字段解析失败")
		return
	}

	updatedBy, _ := getCurrentUserID(c)
	if err := h.svc.UpdateConfig(c.Request.Context(), key, val, updatedBy); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
