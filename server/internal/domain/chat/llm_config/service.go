// Package llmconfig LLM 配置业务逻辑（配置 CRUD、热替换、连接测试）。
package llmconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"cognos/internal/agent/llm"
	"cognos/internal/domain/system/audit"
	"cognos/internal/shared/model"
	"cognos/internal/shared/pkg/errcode"

	"gorm.io/gorm"
)

// llmConfigRepo LLM 配置仓库接口（消费者定义）。
type llmConfigRepo interface {
	Create(ctx context.Context, cfg *model.LlmConfig) error
	FindByID(ctx context.Context, id int64) (*model.LlmConfig, error)
	FindDefault(ctx context.Context) (*model.LlmConfig, error)
	List(ctx context.Context) ([]model.LlmConfig, error)
	Update(ctx context.Context, cfg *model.LlmConfig) error
	Delete(ctx context.Context, id int64) error
	ClearDefault(ctx context.Context) error
	CountReferencingKBs(ctx context.Context, configID int64) (int64, error)
}

type txRepoFactory func(tx *gorm.DB) llmConfigRepo

// =============================================================================
// LLMConfigManager — 配置热替换
// =============================================================================

// LLMConfigManager 管理当前生效的 LLM 配置（atomic.Value 零锁热替换）。
type LLMConfigManager struct {
	current  atomic.Value // *model.LlmConfig
	onChange func()
}

func NewLLMConfigManager() *LLMConfigManager {
	return &LLMConfigManager{}
}

// OnChange 注册配置变更回调（覆盖式：后注册覆盖前注册，调用方须在同一回调内完成所有重建）。
func (m *LLMConfigManager) OnChange(fn func()) {
	m.onChange = fn
}

// GetConfig 返回当前生效配置（零锁，可能为 nil）。
func (m *LLMConfigManager) GetConfig() *model.LlmConfig {
	v := m.current.Load()
	if v == nil {
		return nil
	}
	return v.(*model.LlmConfig)
}

// store 原子替换配置并触发回调。
func (m *LLMConfigManager) store(cfg *model.LlmConfig) {
	clone := *cfg
	m.current.Store(&clone)
	if m.onChange != nil {
		m.onChange()
	}
}

// =============================================================================
// LLMConfigService
// =============================================================================

// LLMConfigService LLM 配置管理服务。
type LLMConfigService struct {
	repo        llmConfigRepo
	newRepo     txRepoFactory
	manager     *LLMConfigManager
	auditWriter audit.AuditWriter
	db          *gorm.DB
}

// NewLLMConfigService 创建 LLMConfigService 实例。
func NewLLMConfigService(repo llmConfigRepo, db *gorm.DB, auditWriter audit.AuditWriter) (*LLMConfigService, error) {
	svc := &LLMConfigService{
		repo:        repo,
		manager:     NewLLMConfigManager(),
		db:          db,
		auditWriter: auditWriter,
	}
	if db != nil {
		svc.newRepo = func(tx *gorm.DB) llmConfigRepo { return NewLlmConfigRepo(tx) }
	}

	if cfg, err := svc.repo.FindDefault(context.Background()); err == nil && cfg != nil {
		svc.manager.store(cfg)
	}

	return svc, nil
}

// GetManager 返回 LLMConfigManager 实例。
func (s *LLMConfigService) GetManager() *LLMConfigManager { return s.manager }

// CreateConfig 创建 LLM 配置。is_default=true 时先清空其他默认。
func (s *LLMConfigService) CreateConfig(ctx context.Context, name, llmBaseURL, llmAPIKey, embeddingBaseURL, embeddingAPIKey, llmModel, embeddingModel, systemPrompt string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "名称不能为空"}
	}
	if strings.TrimSpace(llmBaseURL) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "LLM BaseURL 不能为空"}
	}
	if strings.TrimSpace(llmModel) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "LLM 模型不能为空"}
	}
	if strings.TrimSpace(embeddingModel) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "Embedding 模型不能为空"}
	}
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	if vectorDimension <= 0 {
		vectorDimension = 1536
	}

	cfg := &model.LlmConfig{
		Name: name, LLMBaseURL: llmBaseURL, LLMAPIKey: llmAPIKey,
		EmbeddingBaseURL: embeddingBaseURL, EmbeddingAPIKey: embeddingAPIKey,
		LLMModel: llmModel, EmbeddingModel: embeddingModel, SystemPrompt: systemPrompt,
		MaxTokens: maxTokens, VectorDimension: vectorDimension, IsDefault: isDefault,
	}

	if s.db != nil && isDefault {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Create(ctx, cfg)
		})
		if err != nil {
			return nil, err
		}
	} else {
		if isDefault {
			if err := s.repo.ClearDefault(ctx); err != nil {
				return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Create(ctx, cfg); err != nil {
			return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建 LLM 配置失败"}
		}
	}

	fresh, err := s.repo.FindByID(ctx, cfg.ID)
	if err != nil {
		return nil, err
	}
	cfg = fresh
	if isDefault {
		s.manager.store(cfg)
	}
	return cfg, nil
}

// UpdateConfig 更新 LLM 配置。API Key 为空时保留数据库原值。
func (s *LLMConfigService) UpdateConfig(ctx context.Context, cfg *model.LlmConfig) error {
	existing, err := s.repo.FindByID(ctx, cfg.ID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	if cfg.LLMAPIKey == "" {
		cfg.LLMAPIKey = existing.LLMAPIKey
	}
	if cfg.EmbeddingAPIKey == "" {
		cfg.EmbeddingAPIKey = existing.EmbeddingAPIKey
	}

	if s.db != nil && cfg.IsDefault {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Update(ctx, cfg)
		})
		if err != nil {
			return err
		}
	} else {
		if cfg.IsDefault {
			if err := s.repo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Update(ctx, cfg); err != nil {
			return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新 LLM 配置失败"}
		}
	}

	if cfg.IsDefault {
		fresh, err := s.repo.FindByID(ctx, cfg.ID)
		if err != nil {
			return err
		}
		cfg = fresh
		s.manager.store(cfg)
	}
	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, 0, "llm_config.update", "llm_config", cfg.ID, "")
	}
	return nil
}

// ListConfigs 列出全部 LLM 配置（API Key 脱敏）。
func (s *LLMConfigService) ListConfigs(ctx context.Context) ([]LlmConfigResponse, error) {
	configs, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]LlmConfigResponse, len(configs))
	for i, c := range configs {
		result[i] = LlmConfigResponse{
			ID: c.ID, Name: c.Name,
			LLMBaseURL: c.LLMBaseURL, LLMAPIKey: maskAPIKey(c.LLMAPIKey),
			EmbeddingBaseURL: c.EmbeddingBaseURL, EmbeddingAPIKey: maskAPIKey(c.EmbeddingAPIKey),
			LLMModel: c.LLMModel, EmbeddingModel: c.EmbeddingModel,
			SystemPrompt: c.SystemPrompt, MaxTokens: c.MaxTokens,
			VectorDimension: c.VectorDimension, IsDefault: c.IsDefault,
		}
	}
	return result, nil
}

// GetConfig 获取单个 LLM 配置。
func (s *LLMConfigService) GetConfig(ctx context.Context, id int64) (*model.LlmConfig, error) {
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	return cfg, nil
}

// DeleteConfig 删除 LLM 配置。
func (s *LLMConfigService) DeleteConfig(ctx context.Context, id int64) error {
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	if cfg.IsDefault {
		return errcode.AppError{Code: errcode.ErrParam, Message: "不能删除默认配置，请先设置其他配置为默认"}
	}
	count, err := s.repo.CountReferencingKBs(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errcode.AppError{Code: errcode.ErrConflict, Message: "该配置被知识库引用，无法删除"}
	}
	return s.repo.Delete(ctx, id)
}

// TestConnection 测试指定 LLM 配置的连接是否可用。
func (s *LLMConfigService) TestConnection(ctx context.Context, id int64) (map[string]any, error) {
	cfg, err := s.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 构造独立 ChatModel 测试连接（不复用生产 ChatModel，避免热切换副作用）
	chatModel := llm.NewChatModel(llm.ChatModelConfig{
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
		BaseURL: cfg.LLMBaseURL,
	})

	start := time.Now()
	resp, err := chatModel.Generate(ctx, []*llm.Message{
		llm.UserMessage("ping"),
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("连接测试失败: %w", err)
	}

	return map[string]any{
		"success":      true,
		"model":        cfg.LLMModel,
		"latency_ms":   latency,
		"test_message": resp.Content,
	}, nil
}

// =============================================================================
// LlmConfigResponse — 列表响应（API Key 脱敏）
// =============================================================================

type LlmConfigResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	LLMBaseURL       string `json:"llm_base_url"`
	LLMAPIKey        string `json:"llm_api_key"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingAPIKey  string `json:"embedding_api_key"`
	LLMModel         string `json:"llm_model"`
	EmbeddingModel   string `json:"embedding_model"`
	SystemPrompt     string `json:"system_prompt"`
	MaxTokens        int    `json:"max_tokens"`
	VectorDimension  int    `json:"vector_dimension"`
	IsDefault        bool   `json:"is_default"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func (r LlmConfigResponse) MarshalJSON() ([]byte, error) {
	type Alias LlmConfigResponse
	return json.Marshal(&struct {
		*Alias
		LLMAPIKey       string `json:"llm_api_key"`
		EmbeddingAPIKey string `json:"embedding_api_key"`
	}{
		Alias:           (*Alias)(&r),
		LLMAPIKey:       maskAPIKey(r.LLMAPIKey),
		EmbeddingAPIKey: maskAPIKey(r.EmbeddingAPIKey),
	})
}

func NewLlmConfigResponse(cfg *model.LlmConfig) LlmConfigResponse {
	return LlmConfigResponse{
		ID: cfg.ID, Name: cfg.Name,
		LLMBaseURL: cfg.LLMBaseURL, LLMAPIKey: cfg.LLMAPIKey,
		EmbeddingBaseURL: cfg.EmbeddingBaseURL, EmbeddingAPIKey: cfg.EmbeddingAPIKey,
		LLMModel: cfg.LLMModel, EmbeddingModel: cfg.EmbeddingModel,
		MaxTokens: cfg.MaxTokens, VectorDimension: cfg.VectorDimension,
		IsDefault: cfg.IsDefault,
		CreatedAt: cfg.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
