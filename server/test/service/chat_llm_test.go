//go:build integration

// Package service_test 验证与真实 LLM/Embedding 服务的端到端集成。
//
// 需要运行中的服务：llama.cpp (LLM :8081, Embedding :8082)。
package service_test

import (
	"testing"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"
)

// TestLLMClient_ChatCompletion 验证 LLM 客户端可正常调用 llama.cpp。
//
// 需要运行中的 LLM 服务，不可用时跳过。
func TestLLMClient_ChatCompletion(t *testing.T) {
	client, err := adapter.NewOpenAIClient(
		"http://localhost:8081/v1",
		"",
		5*time.Second, // 短超时用于快速探活
	)
	if err != nil {
		t.Fatalf("创建 LLM 客户端失败: %v", err)
	}

	// 快速探活：获取模型列表
	if _, err := client.ChatCompletion(bgCtx, adapter.ChatRequest{
		Model: "qwen3-4b", Messages: []adapter.ChatMessage{{Role: "user", Content: "ping"}},
		MaxTokens: 1, Temperature: 0,
	}); err != nil {
		t.Skipf("LLM 服务不可用（%v），跳过集成测试", err)
		return
	}

	// 重新创建正常超时的客户端
	client, _ = adapter.NewOpenAIClient("http://localhost:8081/v1", "", 60*time.Second)

	req := adapter.ChatRequest{
		Model: "qwen3-4b",
		Messages: []adapter.ChatMessage{
			{Role: "user", Content: "Say hello in one sentence."},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := client.ChatCompletion(bgCtx, req)
	if err != nil {
		t.Fatalf("ChatCompletion 失败: %v", err)
	}

	t.Logf("LLM 回复: %s", resp.Content)
	if resp.Content == "" {
		t.Error("LLM 应返回非空回复")
	}
}

// TestEmbeddingClient_CreateEmbeddings 验证 Embedding 客户端可正常调用。
//
// 需要运行中的 Embedding 服务，不可用时跳过。
func TestEmbeddingClient_CreateEmbeddings(t *testing.T) {
	client := adapter.NewOpenAIEmbeddingClient(
		"http://localhost:8082/v1",
		"",
		"qwen3-emb",
		5*time.Second, // 短超时用于快速探活
	)

	// 快速探活
	if _, err := client.CreateEmbeddings(bgCtx, adapter.EmbeddingRequest{
		Model: "qwen3-emb", Input: []string{"ping"},
	}); err != nil {
		t.Skipf("Embedding 服务不可用（%v），跳过集成测试", err)
		return
	}

	// 重新创建正常超时的客户端
	client = adapter.NewOpenAIEmbeddingClient("http://localhost:8082/v1", "", "qwen3-emb", 60*time.Second)

	req := adapter.EmbeddingRequest{
		Model: "qwen3-emb",
		Input: []string{"Hello world"},
	}

	resp, err := client.CreateEmbeddings(bgCtx, req)
	if err != nil {
		t.Fatalf("CreateEmbeddings 失败: %v", err)
	}

	t.Logf("向量维度: %d, 输入 tokens: %d", resp.Dimension, resp.TokensUsed)
	if resp.Dimension == 0 {
		t.Error("向量维度不应为 0")
	}
}

// TestChatService_LLMConfigIntegration 验证 LLM 配置管理集成。
func TestChatService_LLMConfigIntegration(t *testing.T) {
	// 清理
	knowledgeSvcDB.Exec("DELETE FROM chat_messages")
	knowledgeSvcDB.Exec("DELETE FROM chat_sessions")
	knowledgeSvcDB.Exec("DELETE FROM llm_configs")

	// 创建 LLM 配置
	llmConfigRepo := repository.NewLlmConfigRepo(knowledgeSvcDB)
	llmConfigSvc, err := service.NewLLMConfigService(llmConfigRepo, knowledgeSvcDB, nil)
	if err != nil {
		t.Fatalf("NewLLMConfigService 失败: %v", err)
	}

	cfg, err := llmConfigSvc.CreateConfig(bgCtx,
		"llama.cpp 本地", 1,
		"http://localhost:8081/v1",  // LLM
		"http://localhost:8082/v1",  // Embedding
		"",                           // APIKey
		"qwen3-4b",                   // LLM Model
		"qwen3-emb",                  // Embedding Model
		"",                           // SystemPrompt
		512, 1024, true,              // MaxTokens, VectorDim, IsDefault
	)
	if err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	t.Logf("LLM 配置创建成功: id=%d, model=%s, emb_model=%s", cfg.ID, cfg.LLMModel, cfg.EmbeddingModel)

	// 验证热加载管理器
	mgr := llmConfigSvc.GetManager()
	activeCfg := mgr.GetConfig()
	if activeCfg == nil {
		t.Fatal("应有活跃的 LLM 配置")
	}
	if activeCfg.LLMModel != "qwen3-4b" {
		t.Errorf("模型不匹配: %s", activeCfg.LLMModel)
	}
	if activeCfg.BaseURL != "http://localhost:8081/v1" {
		t.Errorf("BaseURL 不匹配: %s", activeCfg.BaseURL)
	}

	// 验证 API Key 加密存储（llama.cpp 无 API Key，应加密为空或原始值）
	configs, _ := llmConfigSvc.ListConfigs(bgCtx)
	if len(configs) == 0 {
		t.Fatal("应有至少 1 条配置")
	}
	t.Logf("配置列表: %d 条", len(configs))
	t.Logf("API Key 脱敏: %s", configs[0].APIKey)

	// 验证唯一默认约束
	_, err = llmConfigSvc.CreateConfig(bgCtx,
		"OpenAI 远程", 2,
		"https://api.openai.com/v1", "",
		"sk-test-key",
		"gpt-4o", "text-embedding-3-small",
		"", 4096, 1536, true,
	)
	if err != nil {
		t.Fatalf("创建第二个默认配置失败: %v", err)
	}

	// 验证：第一个默认已自动取消
	defaults := 0
	configs, _ = llmConfigSvc.ListConfigs(bgCtx)
	for _, c := range configs {
		if c.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("is_default=true 的配置应恰好 1 条, 实际 %d", defaults)
	}

	// 获取当前默认配置
	activeCfg = mgr.GetConfig()
	if activeCfg.LLMModel != "gpt-4o" {
		t.Errorf("新默认模型应为 gpt-4o, 实际 %s", activeCfg.LLMModel)
	}
	t.Logf("热替换成功: 新默认模型 = %s", activeCfg.LLMModel)
}

// TestChatService_SessionFlow 验证带有 LLM 配置的会话完整流程。
func TestChatService_SessionFlow(t *testing.T) {
	// 清理
	knowledgeSvcDB.Exec("DELETE FROM chat_messages")
	knowledgeSvcDB.Exec("DELETE FROM chat_sessions")
	knowledgeSvcDB.Exec("DELETE FROM knowledge_articles")
	knowledgeSvcDB.Exec("DELETE FROM knowledge_bases")
	knowledgeSvcDB.Exec("DELETE FROM users WHERE username LIKE 'chattest_%'")

	// 创建用户
	now := time.Now()
	user := &model.User{
		Username: "chattest_user", PasswordHash: "$2a$10$hash",
		RealName: "Chat测试", Phone: "10000000300", Status: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := knowledgeSvcDB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 创建知识库
	kb := &model.KnowledgeBase{
		Name: "Chat会话测试库", EmbeddingModel: "qwen3-emb",
		VectorDimension: 1024, CreatedBy: user.ID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := knowledgeSvcDB.Create(kb).Error; err != nil {
		t.Fatalf("创建知识库失败: %v", err)
	}

	// 创建 ChatService
	knowledgeRepo := repository.NewKnowledgeRepo(knowledgeSvcDB)
	chatRepo := repository.NewChatRepo(knowledgeSvcDB)
	chatSvc := service.NewChatService(knowledgeRepo, chatRepo, nil, service.RAGDefaults{TopK: 3}, nil)

	// 创建会话
	session, err := chatSvc.CreateSession(bgCtx, request.CreateSessionRequest{
		KBID: kb.ID, Title: "端到端会话测试",
	}, user.ID)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if session.ID == 0 {
		t.Error("Session ID 应为非零")
	}
	t.Logf("会话创建: id=%d, question=%s", session.ID, session.Question)

	// 提交反馈
	err = chatSvc.SubmitFeedback(bgCtx, session.ID, user.ID, 1) // 1=已解决
	if err != nil {
		t.Fatalf("SubmitFeedback 失败: %v", err)
	}

	// 查询详情
	detail, err := chatSvc.GetChatDetail(bgCtx, session.ID, user.ID)
	if err != nil {
		t.Fatalf("GetChatDetail 失败: %v", err)
	}
	if detail.Feedback != 1 {
		t.Errorf("Feedback 应为 1, 实际 %d", detail.Feedback)
	}

	// 列表查询
	sessions, total, err := chatSvc.ListSessions(bgCtx, user.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListSessions 失败: %v", err)
	}
	if total < 1 {
		t.Errorf("应至少有 1 个会话, 实际 %d", total)
	}
	_ = sessions

	// 删除会话
	err = chatSvc.DeleteSession(bgCtx, session.ID, user.ID)
	if err != nil {
		t.Fatalf("DeleteSession 失败: %v", err)
	}
	t.Log("会话端到端流程测试通过 ✓")
}
