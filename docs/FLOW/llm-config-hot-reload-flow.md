# LLM 配置热重载数据流 — 每个 API 端点

> 涉及文件: `handler/llm_config.go`, `service/llm_config_service.go`, `repository/llm_config_repo.go`, `service/llm_service.go`, `service/chat_service.go`, `adapter/llm_client.go`, `adapter/embedding_client.go`, `rag/embedder.go`, `model/llm_config.go`

---

## LLMConfigManager — 热重载核心

`LLMConfigManager` (service/llm_config_service.go) 使用 `atomic.Value` 无锁读写:

```
LLMConfigManager.SetConfig (service/llm_config_service.go 内部)
  → current.Store(config) — atomic 写入, 纳秒级
  → if onChange != nil: onChange() — 触发回调

LLMConfigManager.GetConfig (service/llm_config_service.go:38)
  → current.Load() — atomic 读取

LLMConfigManager.OnChange (service/llm_config_service.go:33)
  → 注册回调: configMgr 变更时自动重建客户端
```

### OnChange 回调（wireApp 装配时注册）

```
1. cfg := configMgr.GetConfig()
2. OpenAIClient ← NewOpenAIClient (adapter/llm_client.go:68)
      (cfg.BaseURL, cfg.APIKey, llmTimeout)
3. LLMService.SetLLMClient (service/llm_service.go:103)
4. OpenAIEmbeddingClient ← NewOpenAIEmbeddingClient (adapter/embedding_client.go:62)
      (cfg.GetEmbeddingBaseURL(), cfg.APIKey, cfg.EmbeddingModel, embedTimeout)
5. Embedder.SetClient (rag/embedder.go:42)
```

### 热配置生效路径

```
每次 LLM 调用 → LLMService.getModelConfig (service/llm_service.go 内部):
  model = configMgr.GetConfig().LLMModel || defaultModel (config.yaml)
  maxTokens = configMgr.GetConfig().MaxTokens || 2048

每次 LLM 调用 → buildMessages:
  systemPrompt = configMgr.GetConfig().SystemPrompt || "你是一个运维知识助手..."

每次 Embedding → Embedder.Embed:
  client 已被 OnChange 回调替换
```

优先: `DB 热配置 (atomic.Value)` > `config.yaml` > `硬编码默认值`

---

## CRUD 端点

### GET /api/v1/admin/llm-configs &emsp; 列出全部 &emsp; [PermSystemConfig]

```
LLMConfigHandler.ListConfigs (handler/llm_config.go:52)
  → LLMConfigService.ListConfigs (service/llm_config_service.go:211)
    └─ LlmConfigRepo.List (repository/llm_config_repo.go:62)
        → SELECT * FROM llm_configs ORDER BY id ASC
        → AfterFind: 解密 APIKey → 内存明文
    返回: APIKey 脱敏 (LlmConfigResponse.MarshalJSON: 前4位****后4位)
```

### POST /api/v1/admin/llm-configs &emsp; 创建 &emsp; [PermSystemConfig]

**输入** `{"name":"DeepSeek","provider_type":2,"base_url":"https://api.deepseek.com/v1",`<br/>`"api_key":"sk-xxx","llm_model":"deepseek-chat","embedding_model":"bge-m3","is_default":true}`

```
LLMConfigHandler.CreateConfig (handler/llm_config.go:64)
  → LLMConfigService.CreateConfig (service/llm_config_service.go:103)
    ├─ 校验: name 唯一, providerType∈{1,2}, baseURL 非空
    ├─ 默认值: MaxTokens=8192, VectorDimension=1024
    │
    ├─ is_default=true:
    │   GormTxManager.Transaction:
    │     ├─ LlmConfigRepo.ClearDefault (repository/llm_config_repo.go:83)
    │     │   → UPDATE llm_configs SET is_default=false
    │     ├─ LlmConfigRepo.Create (repository/llm_config_repo.go:33)
    │     │   → BeforeSave: AES-GCM 加密 APIKey → INSERT
    │     ├─ LlmConfigRepo.FindByID → AfterFind 解密 → 热配置
    │     └─ LLMConfigManager.SetConfig → 触发热重载 → OnChange 回调
    │
    └─ is_default=false: LlmConfigRepo.Create → 直接插入（不触发重载）
```

### GET /api/v1/admin/llm-configs/:id &emsp; 详情 &emsp; [PermSystemConfig]

```
LLMConfigHandler.GetConfig (handler/llm_config.go:83)
  → LLMConfigService.GetConfig (service/llm_config_service.go:229)
    └─ LlmConfigRepo.FindByID → AfterFind 解密 → 返回完整配置 (含明文 Key, 供测试连接用)
```

### PUT /api/v1/admin/llm-configs/:id &emsp; 更新 &emsp; [PermSystemConfig]

**输入** `{"name":"DeepSeek-v3","api_key":"",...}` (留空 APIKey 保留已存密文)

```
LLMConfigHandler.UpdateConfig (handler/llm_config.go:101)
  → LLMConfigService.UpdateConfig (service/llm_config_service.go:161)
    ├─ LlmConfigRepo.FindByID → 校验存在
    ├─ APIKey 空值: BeforeSave 跳过 → 保留已存密文
    │
    └─ is_default 变更:
        Transaction → ClearDefault → Update → FindByID → SetConfig → 热重载
```

### DELETE /api/v1/admin/llm-configs/:id &emsp; 删除 &emsp; [PermSystemConfig]

```
LLMConfigHandler.DeleteConfig (handler/llm_config.go:140)
  → LLMConfigService.DeleteConfig (service/llm_config_service.go:237)
    ├─ config.IsDefault → 拒绝 (不能删默认配置)
    ├─ LlmConfigRepo.CountReferencingKBs (repository/llm_config_repo.go:76)
    │   → SELECT COUNT(*) FROM knowledge_bases WHERE llm_config_id=?
    │   → count>0 → 拒绝 (存在关联知识库)
    ├─ LlmConfigRepo.Delete → DELETE FROM llm_configs WHERE id=?
    └─ 若为默认 → configMgr.SetConfig(nil) → OnChange → 重建空客户端 → 降级到 config.yaml
```

### POST /api/v1/admin/llm-configs/:id/test &emsp; 测试连接 &emsp; [PermSystemConfig]

```
LLMConfigHandler.TestConnection (handler/llm_config.go:157)
  → LLMConfigService.TestConnection (service/llm_config_service.go 内部)
    ├─ LlmConfigRepo.FindByID → AfterFind 解密
    ├─ 临时客户端: NewOpenAIClient(config.BaseURL, config.APIKey, 10s)
    └─ OpenAIClient.ChatCompletion (adapter/llm_client.go:108)
        → POST /chat/completions {model, messages:[{role:"user",content:"hello"}], max_tokens:10}
    → 返回 {success, latency_ms, tokens_used, model}
```

---

## 数据模型与加解密

`model/llm_config.go`:

```
LlmConfig.BeforeSave (model/llm_config.go:43)
  → crypto.Encrypt(secret, APIKey) → AES-GCM → base64

LlmConfig.AfterFind (model/llm_config.go:55)
  → crypto.Decrypt(secret, ciphertext) → base64 → AES-GCM → 明文

LlmConfig.GetEmbeddingBaseURL (model/llm_config.go:70)
  → 返回 EmbeddingBaseURL || BaseURL
```

## 完整数据流

```
CRUD 操作 →
  LLMConfigHandler →
    LLMConfigService →
      LlmConfigRepo (BeforeSave 加密/AfterFind 解密) →
        PostgreSQL llm_configs
      └─ is_default=true →
        LLMConfigManager.SetConfig (atomic.Value) →
          OnChange 回调 →
            NewOpenAIClient → LLMService.SetLLMClient
            NewOpenAIEmbeddingClient → Embedder.SetClient
              ↓
        所有后续 Chat/Embedding 调用自动使用新配置
```
