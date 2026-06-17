// Package service 实现 LLM 配置管理业务逻辑。
//
// LLMConfigManager 使用 atomic.Value 实现零锁配置热替换。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/gorm"
)

// LLMConfigManager 管理当前生效的 LLM 配置（热替换）。
type LLMConfigManager struct {
	current atomic.Value // *model.LlmConfig
}

func NewLLMConfigManager() *LLMConfigManager {
	return &LLMConfigManager{}
}

// GetConfig 返回当前生效的配置（零锁读取），可能为 nil。
func (m *LLMConfigManager) GetConfig() *model.LlmConfig {
	v := m.current.Load()
	if v == nil {
		return nil
	}
	return v.(*model.LlmConfig)
}

// store 原子替换配置（深拷贝，防止调用方修改原对象影响热配置）。
func (m *LLMConfigManager) store(cfg *model.LlmConfig) {
	clone := *cfg
	m.current.Store(&clone)
}

// llmConfigRepo 定义 LLM 配置仓库接口（消费者定义接口）。
type llmConfigRepo interface {
	Create(cfg *model.LlmConfig) error
	FindByID(id int64) (*model.LlmConfig, error)
	FindDefault() (*model.LlmConfig, error)
	List() ([]model.LlmConfig, error)
	Update(cfg *model.LlmConfig) error
	Delete(id int64) error
	ClearDefault() error
}

type txRepoFactory func(tx *gorm.DB) llmConfigRepo

// LLMConfigService LLM 配置管理服务。
type LLMConfigService struct {
	repo       llmConfigRepo
	newRepo    txRepoFactory
	manager    *LLMConfigManager
	db         *gorm.DB
}

// NewLLMConfigService 创建 LLMConfigService 实例。
// repo 可以是 *repository.LlmConfigRepo 或测试 mock。
// 返回 error 而非 panic，便于 main 统一处理装配错误。
func NewLLMConfigService(repo interface{}) (*LLMConfigService, error) {
	svc := &LLMConfigService{
		manager: NewLLMConfigManager(),
	}

	switch r := repo.(type) {
	case *repository.LlmConfigRepo:
		svc.repo = r
		svc.db = r.DB()
		svc.newRepo = func(tx *gorm.DB) llmConfigRepo {
			return repository.NewLlmConfigRepo(tx)
		}
	case llmConfigRepo:
		svc.repo = r
	default:
		return nil, fmt.Errorf("NewLLMConfigService: unsupported repo type %T", repo)
	}

	if cfg, err := svc.repo.FindDefault(); err == nil {
		svc.manager.store(cfg)
	}

	return svc, nil
}

func (s *LLMConfigService) GetManager() *LLMConfigManager { return s.manager }

// CreateConfig 创建 LLM 配置。is_default=true 时先清空其他默认（事务保证原子性）。
func (s *LLMConfigService) CreateConfig(name string, providerType int16, baseURL, embeddingBaseURL, apiKey, llmModel, embeddingModel string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error) {
	if strings.TrimSpace(name) == "" {
		return nil, AppError{Code: errcode.ErrParam, Message: "名称不能为空"}
	}
	if providerType != 1 && providerType != 2 {
		return nil, AppError{Code: errcode.ErrParam, Message: "提供商类型无效（1=llama.cpp, 2=OpenAI-compatible）"}
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, AppError{Code: errcode.ErrParam, Message: "BaseURL 不能为空"}
	}
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	if vectorDimension <= 0 {
		vectorDimension = 1024
	}

	cfg := &model.LlmConfig{
		Name: name, ProviderType: providerType, BaseURL: baseURL,
		EmbeddingBaseURL: embeddingBaseURL, APIKey: apiKey,
		LLMModel: llmModel, EmbeddingModel: embeddingModel,
		MaxTokens: maxTokens, VectorDimension: vectorDimension, IsDefault: isDefault,
	}

	if s.db != nil && isDefault {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(); err != nil {
				return AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Create(cfg)
		})
		if err != nil {
			return nil, err
		}
	} else {
		if isDefault {
			if err := s.repo.ClearDefault(); err != nil {
				return nil, AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Create(cfg); err != nil {
			return nil, AppError{Code: errcode.ErrUnknown, Message: "创建 LLM 配置失败"}
		}
	}

	if isDefault {
		s.manager.store(cfg)
	}
	return cfg, nil
}

// UpdateConfig 更新 LLM 配置。api_key 为空时保留原值。
func (s *LLMConfigService) UpdateConfig(cfg *model.LlmConfig) error {
	// api_key 为空时保留数据库中原值
	if cfg.APIKey == "" {
		existing, err := s.repo.FindByID(cfg.ID)
		if err != nil {
			return AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
		}
		cfg.APIKey = existing.APIKey
	}

	if s.db != nil && cfg.IsDefault {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(); err != nil {
				return AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Update(cfg)
		})
		if err != nil {
			return err
		}
	} else {
		if cfg.IsDefault {
			if err := s.repo.ClearDefault(); err != nil {
				return AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Update(cfg); err != nil {
			return AppError{Code: errcode.ErrUnknown, Message: "更新 LLM 配置失败"}
		}
	}

	if cfg.IsDefault {
		s.manager.store(cfg)
	}
	return nil
}

func (s *LLMConfigService) ListConfigs() ([]LlmConfigResponse, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	result := make([]LlmConfigResponse, len(configs))
	for i, c := range configs {
		result[i] = LlmConfigResponse{
			ID: c.ID, Name: c.Name, ProviderType: c.ProviderType,
			BaseURL: c.BaseURL, EmbeddingBaseURL: c.EmbeddingBaseURL,
			APIKey: maskAPIKey(c.APIKey), LLMModel: c.LLMModel,
			EmbeddingModel: c.EmbeddingModel, MaxTokens: c.MaxTokens,
			VectorDimension: c.VectorDimension, IsDefault: c.IsDefault,
		}
	}
	return result, nil
}

func (s *LLMConfigService) GetConfig(id int64) (*model.LlmConfig, error) {
	cfg, err := s.repo.FindByID(id)
	if err != nil {
		return nil, AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	return cfg, nil
}

func (s *LLMConfigService) DeleteConfig(id int64) error {
	cfg, err := s.repo.FindByID(id)
	if err != nil {
		return AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	if cfg.IsDefault {
		return AppError{Code: errcode.ErrParam, Message: "不能删除默认配置，请先设置其他配置为默认"}
	}
	// 检查知识库引用
	if r, ok := s.repo.(*repository.LlmConfigRepo); ok {
		count, err := r.CountReferencingKBs(id)
		if err != nil {
			return err
		}
		if count > 0 {
			return AppError{Code: errcode.ErrConflict, Message: "该配置被知识库引用，无法删除"}
		}
	}
	return s.repo.Delete(id)
}

// =============================================================================
// LlmConfigResponse — 列表响应（API Key 脱敏）
// =============================================================================

type LlmConfigResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	ProviderType     int16  `json:"provider_type"`
	BaseURL          string `json:"base_url"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	APIKey           string `json:"api_key"`
	LLMModel         string `json:"llm_model"`
	EmbeddingModel   string `json:"embedding_model"`
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
		APIKey string `json:"api_key"`
	}{
		Alias:  (*Alias)(&r),
		APIKey: maskAPIKey(r.APIKey),
	})
}

func NewLlmConfigResponse(cfg *model.LlmConfig) LlmConfigResponse {
	return LlmConfigResponse{
		ID: cfg.ID, Name: cfg.Name, ProviderType: cfg.ProviderType,
		BaseURL: cfg.BaseURL, EmbeddingBaseURL: cfg.EmbeddingBaseURL,
		APIKey: cfg.APIKey, LLMModel: cfg.LLMModel,
		EmbeddingModel: cfg.EmbeddingModel, MaxTokens: cfg.MaxTokens,
		VectorDimension: cfg.VectorDimension, IsDefault: cfg.IsDefault,
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
