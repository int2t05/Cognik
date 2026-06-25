# 置信度算法设计

> **版本：** v1.1 | **日期：** 2026-06-25 | **状态：** 设计阶段

## 1. 概述

定义 OpsMind RAG 智能问答的三层置信度评分体系：

| 层次 | 粒度 | 用途 | 计算时机 |
|------|------|------|----------|
| Chunk 展示分 | 单条检索结果 | 前端展示各来源的相对匹配度（进度条） | 检索完成后 |
| 原始综合分 (Conf_raw) | 单次对话 | 落库持久化，供分位数统计，全局可比 | LLM 生成完成后 |
| 置信度等级 (level) | 单次对话 | 前端话术选择、交互决策（低/中/高） | Conf_raw 与阈值比较 |

**设计原则：** 在线链路低延迟（仅一次额外 embedding）→ 原始分全局可比（[0,1]）→ 阈值自适应（历史分位数）→ 兼容管线降级。

---

## 2. 分数算法

### 2.1 Chunk 展示分

展示分仅用于前端渲染匹配度进度条，不参与 Conf_raw 计算，不落库。

**算法：** 对当前检索批次的最终管线分数做线性归一化到 [0,1]：

```
display_score_i = (raw_i - min_batch) / (max_batch - min_batch)
```

其中 `raw_i` 取最终管线分数：rerank 可用时取 rerank 分数，否则取 RRF/向量余弦相似度/BM25 分数。批次内分数全相同时全部设为 1.0，单 chunk 直接设为 1.0。

**为什么用批次归一化：** BM25 和 RRF 分数的绝对值在不同查询间不可比，展示分只需回答"在当前检索中该 chunk 有多靠前"——前端进度条语义恰好如此。

### 2.2 原始综合分 (Conf_raw)

采用"检索聚合 + 答案质量校验"混合方案：

```
Conf_raw = α × S_retrieval + (1-α) × S_qa
```

默认 α = 0.7（检索为主，答案校验为辅）。

#### S_retrieval — 检索聚合分

使用 **原始向量余弦相似度**（`1 - (embedding <=> query)`，天然 [0,1]），按检索结果排序做排名加权平均：

```
S_retrieval = Σ(w_i × raw_cosine_i) / Σ(w_i)
w_i = 1 / (rank_i + 1)     // rank 从 0 开始：w_0=1.0, w_1=0.5, w_2=0.33...
```

**为什么用原始余弦而非批次归一化分数：** 批次归一化后 top chunk 永远接近 1.0，会导致好检索和差检索产生相近的聚合分——破坏全局可比性。原始余弦相似度天然 [0,1] 且跨查询可比。

**降级处理：**
- 纯 BM25 模式（无向量检索）：用 BM25 分数的批次内归一化值替代 raw_cosine
- 检索结果为空：S_retrieval = 0

**所需数据变更：** `RetrievalResult` 需新增 `RawCosineScore float64` 字段，在向量检索时填充。

#### S_qa — 问答匹配分

```
S_qa = cosine(embed(question), embed(answer))
```

LLM 生成完成后，对 answer 做一次 embedding，与已缓存的 question embedding 计算余弦相似度。

**为什么需要 S_qa：** 检索分高不代表答案好——LLM 可能忽略检索文档或检索到不相关文档。S_qa 检测"答案是否在回答用户的问题"，作为轻量级事后校验。

**可靠性补偿（短答案噪声处理）：**

| answer 长度 | S_qa 权重调整 |
|-------------|--------------|
| ≥ 20 字符 | 正常：α = 0.7 |
| 5 ~ 19 字符 | 降权：α = 0.85 |
| < 5 字符或纯空白 | 归零：α = 1.0（S_qa 不计入） |

短答案（"是的"、"请参考上述步骤"）embedding 信息量少，余弦相似度噪声大，降低其影响。

**S_qa 计算失败：** embedding 服务超时/不可达 → α = 1.0，Conf_raw = S_retrieval。

#### 特殊场景

| 场景 | Conf_raw |
|------|----------|
| LLM 生成空回答 | **强制 0**（即使检索质量高） |
| RAG 禁用 | 0 |
| 检索为空 | S_retrieval = 0，α = 0.7（保留 S_qa） |

---

## 3. 置信度分级

### 3.1 三级判定

```
if   Conf_raw >= T_high  →  high  （高置信）
elif Conf_raw >= T_low   →  medium（中置信）
else                     →  low   （低置信）
```

### 3.2 阈值配置

存储于 `system_configs`，支持热更新：

| 配置键 | 含义 | 默认值 |
|--------|------|--------|
| `ai.confidence_threshold_high` | 高置信下限 | 0.70 |
| `ai.confidence_threshold_low` | 中置信下限 | 0.40 |

默认值仅为首日兜底，上线积累数据后通过分位数计算校准。

### 3.3 前端话术与交互

| 等级 | 答案展示 | 来源引用 | 转人工引导 |
|------|----------|----------|-----------|
| high | 正常输出 | 展示 | 不引导 |
| medium | 答案前自动插入「匹配资料有限，内容仅供参考」 | 展示 | 建议提交申告 |
| low | 正常输出，**答案上方**展示醒目警告条 | **不展示** | 自动展开申告表单 |

**低置信不隐藏答案的原因：** SSE 流式已将答案逐 token 展示给用户，事后隐藏会造成困惑。改为警告条 + 自动展开申告入口，既尊重用户已看到的内容，又明确提示风险。

低置信警告条文案：「⚠️ 以下回答匹配的资料有限，可能不够准确，建议提交申告由运维人员确认」

**can_submit_ticket 逻辑：** `level == "low" || level == "medium"`

---

## 4. 阈值分位数计算

### 4.1 流程

管理员在系统配置页点击"计算阈值"→ 后端计算 → 返回 P30/P70 → 管理员确认 → 一键写入。

### 4.2 后端计算

**步骤 1：采样**

```sql
SELECT confidence_raw
FROM chat_messages
WHERE role = 'assistant'
  AND status = 'completed'
  AND confidence_raw IS NOT NULL
  AND content != ''
  AND created_at >= NOW() - INTERVAL ':days days'
```

注意：**不过滤 `confidence_raw = 0`**。RAG 禁用/检索为空的对话本身就是"低质量"的真实信号，排除它们会系统性抬高分布。

**步骤 2：线性插值法计算 P30、P70**

```
P30 → 建议作为 T_low
P70 → 建议作为 T_high
```

**步骤 3：合理性校验**

| 条件 | 处理 |
|------|------|
| 样本数 = 0 | 返回默认值 (0.40, 0.70)，提示无数据 |
| 样本数 < 50 | 正常返回但附带警告 |
| P30 < 0.10 | 钳位到 0.10 |
| P70 > 0.95 | 钳位到 0.95 |
| P70 - P30 < 0.10 | P70 = max(P70, P30 + 0.10) |

### 4.3 API

```
POST /api/v1/admin/confidence/compute-thresholds
```

请求：`{ "days": 7 }`（默认 7，范围 1~90）

响应：
```json
{
  "code": 0,
  "data": {
    "p30": 0.42,
    "p70": 0.76,
    "sample_count": 1234,
    "date_range": { "from": "2026-06-18", "to": "2026-06-25" },
    "warning": ""
  }
}
```

管理员确认后，前端调用现有配置 API 写入（需做事务性处理——两个都成功才算完成，任一失败则回滚第一个）：

```
PUT /api/v1/admin/configs/ai.confidence_threshold_low   {"value": 0.42}
PUT /api/v1/admin/configs/ai.confidence_threshold_high  {"value": 0.76}
```

---

## 5. 数据模型变更

### 5.1 chat_messages

```sql
ALTER TABLE chat_messages ADD COLUMN confidence_raw DOUBLE PRECISION;
COMMENT ON COLUMN chat_messages.confidence_raw IS '原始综合置信度 [0,1]，用于分位数统计';
```

保留现有 `confidence` 列但不再写入新值（向后兼容）。前端和数据看板优先读 `confidence_raw`，NULL 时 fallback 到 `confidence`。

### 5.2 system_configs

```sql
INSERT INTO system_configs (config_key, config_value, value_type, description) VALUES
  ('ai.confidence_threshold_high', '0.70', 'number', '高置信度阈值'),
  ('ai.confidence_threshold_low',  '0.40', 'number', '中置信度阈值（低于此值为低置信）');
```

现有 `ai.threshold` 键标记废弃，不再参与 `can_submit_ticket` 判定。

---

## 6. SSE 协议变更

### 6.1 新增 `chunks` 事件

检索完成后、LLM 生成前发送。**不发送 chunk 内容文本**（避免安全泄露和流式噪声），仅发送标识与分数：

```
data: {"type":"chunks","chunks":[{"id":142,"score":1.0,"source":"VPN FAQ"},{"id":89,"score":0.72,"source":"账号管理"}]}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| chunks[].id | int64 | chunk ID |
| chunks[].score | float | 展示分 [0,1]（批次归一化后的值） |
| chunks[].source | string | 来源文档名称 |

### 6.2 done 事件扩展

```json
{
  "type": "done",
  "metadata": {
    "...": "...",
    "confidence_raw": 0.76,
    "confidence_level": "high",
    "can_submit_ticket": false
  }
}
```

| 新增字段 | 类型 | 说明 |
|----------|------|------|
| confidence_raw | float | 原始综合分 [0,1] |
| confidence_level | string | `"high"` / `"medium"` / `"low"` |

现有 `confidence` 字段保留，值同 `confidence_raw`。现有 `sources` 数组保留，仅在 `level != "low"` 时填充。

---

## 7. 降级与边界

| 场景 | Conf_raw | Level | 说明 |
|------|----------|-------|------|
| 正常（全管线） | 完整计算 | 正常判定 | — |
| Rerank/BM25 不可用 | 完整计算 | 正常判定 | S_retrieval 用原始余弦，不受影响 |
| S_qa embedding 失败 | α=1.0 | 仅基于 S_retrieval | 日志记录降级 |
| LLM 生成空回答 | 0 | low | 覆盖 S_retrieval |
| RAG 禁用 | 0 | low | — |
| 检索为空 | S_retrieval=0 | 基于 S_qa 判定 | — |
| 首日无历史数据 | — | — | 使用默认阈值 (0.40, 0.70) |

---

## 8. 实施清单

| # | 变更项 | 模块 |
|---|--------|------|
| 1 | `RetrievalResult` 新增 `RawCosineScore` 字段 | rag/types.go |
| 2 | 向量检索/BM25 检索时填充原始分数 | rag/retriever.go, rag/bm25.go |
| 3 | Pipeline 检索完成后计算 chunk 展示分 | rag/pipeline.go |
| 4 | 实现 S_retrieval + S_qa + Conf_raw + 短答案降权 | service/llm_service.go |
| 5 | done 事件扩展 `confidence_raw` + `confidence_level` | service/llm_service.go |
| 6 | 新增 `chunks` SSE 事件发送 | service/llm_service.go |
| 7 | `chat_messages` 新增 `confidence_raw` 列 | migration |
| 8 | 落库写入 `confidence_raw` | service/chat_service.go |
| 9 | `system_configs` 新增两条阈值配置 | migration/seed |
| 10 | 实现 `POST /api/v1/admin/confidence/compute-thresholds` | handler + service |
| 11 | 会话详情/列表响应扩展 | dto/response/chat.go |
| 12 | 前端：chunk 分数卡片 + 置信度标签 + 阈值配置页 | web |
| 13 | 更新 `docs/API/chat.md` | docs |

---

## 9. 兼容性

- 现有 `confidence` 列保留，`confidence_raw` 为 NULL 时前端/data 看板 fallback 读取 `confidence`
- SSE `done` 事件仅新增字段，不删除
- 现有 `ai.threshold` 键保留但不再被 `can_submit_ticket` 引用
- 权重 α 可通过常量调整，后续如反馈数据验证 S_qa 与用户满意度低相关，可降低其权重
