// Package handler 实现 HTTP 请求处理。
//
// llm_config.go 提供 LLM 配置管理 API，统一管理 LLM 和 Embedding 提供商配置。
package handler

import (
	"context"
	"strconv"

	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// LLMConfigHandler
// =============================================================================

// LLMConfigHandler LLM 配置管理接口。
type LLMConfigHandler struct {
	svc llmConfigService
}

// llmConfigService 定义 Handler 需要的 Service 方法（消费者定义接口）。
type llmConfigService interface {
	CreateConfig(ctx context.Context, name, llmBaseURL, llmAPIKey, embeddingBaseURL, embeddingAPIKey, llmModel, embeddingModel, systemPrompt string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error)
	ListConfigs(ctx context.Context) ([]service.LlmConfigResponse, error)
	GetConfig(ctx context.Context, id int64) (*model.LlmConfig, error)
	UpdateConfig(ctx context.Context, cfg *model.LlmConfig) error
	DeleteConfig(ctx context.Context, id int64) error
	TestConnection(ctx context.Context, id int64) (map[string]any, error)
	GetManager() *service.LLMConfigManager
}

// NewLLMConfigHandler 创建 LLMConfigHandler 实例。
func NewLLMConfigHandler(svc llmConfigService) *LLMConfigHandler {
	return &LLMConfigHandler{svc: svc}
}

// =============================================================================
// CRUD 端点
// =============================================================================

// ListConfigs 列出全部 LLM 配置。
//
// GET /api/v1/admin/llm-configs
func (h *LLMConfigHandler) ListConfigs(c *gin.Context) {
	configs, err := h.svc.ListConfigs(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, configs)
}

// CreateConfig 创建 LLM 配置。
//
// POST /api/v1/admin/llm-configs
func (h *LLMConfigHandler) CreateConfig(c *gin.Context) {
	var req request.CreateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	cfg, err := h.svc.CreateConfig(c.Request.Context(), req.Name, req.LLMBaseURL, req.LLMAPIKey, req.EmbeddingBaseURL, req.EmbeddingAPIKey,
		req.LLMModel, req.EmbeddingModel, req.SystemPrompt, req.MaxTokens, req.VectorDimension, req.IsDefault)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, cfg)
}

// GetConfig 获取单个 LLM 配置详情。
//
// GET /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) GetConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	cfg, err := h.svc.GetConfig(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig 更新 LLM 配置。
//
// PUT /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) UpdateConfig(c *gin.Context) {
	// api_key 为空时 Service 层自动保留原值，无需 Handler 处理
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	var req request.UpdateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
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
	response.Success(c, nil)
}

// DeleteConfig 删除 LLM 配置。
//
// DELETE /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) DeleteConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, nil)
}

// TestConnection 测试指定 LLM 配置的连接。
//
// POST /api/v1/admin/llm-configs/:id/test
// 委托给 LlmConfigService.TestConnection（Handler 不直接创建适配器或调用 LLM）。
func (h *LLMConfigHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	result, err := h.svc.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.Error(c, errcode.ErrAIUnavailable, err.Error())
		return
	}

	response.Success(c, result)
}
