# 🏠 OpsMind — 私有部署的 AI 运维数字员工

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Next.js-16-000000?logo=nextdotjs" alt="Next.js">
  <img src="https://img.shields.io/badge/PostgreSQL-pgvector-4169E1?logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/Docker-blue?logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

**OpsMind** 是一个面向企业运维场景的 AI 数字员工系统。它通过本地化大模型、私有知识库、运维申告门户和后台运营管理，辅助或替代人工完成常见咨询、自助处理、申告记录、人工流转和知识库更新。

> 不是另一个 ChatGPT 套壳。是一个从检索管道到业务流程都自建的运维数字员工系统。

## 为什么选择 OpsMind？

企业运维团队每天面对大量重复性咨询——密码重置、权限申请、系统报障、操作指引。这些工作消耗运维人员 40% 以上的时间，却无法沉淀为可复用的知识。

OpsMind 的核心理念：

1. **自建 RAG 引擎** — BM25 + 向量混合检索 + RRF 融合 + 重排序，全过程可控、可审计
2. **知识资产化** — 每次问答、每条申告处理记录都可转化为知识库文章，经审核后发布
3. **私有数据不出域** — 全部数据存储在自有 PostgreSQL + pgvector，支持本地 llama.cpp 或任意 OpenAI-compatible API
4. **全流程闭环** — 问答 → 申告 → 处理 → 知识沉淀，形成运维知识自循环

## 核心能力一览

| 能力 | 说明 |
|------|------|
| 🧠 **RAG 增强智能问答** | 自建管道：查询改写 → 多路检索 → BM25+向量混合 → RRF 融合 → 重排序 → LLM 生成，SSE 流式输出 |
| 📚 **多格式文档上传** | 支持 PDF / DOCX / MD / TXT，异步解析 → 分块 → embedding → pgvector |
| 🎫 **运维申告全流程** | 状态机管理（待处理 → 处理中 → 需补充 → 已解决 / 已关闭），自动关闭 |
| 📝 **统一知识文章模型** | 手动创建 + 文档上传双通道，审核 → 发布 → pgvector 向量写入 |
| 🔐 **RBAC 权限管控** | 4 个预设角色，JWT 双令牌认证，菜单按权限动态渲染 |
| 📊 **数据看板与审计** | 实时统计卡片 + 趋势图 + 完整操作审计日志 |
| ⚙️ **LLM 配置热替换** | 支持 llama.cpp 和 OpenAI-compatible API 双模式，配置修改即时生效 |
| 🐳 **一键部署** | Docker Compose 编排 4 个必须服务，可选 llama.cpp profile |

## 快速导航

| 想要... | 看这里 |
|---------|--------|
| 快速跑起来 | [🚀 快速开始](Quick-Start) |
| 理解系统设计 | [🏗️ 架构概览](Architecture) |
| 深入了解 RAG | [🧠 RAG 引擎](RAG-Engine) |
| 查看全部功能 | [✨ 功能详解](Features) |
| 对接 API | [📡 API 参考](API-Reference) |
| 调整配置 | [⚙️ 配置指南](Configuration) |
| 部署到生产 | [🐳 部署指南](Deployment) |
| 参与开发 | [💻 开发指南](Development) |
| 了解数据模型 | [🗄️ 数据库设计](Database) |
| 遇到问题 | [❓ FAQ](FAQ) |

## 技术栈速览

| 层级 | 技术选型 |
|------|----------|
| 后端语言 | Go 1.26+ |
| HTTP 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL + pgvector（HNSW 索引 + halfvec 半精度） |
| 中文分词 | gse（纯 Go，零 CGO 依赖） |
| 对象存储 | MinIO（S3-compatible） |
| 认证 | JWT（access + refresh 双令牌） + bcrypt |
| 前端框架 | Next.js 16 / React / TypeScript |
| UI 系统 | Radix UI + Apple Design System（浅色/暗色双主题） |
| 状态管理 | React Context + SWR |
| 部署 | Docker Compose |

## 许可证

[MIT License](https://github.com/int2t05/OpsMind/blob/main/LICENSE)
