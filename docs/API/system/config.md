# 系统配置接口

> **Base URL:** `/api/v1/admin/configs` 与 `/api/v1/public/configs` | **Auth:** JWT + `system:config`（admin 端） / 无（public 端） | **Module:** System Config

所有配置从 `.env` 读取，不入库。写入 `.env` 后触发热重建（LLM/Embedding 客户端原子替换，零锁读）。

## 1. 公开配置（无需认证）

```http
GET /api/v1/public/configs/:key
```

仅 `app_name` 可公开读取，用于前端标题/登录页展示。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": "Cognik"
}
```

## 2. LLM/Embedding 信息（只读）

```http
GET /api/v1/admin/configs/llm-info
Authorization: Bearer <token>
```

返回 `.env` 派生的 LLM/Embedding 配置（不含 API key）。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "llm_base_url": "https://your-llm/v1",
    "llm_model": "glm-5.2",
    "embedding_base_url": "https://your-embedding/v1",
    "embedding_model": "text-embedding-v2",
    "embedding_dimension": 1536
  }
}
```

## 3. 全部配置项（API key 脱敏）

```http
GET /api/v1/admin/configs/env
Authorization: Bearer <token>
```

返回所有可管理的 `.env` 配置项，key 即真实环境变量名（如 `COGNIK_LLM_MODEL`），API key 类字段脱敏显示前 8 位。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    { "key": "COGNIK_APP_NAME", "value": "Cognik" },
    { "key": "COGNIK_LLM_BASE_URL", "value": "https://your-llm/v1" },
    { "key": "COGNIK_LLM_API_KEY", "value": "7cbc03aa..." }
  ]
}
```

## 4. 更新配置项（触发热重建）

```http
PUT /api/v1/admin/configs/env
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "key": "COGNIK_LLM_MODEL",
  "value": "qwen3-4b"
}
```

写入 `.env` 对应行并重载配置；随后热重建 LLM（`ChatModelFactory.BuildFromEnv`，atomic.Store）与 Embedding 客户端（`Embedder.SetClient`，atomic.Pointer），无需重启即生效。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```
