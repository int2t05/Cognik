// Package llmconfig LLM 配置 HTTP 请求处理。
package llmconfig

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"cognik/internal/shared/dto/request"
	"cognik/internal/shared/model"
	"cognik/internal/shared/pkg/errcode"
	resp "cognik/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// llmConfigService Handler 需要的 Service 方法（消费者定义）。
type llmConfigService interface {
	CreateConfig(ctx context.Context, name, llmBaseURL, llmAPIKey, embeddingBaseURL, embeddingAPIKey, llmModel, embeddingModel, systemPrompt string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error)
	ListConfigs(ctx context.Context) ([]LlmConfigResponse, error)
	GetConfig(ctx context.Context, id int64) (*model.LlmConfig, error)
	UpdateConfig(ctx context.Context, cfg *model.LlmConfig) error
	DeleteConfig(ctx context.Context, id int64) error
	TestConnection(ctx context.Context, id int64) (map[string]any, error)
	GetManager() *LLMConfigManager
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

// LLMConfigHandler LLM 配置管理接口。
type LLMConfigHandler struct {
	svc llmConfigService
}

// NewLLMConfigHandler 创建 LLMConfigHandler 实例。
func NewLLMConfigHandler(svc llmConfigService) *LLMConfigHandler {
	return &LLMConfigHandler{svc: svc}
}

// ListConfigs 列出全部 LLM 配置。
//
// GET /api/v1/admin/llm-configs
func (h *LLMConfigHandler) ListConfigs(c *gin.Context) {
	configs, err := h.svc.ListConfigs(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, configs)
}

// CreateConfig 创建 LLM 配置。
//
// POST /api/v1/admin/llm-configs
func (h *LLMConfigHandler) CreateConfig(c *gin.Context) {
	var req request.CreateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	cfg, err := h.svc.CreateConfig(c.Request.Context(), req.Name, req.LLMBaseURL, req.LLMAPIKey, req.EmbeddingBaseURL, req.EmbeddingAPIKey,
		req.LLMModel, req.EmbeddingModel, req.SystemPrompt, req.MaxTokens, req.VectorDimension, req.IsDefault)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, cfg)
}

// GetConfig 获取单个 LLM 配置详情。
//
// GET /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) GetConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	cfg, err := h.svc.GetConfig(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, cfg)
}

// UpdateConfig 更新 LLM 配置。
//
// PUT /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) UpdateConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	var req request.UpdateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	cfg := &model.LlmConfig{
		ID:               id,
		Name:             req.Name,
		LLMBaseURL:       req.LLMBaseURL,
		LLMAPIKey:        req.LLMAPIKey,
		EmbeddingBaseURL: req.EmbeddingBaseURL,
		EmbeddingAPIKey:  req.EmbeddingAPIKey,
		LLMModel:         req.LLMModel,
		EmbeddingModel:   req.EmbeddingModel,
		SystemPrompt:     req.SystemPrompt,
		MaxTokens:        req.MaxTokens,
		VectorDimension:  req.VectorDimension,
		IsDefault:        req.IsDefault,
	}

	if err := h.svc.UpdateConfig(c.Request.Context(), cfg); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// DeleteConfig 删除 LLM 配置。
//
// DELETE /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) DeleteConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// TestConnection 测试指定 LLM 配置的连接。
//
// POST /api/v1/admin/llm-configs/:id/test
func (h *LLMConfigHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	result, err := h.svc.TestConnection(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, errcode.ErrAIUnavailable, err.Error())
		return
	}

	resp.Success(c, result)
}
