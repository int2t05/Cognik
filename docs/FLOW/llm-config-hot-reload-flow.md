# LLM 配置管理与热重载数据流

> **聚焦：后端数据逻辑。不含前端调用链和用户故事。**

---

## 1. 路由注册

```
router.Setup() → registerAdminRoutes():
  GET    /api/v1/admin/llm-configs       → LLMConfigHandler.ListConfigs     [PermSystemConfig]
  POST   /api/v1/admin/llm-configs       → LLMConfigHandler.CreateConfig    [PermSystemConfig]
  GET    /api/v1/admin/llm-configs/:id   → LLMConfigHandler.GetConfig       [PermSystemConfig]
  PUT    /api/v1/admin/llm-configs/:id   → LLMConfigHandler.UpdateConfig    [PermSystemConfig]
  DELETE /api/v1/admin/llm-configs/:id   → LLMConfigHandler.DeleteConfig    [PermSystemConfig]
  POST   /api/v1/admin/llm-configs/:id/test → LLMConfigHandler.TestConnection [PermSystemConfig]
```

---

## 2. 数据模型

### `llm_configs` 表

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | BIGINT PK | 自增主键 |
| Name | VARCHAR | 配置名称（如 "DeepSeek-v3"） |
| ProviderType | INT | 1=llama.cpp, 2=OpenAI-compatible |
| BaseURL | VARCHAR | LLM API 地址 |
| EmbeddingBaseURL | VARCHAR | Embedding API 地址（空则回退 BaseURL） |
| APIKey | VARCHAR | API 密钥（BeforeSave 加密/AfterFind 解密） |
| LLMModel | VARCHAR | 模型名（如 "deepseek-chat"） |
| EmbeddingModel | VARCHAR | Embedding 模型名（如 "bge-m3"） |
| MaxTokens | INT | 最大生成 Token（默认 8192） |
| VectorDimension | INT | 向量维度（默认 1024） |
| SystemPrompt | TEXT | 自定义系统提示词 |
| IsDefault | BOOL | 是否为默认配置 |

### API Key 加解密机制

```
BeforeSave hook:
  APIKey = crypto.Encrypt(secret, plaintext)
  → AES-GCM 加密 → base64 编码 → 存库

AfterFind hook:
  APIKey = crypto.Decrypt(secret, ciphertext)
  → base64 解码 → AES-GCM 解密 → 内存明文

API 响应脱敏 (MarshalJSON):
  返回 "前4位****后4位" 格式
```

---

## 3. 热重载核心: `LLMConfigManager`

### 数据结构

```go
type LLMConfigManager struct {
    current  atomic.Value  // 持有 *model.LlmConfig 或 nil，无锁读写
    onChange func()        // 配置变更回调（在 wireApp 依赖注入时注册）
}
```

### 核心方法

```
SetConfig(config):
  1. current.Store(config)   // atomic 写入（纳秒级延迟）
  2. if onChange != nil: onChange()  // 触发回调

GetConfig() *model.LlmConfig:
  1. v := current.Load()    // atomic 读取
  2. v == nil → return nil
  3. return v.(*model.LlmConfig)
```

### OnChange 回调（在 wireApp 装配时注册）

```
configMgr.OnChange(func() {
  1. cfg := configMgr.GetConfig()

  2. 重建 LLM 客户端:
     llmClient = adapter.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, llmTimeout)
     llmService.SetLLMClient(llmClient)  // 替换运行时引用

  3. 重建 Embedding 客户端:
     embedClient = adapter.NewOpenAIEmbeddingClient(
       cfg.GetEmbeddingBaseURL(), // EmbeddingBaseURL || BaseURL
       cfg.APIKey, cfg.EmbeddingModel, embedTimeout,
     )
     embedder.SetClient(embedClient)  // 替换运行时引用
})
```

### 热重载生效路径

```
每次 LLM 调用:
  LLMService.getModelConfig()
    → cfg := configMgr.GetConfig()  // 读 atomic.Value
    → model = cfg.LLMModel || defaultModel (config.yaml)
    → maxTokens = cfg.MaxTokens || 2048

  LLMService.buildMessages()
    → systemPrompt = configMgr.GetConfig().SystemPrompt || "你是一个运维知识助手..."

每次 Embedding 调用:
  Embedder.Embed()
    → embedder.client.Embeddings(...)  // client 已被 OnChange 回调替换
```

---

## 4. CRUD 操作数据流

### 4.1 创建 — `LLMConfigService.CreateConfig(ctx, req)`

```
1. 校验: name 唯一, providerType ∈ {1,2}, baseURL 非空

2. 默认值: MaxTokens=8192, VectorDimension=1024

3. 分支:
   ┌─ req.IsDefault == true:
   │   db.Transaction(func(tx) {
   │     a. txRepo.ClearDefault(ctx)  → UPDATE llm_configs SET is_default=false
   │     b. txRepo.Create(ctx, &config)  → INSERT INTO llm_configs (...)
   │     c. fresh := txRepo.FindByID(ctx, config.ID)
   │     d. configMgr.SetConfig(fresh) → 触发热重载
   │   })
   │
   └─ req.IsDefault == false:
       llmConfigRepo.Create(ctx, &config)  // 直接插入，不触发重载
```

### 4.2 更新 — `LLMConfigService.UpdateConfig(ctx, id, req)`

```
1. llmConfigRepo.FindByID(ctx, id) → 校验存在

2. API Key 空值处理: req.APIKey == "" → 保留已存密文（BeforeSave 跳过空值）

3. 字段更新: name, providerType, baseURL, embeddingBaseURL, llmModel, embeddingModel, ...

4. 分支:
   ┌─ req.IsDefault == true:
   │   Transaction:
   │     a. ClearDefault → txRepo.Update → txRepo.FindByID → configMgr.SetConfig → 热重载
   └─ req.IsDefault == false:
       llmConfigRepo.Update(ctx, config)
```

### 4.3 删除 — `LLMConfigService.DeleteConfig(ctx, id)`

```
1. llmConfigRepo.FindByID(ctx, id)
2. config.IsDefault == true → 拒绝（不能删除默认配置）
3. llmConfigRepo.CountReferencingKBs(ctx, id)
   → SQL: SELECT COUNT(*) FROM knowledge_bases WHERE llm_config_id = ?
   → count > 0 → 拒绝（存在关联知识库）
4. llmConfigRepo.Delete(ctx, id) → SQL: DELETE FROM llm_configs WHERE id = ?
5. 若删除的是默认配置 → configMgr.SetConfig(nil) → OnChange → 重建空客户端 → 降级到 config.yaml
```

### 4.4 测试连接 — `LLMConfigService.Test(ctx, id)`

```
1. llmConfigRepo.FindByID(ctx, id)
2. 创建临时客户端: adapter.NewOpenAIClient(config.BaseURL, config.APIKey, 10s)
3. tempClient.ChatCompletion(ctx, ChatRequest{
     Model: config.LLMModel, Messages: [{role:"user", content:"hello"}], MaxTokens: 10
   })
4. 返回 {success, latency_ms, tokens_used, model, test_message}
```

---

## 5. 配置读取优先级

```
LLMService.getModelConfig() → (model, maxTokens):
  1. cfg := configMgr.GetConfig()        // DB 热配置 (atomic.Value)
  2. model    = cfg.LLMModel || defaultModel    // fallback: config.yaml OPSMIND_LLM_MODEL
  3. maxTokens = cfg.MaxTokens || 2048           // fallback: 硬编码默认值

优先级: DB 热配置 > config.yaml > 硬编码默认值
```

---

## 6. 完整数据流图

```
┌─ HTTP ─┐  ┌── Handler ──┐  ┌─── Service ───┐  ┌── Repository ──┐  ┌─ PostgreSQL ─┐
│         │  │             │  │                │  │                 │  │              │
│ CRUD    │→│ LLMConfig   │→│ LLMConfig      │→│ LLMConfigRepo   │→│ llm_configs  │
│ 操作    │  │ Handler     │  │ Service        │  │  .Create        │  │              │
│         │  │             │  │   └─若 default→│  │  .FindByID      │  │              │
│         │  │             │  │     configMgr  │  │  .Update        │  │              │
│         │  │             │  │     .SetConfig │  │  .Delete        │  │              │
└─────────┘  └─────────────┘  └───────┬────────┘  │  .ClearDefault  │  └──────────────┘
                                      │            │  .FindDefault   │
                           ┌──────────▼──────────┐ └─────────────────┘
                           │  LLMConfigManager    │
                           │  atomic.Value        │
                           │  ┌────────────────┐  │
                           │  │ GetConfig() ←──┼──┼── LLMService.getModelConfig()
                           │  │                │  │     (每次 LLM 调用时读取)
                           │  │ SetConfig()───→┼──┼── onChange():
                           │  │                │  │     ├─ NewOpenAIClient
                           │  │                │  │     ├─ llmService.SetLLMClient
                           │  │                │  │     ├─ NewOpenAIEmbeddingClient
                           │  │                │  │     └─ embedder.SetClient
                           │  └────────────────┘  │
                           └──────────────────────┘
```
