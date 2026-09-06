// Package config 系统配置 HTTP 请求处理。
package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"cognik/internal/infra/config"
	"cognik/internal/shared/dto/request"
	"cognik/internal/shared/pkg/errcode"
	resp "cognik/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// parsePagination 解析分页参数。
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

// getCurrentUserID 从 Gin context 获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 错误。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		resp.Error(c, appErr.Code, appErr.Message)
		return
	}
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	resp.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// LLMInfo 从 .env 派生的 LLM/Embedding 配置(只读,不含 API key)。
type LLMInfo struct {
	LLMBaseURL       string `json:"llm_base_url"`
	LLMModel         string `json:"llm_model"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingModel  string `json:"embedding_model"`
	EmbeddingDimension int `json:"embedding_dimension"`
}

// ConfigHandler 系统配置管理接口。
type ConfigHandler struct {
	svc     *ConfigService
	llmInfo LLMInfo
	cfg     *config.AppConfig
}

// NewConfigHandler 创建 ConfigHandler 实例。
func NewConfigHandler(svc *ConfigService, llmInfo LLMInfo, cfg *config.AppConfig) *ConfigHandler {
	return &ConfigHandler{svc: svc, llmInfo: llmInfo, cfg: cfg}
}

// GetPublic 获取公开配置值（无需认证）。
//
// GET /api/v1/public/configs/:key
func (h *ConfigHandler) GetPublic(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		resp.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}
	if !h.svc.IsPublicKey(key) {
		resp.Error(c, errcode.ErrNotFound, "配置不存在")
		return
	}

	val, err := h.svc.GetConfig(c.Request.Context(), key)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, val)
}

// Get 获取指定 key 的配置值。
//
// GET /api/v1/admin/configs/:key
func (h *ConfigHandler) Get(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		resp.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	val, err := h.svc.GetConfig(c.Request.Context(), key)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, val)
}

// Update 更新或创建系统配置。
//
// PUT /api/v1/admin/configs/:key
func (h *ConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		resp.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	raw, err := c.GetRawData()
	if err != nil {
		resp.Error(c, errcode.ErrParam, "读取请求体失败")
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		resp.Error(c, errcode.ErrParam, "请求体不是合法 JSON")
		return
	}
	valRaw, ok := m["value"]
	if !ok {
		resp.Error(c, errcode.ErrParam, "缺少 value 字段")
		return
	}

	var val interface{}
	if err := json.Unmarshal(valRaw, &val); err != nil {
		resp.Error(c, errcode.ErrParam, "value 字段解析失败")
		return
	}

	updatedBy, _ := getCurrentUserID(c)
	if err := h.svc.UpdateConfig(c.Request.Context(), key, val, updatedBy); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// ComputeThresholds 计算置信度阈值分位数。
//
// POST /api/v1/admin/confidence/compute-thresholds
func (h *ConfigHandler) ComputeThresholds(c *gin.Context) {
	var req request.ComputeThresholdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}

	result, err := h.svc.ComputeThresholds(c.Request.Context(), req.Days)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, result)
}

// GetLLMInfo 返回 .env 派生的 LLM/Embedding 配置(只读,不含 API key)。
//
// GET /api/v1/admin/configs/llm-info
func (h *ConfigHandler) GetLLMInfo(c *gin.Context) {
	resp.Success(c, h.llmInfo)
}

// GetEnvConfigs 返回 .env 派生的全部配置项(LLM/Embedding/API key 脱敏)。
//
// GET /api/v1/admin/configs/env
func (h *ConfigHandler) GetEnvConfigs(c *gin.Context) {
	resp.Success(c, config.GetEnvConfigs(h.cfg))
}

// UpdateEnvConfig 更新 .env 配置项并触发热重建。
// body: {"key": "llm_model", "value": "qwen3-4b"}
//
// PUT /api/v1/admin/configs/env
func (h *ConfigHandler) UpdateEnvConfig(c *gin.Context) {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	newCfg, err := config.UpdateEnvConfig(body.Key, body.Value)
	if err != nil {
		resp.Error(c, errcode.ErrUnknown, "更新 .env 失败: "+err.Error())
		return
	}
	// 更新 handler 的 cfg 快照(供 GetEnvConfigs/GetLLMInfo)
	h.cfg = newCfg
	h.llmInfo = LLMInfo{
		LLMBaseURL:       newCfg.LLM.BaseURL,
		LLMModel:        newCfg.LLM.Model,
		EmbeddingBaseURL: newCfg.Embedding.BaseURL,
		EmbeddingModel:  newCfg.Embedding.Model,
		EmbeddingDimension: newCfg.Embedding.Dimension,
	}
	resp.Success(c, nil)
}
