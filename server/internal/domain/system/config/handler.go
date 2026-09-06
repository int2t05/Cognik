// Package config 系统配置 HTTP 请求处理（.env 驱动,无 DB）。
package config

import (
	"errors"
	"log/slog"

	"cognik/internal/infra/config"
	"cognik/internal/shared/pkg/errcode"
	resp "cognik/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// LLMInfo 从 .env 派生的 LLM/Embedding 配置(只读,不含 API key)。
type LLMInfo struct {
	LLMBaseURL         string `json:"llm_base_url"`
	LLMModel          string `json:"llm_model"`
	EmbeddingBaseURL   string `json:"embedding_base_url"`
	EmbeddingModel    string `json:"embedding_model"`
	EmbeddingDimension int    `json:"embedding_dimension"`
}

// ConfigHandler 系统配置管理接口（.env 驱动）。
type ConfigHandler struct {
	llmInfo LLMInfo
	cfg     *config.AppConfig
}

// NewConfigHandler 创建 ConfigHandler 实例。
func NewConfigHandler(cfg *config.AppConfig, llmInfo LLMInfo) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, llmInfo: llmInfo}
}

// GetPublic 获取公开配置值（无需认证,仅 app_name）。
//
// GET /api/v1/public/configs/:key
func (h *ConfigHandler) GetPublic(c *gin.Context) {
	key := c.Param("key")
	if key != "app_name" {
		resp.Error(c, errcode.ErrNotFound, "配置项不存在")
		return
	}
	resp.Success(c, h.cfg.AppName)
}

// GetLLMInfo 返回 .env 派生的 LLM/Embedding 配置(只读,不含 API key)。
//
// GET /api/v1/admin/configs/llm-info
func (h *ConfigHandler) GetLLMInfo(c *gin.Context) {
	resp.Success(c, h.llmInfo)
}

// GetEnvConfigs 返回 .env 派生的全部配置项(API key 脱敏)。
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
	h.cfg = newCfg
	h.llmInfo = LLMInfo{
		LLMBaseURL:         newCfg.LLM.BaseURL,
		LLMModel:          newCfg.LLM.Model,
		EmbeddingBaseURL:   newCfg.Embedding.BaseURL,
		EmbeddingModel:    newCfg.Embedding.Model,
		EmbeddingDimension: newCfg.Embedding.Dimension,
	}
	resp.Success(c, nil)
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
