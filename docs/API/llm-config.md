# LLM 配置接口

> **Base URL:** `/api/v1/admin` | **Auth:** JWT + `system:config` | **Module:** LLM Configuration

LLM 配置管理 llama.cpp server 和 OpenAI-compatible API 的连接参数。配置修改后通过 `atomic.Value` 热替换**即时生效**，无需重启服务。

## 配置模型

LLM 和 Embedding 各自拥有独立的 Base URL。两个端点可指向同一服务或不同服务。提供两种方案：

| 方案 | `provider_type` | LLM | Embedding | 说明 |
|------|----------------|-----|-----------|------|
| **方案 A** — llama.cpp 本地 | 1 | llama.cpp server | llama.cpp server | 本地部署，两者共用同一服务，无需 API Key |
| **方案 B** — OpenAI-compatible | 2 | OpenAI / DeepSeek / Moonshot | 任意 OpenAI-compatible API | LLM 和 Embedding 可分别指向不同服务 |

> `embedding_base_url` 为空时回退到 `base_url`，确保配置向后兼容。

---

## 1. LLM 配置列表

```http
GET /api/v1/admin/llm-configs
Authorization: Bearer <token>
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "本地 llama.cpp",
      "provider_type": 1,
      "base_url": "http://llama-cpp:8080/v1",
      "embedding_base_url": "",
      "api_key": "",
      "llm_model": "qwen3-4b",
      "embedding_model": "bge-m3",
      "max_tokens": 8192,
      "vector_dimension": 1024,
      "is_default": true,
      "created_at": "2026-06-11T19:00:00Z",
      "updated_at": "2026-06-11T19:00:00Z"
    },
    {
      "id": 2,
      "name": "OpenAI API",
      "provider_type": 2,
      "base_url": "https://api.openai.com/v1",
      "embedding_base_url": "",
      "api_key": "sk-****Ab12",
      "llm_model": "gpt-4o-mini",
      "embedding_model": "text-embedding-3-small",
      "max_tokens": 16384,
      "vector_dimension": 1536,
      "is_default": false,
      "created_at": "2026-06-11T19:30:00Z",
      "updated_at": "2026-06-11T19:30:00Z"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int64 | 配置 ID |
| name | string | 配置名称（用户自定义标签） |
| provider_type | int | 1=llama.cpp, 2=OpenAI-compatible |
| base_url | string | LLM API 基础地址 |
| embedding_base_url | string | Embedding API 地址（空时回退到 base_url） |
| api_key | string | API 密钥掩码（如 `****`，空表示无密钥） |
| llm_model | string | 文本生成模型名称 |
| embedding_model | string | Embedding 模型名称 |
| max_tokens | int | 最大生成 Token 数 |
| vector_dimension | int | embedding 向量维度 |
| is_default | bool | 是否系统默认配置（最多一个 true） |

---

## 2. 创建 LLM 配置

```http
POST /api/v1/admin/llm-configs
Authorization: Bearer <token>
```

**方案 A — llama.cpp 本地部署：**

```json
{
  "name": "本地 llama.cpp",
  "provider_type": 1,
  "base_url": "http://llama-cpp:8080/v1",
  "embedding_base_url": "",
  "api_key": "",
  "llm_model": "qwen3-4b",
  "embedding_model": "bge-m3",
  "max_tokens": 8192,
  "vector_dimension": 1024,
  "is_default": true
}
```

**方案 B — OpenAI-compatible API（LLM / Embedding 可任意组合）：**

```json
{
  "name": "OpenAI API",
  "provider_type": 2,
  "base_url": "https://api.openai.com/v1",
  "embedding_base_url": "",
  "api_key": "sk-your-api-key",
  "llm_model": "gpt-4o-mini",
  "embedding_model": "text-embedding-3-small",
  "max_tokens": 16384,
  "vector_dimension": 1536,
  "is_default": false
}
```
> `embedding_base_url` 留空则复用 `base_url`；也可填入其他服务地址（如 `http://llama-cpp:8080/v1`），实现 LLM 和 Embedding 跨服务商组合。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | ✓ | 配置名称 |
| provider_type | int | ✓ | 1=llama.cpp, 2=OpenAI-compatible |
| base_url | string | ✓ | LLM API 基础地址 |
| embedding_base_url | string | | Embedding API 地址（空则复用 base_url） |
| api_key | string | | API 密钥（llama.cpp 通常为空；数据库 AES-256 加密存储） |
| llm_model | string | ✓ | 文本生成模型名称 |
| embedding_model | string | ✓ | Embedding 模型名称 |
| max_tokens | int | | 最大生成 Token 数，默认 8192（建议 4096-32768） |
| vector_dimension | int | | 向量维度，默认 1024（bge-m3=1024, text-embedding-3-small=1536） |
| is_default | bool | | 是否设为默认配置（设为 true 时自动将旧默认改为 false） |

**业务规则：**
- 系统最多一个默认配置。设为 `is_default=true` 时，旧的默认配置自动改为 `false`。
- `api_key` 在数据库中以 AES-256 加密存储，API 响应中掩码显示（仅显示前 4 位和后 4 位，如 `sk-****Ab12`）。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "本地 llama.cpp",
    "provider_type": 1,
    "base_url": "http://llama-cpp:8080/v1",
    "embedding_base_url": "",
    "api_key": "",
    "llm_model": "qwen3-4b",
    "embedding_model": "bge-m3",
    "max_tokens": 8192,
    "vector_dimension": 1024,
    "is_default": true,
    "created_at": "2026-06-11T19:00:00Z",
    "updated_at": "2026-06-11T19:00:00Z"
  }
}
```

**错误：**

| code | HTTP 状态 | 说明 |
|------|----------|------|
| 10003 | 400 | 参数校验失败（name/provider/model 未提供） |
| 10005 | 409 | 配置名称已存在 |

---

## 3. LLM 配置详情

```http
GET /api/v1/admin/llm-configs/:id
Authorization: Bearer <token>
```

**响应：** 同列表项结构

**错误：**

| code | HTTP 状态 | 说明 |
|------|----------|------|
| 10003 | 400 | 无效的配置 ID |
| 10004 | 404 | 配置不存在 |

---

## 4. 更新 LLM 配置

```http
PUT /api/v1/admin/llm-configs/:id
Authorization: Bearer <token>
```

**请求体：** 同创建（全量替换），其中 `max_tokens` 默认 8192，`vector_dimension` 默认 1024，不传时使用默认值。

> 修改后通过 `atomic.Value` 热替换内存中配置，**即时生效**，无需重启。注意 `api_key` 不传时保留原有密钥（不传递不等同于清空）。

**错误：**

| code | HTTP 状态 | 说明 |
|------|----------|------|
| 10003 | 400 | 参数校验失败或无效 ID |
| 10004 | 404 | 配置不存在 |
| 10005 | 409 | 配置名称已存在 |

---

## 5. 删除 LLM 配置

```http
DELETE /api/v1/admin/llm-configs/:id
Authorization: Bearer <token>
```

**错误响应：**

```json
{
  "code": 10003,
  "message": "不能删除默认配置，请先设置其他配置为默认"
}
```

**错误：**

| code | HTTP 状态 | 说明 |
|------|----------|------|
| 10003 | 400 | 不能删除默认配置 |
| 10004 | 404 | 配置不存在 |

> 不允许删除当前 `is_default=true` 的配置。需先将其他配置设为默认后再删除。

---

## 6. 测试 LLM 连接

```http
POST /api/v1/admin/llm-configs/:id/test
Authorization: Bearer <token>
```

发送一个简短的 ChatCompletion 请求验证 Base URL 可达性和模型可用性。

**成功响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "latency_ms": 120,
    "tokens_used": 15,
    "model": "qwen3-4b",
    "test_message": "Hello, this is a test."
  }
}
```

**失败响应：**

```json
{
  "code": 20001,
  "message": "AI 服务不可用：dial tcp: i/o timeout"
}
```

> 测试连接超时 10 秒。失败时返回 `code=20001`（AI 服务不可用）。

**错误：**

| code | HTTP 状态 | 说明 |
|------|----------|------|
| 10003 | 400 | 无效的配置 ID |
| 20001 | 503 | 连接测试失败（超时、不可达或模型无响应） |
| 99999 | 500 | 服务未初始化 |

