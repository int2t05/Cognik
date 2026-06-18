# LLM 配置管理与热重载数据流

> LLM 配置支持双提供商（llama.cpp / OpenAI-compatible），通过热重载实现运行时切换，
> 无需重启服务。

---

## 输入
```
POST /api/v1/admin/llm-configs
Authorization: Bearer <admin_jwt>
{
  "name": "DeepSeek",
  "provider": "openai_compatible",
  "base_url": "https://api.deepseek.com/v1",
  "api_key": "sk-xxxxx",
  "llm_model": "deepseek-chat",
  "embedding_model": "deepseek-embed",
  "max_tokens": 4096,
  "system_prompt": "你是运维专家，请基于知识库回答问题。",
  "is_default": true
}
```

## 分层数据流

### 0. 路由 & 中间件

1. `router.Setup()` → `registerAdminRoutes()` 注册：
   - `POST /llm-configs` → `middleware.RequirePermission(PermSystemConfig)` → `LLMConfigHandler.CreateConfig`

### 接入层 — Handler

2. 经由 `LLMConfigHandler.CreateConfig(c)` 处理：
   - `c.ShouldBindJSON(&req)` → `request.CreateLLMConfigRequest`
   - `h.svc.Create(c.Request.Context(), req)`

### 业务层 — Service

3. 经由 `LLMConfigService.Create(ctx, req)` 处理：
   - 名称唯一性校验
   - `repo.Create(ctx, &LLMConfig{...})` — 写入 llm_configs 表
   - 若 `req.IsDefault == true`：调用 `configMgr.SetConfig(config)` — 触发热重载

---

## 热重载机制

```
┌───────────────────────────────────────────────────────────────────────┐
│                     LLMConfigManager（atomic.Value）                    │
│                                                                       │
│  SetConfig(config) ──→ 写入 atomic.Value（线程安全，无需锁）            │
│  GetConfig()       ──→ 读取 atomic.Value（纳秒级延迟）                 │
│  OnChange(cb)      ──→ 注册变更回调                                   │
└───────────────────────────────────────────────────────────────────────┘
```

### 变更回调链路（在 wireApp 中注册）

4. `configMgr.OnChange(func() { ... })` 回调：
   - `configMgr.GetConfig()` — 获取新的默认 LLM 配置
   - `adapter.NewOpenAIClient(newBaseURL, newAPIKey, llmTimeout)` — 重建 LLM 客户端
   - `llmService.SetLLMClient(newClient)` — 替换 LLMService 的 LLM 客户端
   - `adapter.NewOpenAIEmbeddingClient(embedBaseURL, newAPIKey, newEmbeddingModel, embedTimeout)` — 重建 Embedding 客户端
   - `embedder.SetClient(newEmbedClient)` — 替换 Embedder 的 Embedding 客户端
   - 整个 LLM 调用链和 Embedding 链实时切换，不依赖重启

### 热重载生效路径（后续请求自动使用新配置）

5. 每次 Chat/Embedding 调用时：
   - `LLMService.getModelConfig()` — `configMgr.GetConfig()` 读取当前默认配置
   - `LLMService.buildMessages()` — `configMgr.GetConfig().SystemPrompt` 读取系统提示词
   - 优先级：DB 热配置 > config.yaml 默认值

6. Embedding 调用：
   - `Embedder.Embed(ctx, texts)` → `client.Embeddings(ctx, EmbeddingRequest{Model, Input})`
   - client 已被热重载回调替换为新的 `OpenAIEmbeddingClient`

---

## 配置管理 CRUD 数据流

### 创建配置
```
POST /api/v1/admin/llm-configs
→ LLMConfigHandler.CreateConfig
  → LLMConfigService.Create(ctx, req)
    → llmConfigRepo.Create(ctx, config)     // INSERT INTO llm_configs
    → isDefault → configMgr.SetConfig(config) // 热重载
```

### 列出全部配置
```
GET /api/v1/admin/llm-configs
→ LLMConfigHandler.ListConfigs
  → LLMConfigService.List(ctx)
    → llmConfigRepo.FindAll(ctx)            // SELECT * FROM llm_configs ORDER BY id
  → 脱敏处理（API Key 仅显示前 4 位 + ****）
```

### 获取单个配置
```
GET /api/v1/admin/llm-configs/:id
→ LLMConfigHandler.GetConfig
  → LLMConfigService.GetByID(ctx, id)
    → llmConfigRepo.FindByID(ctx, id)
  → 脱敏后返回
```

### 更新配置
```
PUT /api/v1/admin/llm-configs/:id
→ LLMConfigHandler.UpdateConfig
  → LLMConfigService.Update(ctx, id, req)
    → llmConfigRepo.FindByID(ctx, id)       // 校验存在
    → 字段更新
    → llmConfigRepo.Update(ctx, config)     // UPDATE llm_configs
    → isDefault → configMgr.SetConfig(config) // 热重载
```

### 删除配置
```
DELETE /api/v1/admin/llm-configs/:id
→ LLMConfigHandler.DeleteConfig
  → LLMConfigService.Delete(ctx, id)
    → llmConfigRepo.FindByID(ctx, id)       // 校验存在
    → llmConfigRepo.Delete(ctx, id)         // DELETE FROM llm_configs WHERE id = ?
    → 若删除的是默认配置 → configMgr.SetConfig(nil) // 清除默认
```

### 测试连接
```
POST /api/v1/admin/llm-configs/:id/test
→ LLMConfigHandler.TestConnection
  → LLMConfigService.Test(ctx, id)
    → llmConfigRepo.FindByID(ctx, id)       // 获取配置
    → adapter.NewOpenAIClient(baseURL, apiKey, 10s) // 临时客户端
    → client.ChatCompletion(ctx, ChatRequest{
        Model: config.LLMModel,
        Messages: [{role:"user", content:"hello"}],
        MaxTokens: 10
      })
    → 返回 {success, latency_ms, tokens_used}
```
