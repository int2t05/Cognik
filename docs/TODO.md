# Cognos 改进清单

> 优先级：🔴 生产隐患 / 🟠 功能缺陷 / 🟡 架构债务 / 🟢 优化建议

---

# 后端

## 1. RAG 管道

- 🟡 BM25 索引无增量更新，每次刷新全量重建——需算法重构
- 🟡 文档处理器无阶段内重试机制，embedding API 瞬时失败直接中止——需架构变更
- 🟢 语义去重阈值 0.92 硬编码，不可配置——设计权衡
- 🟢 CreateAndPublish 无速率限制——Agent 短时间内大量 create 可能灌入低质量文章

## 2. 知识库管理

- 🟡 DOCX 解析仅读取 `word/document.xml`，不处理 `word/document2.xml` 分割文档
- 🟡 PDF/DOCX 解析前全量读入内存（`io.ReadAll`），大文件 OOM 风险
- 🟢 UpdateAndRepublish 并发写同一文章无 CAS 保护——增量 reindex 可能竞态

## 3. Agent

- 🟡 Agent 任务无持久化——崩溃后 ReAct 循环不可恢复（SQLite 仅存事件流）
- 🟢 AutoDream 复盘无触发条件配置——固定双门（24h+5 sessions），不可调

---

# 生产化与可靠性

> 来源：v1.7 发布后全量项目评价。对应版本规划见 [`ROADMAP.md`](ROADMAP.md) §10.5–10.6。

- 🔴 **Agent 自动写入知识库无审核门控** — `CreateAndPublish` 默认直发进 RAG，AI 幻觉可直接污染 KB；需配置开关（auto_publish=false 时落 draft/ 走人工审核）+ 写入前质量门（置信度/引用完整性校验）
- 🟠 **无监控告警** — RAG 管道、Agent 循环、embedding 延迟无指标采集与告警；接 Prometheus exporter + 告警规则（检索失败率、embedding 超时、Agent 循环异常）
- 🟠 **审计覆盖不全** — 仅有用户/知识/工单管理操作审计日志，缺 Agent 决策轨迹完整回放与异常聚合；需记录每轮 ReAct 的 tool_call/tool_result/检索召回，支持按 thread 回放
- 🟡 **RAG 管道无成本控制** — 多轮工具调用 + rerank 无 token/调用预算上限；需 budget gate（单会话 token 上限、rerank 按置信度阈值触发而非每轮强制）
- 🟡 **缺压测基线与容量文档** — 无 `docs/CAPACITY.md`；需 k6 压测标定并发上限、RAG p95 延迟、单节点承载 KB 规模

---

# 前端

## 1. 智能问答

（暂无未完成项）

## 2. 表单与交互

- 🟢 表单缺 required 标记——非阻塞
- 🟢 用户搜索无结果提示——非阻塞

## 3. 组件架构

- 🟢 StatusBadge 领域状态映射硬编码在组件内——`statusText` prop 已提供逃生舱

## 4. 工单展示

- 🟢 系统复核工单（source=3）在前端列表无视觉区分——可加 badge 标记

---

## 统计

| | 🔴 P0 | 🟠 P1 | 🟡 P2 | 🟢 P3 |
|---|---|---|---|---|
| 后端 | 0 | 0 | 4 | 4 |
| 生产化 | 1 | 2 | 2 | 0 |
| 前端 | 0 | 0 | 0 | 4 |
| **合计** | **1** | **2** | **6** | **8** |

---

> 产品技术路线图与未来方向见 [`ROADMAP.md`](ROADMAP.md) §10 V2.0。
