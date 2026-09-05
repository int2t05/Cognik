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
| 前端 | 0 | 0 | 0 | 4 |
| **合计** | **0** | **0** | **4** | **8** |

---

> 产品技术路线图与未来方向见 [`ROADMAP.md`](ROADMAP.md) §10 V2.0。

