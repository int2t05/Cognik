# ❓ FAQ & 故障排除

## 常见问题

### Q: 与直接用 ChatGPT 有什么区别？

OpsMind 不是 ChatGPT 套壳。核心差异：

1. **自建 RAG 引擎** — BM25 + 向量混合检索 + RRF 融合 + 重排序，全过程可审计、可调优
2. **知识资产化** — 问答→申告→处理→知识沉淀形成闭环，知识持续积累
3. **私有数据不出域** — 全部数据在自有 PostgreSQL + pgvector，支持完全离线部署
4. **运维场景专属** — 申告管理、知识审核、RBAC 权限等运维特定业务流程内置

### Q: 必须使用本地模型吗？

不必须。OpsMind 支持两种模式：

- **云端 API 模式：** 配置 OpenAI / DeepSeek / Moonshot 等 API，无需 GPU
- **本地模式：** 使用 llama.cpp 本地推理，数据完全不出域

两种模式可在后台管理 UI 中实时切换。

### Q: 支持哪些文档格式？

- PDF（通过 pdfcpu 解析）
- DOCX（通过 docx2md 转换为 Markdown 后处理）
- Markdown（`.md`）
- 纯文本（`.txt`）

上传后系统自动解析、分块、生成 embedding 并写入 pgvector。

### Q: pgvector 和专用向量数据库（Milvus/Qdrant）有什么区别？

pgvector 是 PostgreSQL 扩展，优势在于：

- 无需额外运维一个数据库服务
- 向量数据和业务数据在同一事务中操作
- HNSW 索引 + halfvec 半精度在 10 万级向量下查询 < 50ms

专用向量数据库在百万级以上向量时有性能优势，但 MVP 阶段 pgvector 完全够用。如需升级，只需替换 `VectorStore` 接口的实现即可。

### Q: 为什么要自建 RAG 而非用 LangChain？

- LangChain 抽象层次过高，调试困难，状态不可观测
- 引入 Python 依赖链，部署复杂
- 自建 Go 管道每个步骤是明确的函数调用，可独立测试、可逐步骤审计
- 降级逻辑可控——我们知道每一步失败时会发生什么

### Q: 支持非中文内容吗？

RAG 管道中的 BM25 检索使用 gse 中文分词器，对中文优化最好。英文内容可以通过向量检索正常召回，但 BM25 分词效果不如中文。如有英文需求，可替换为支持英文的分词器。

LLM 生成和 Embedding 取决于所选模型——推荐的多语言模型（bge-m3、Qwen3）都支持中英双语。

### Q: 如何备份数据？

```bash
# PostgreSQL（含 pgvector 向量数据）
docker compose exec postgres pg_dump -U opsmind opsmind > backup.sql

# MinIO（上传的文档和附件）
docker compose stop minio
tar -czf minio_backup.tar.gz /path/to/minio/data
docker compose start minio
```

建议设置 cron 定期执行备份。

### Q: 密码策略是什么？

- 8-32 位长度
- 必须包含大写字母、小写字母和数字
- 使用 bcrypt（cost=10）哈希存储
- 密码修改时校验新旧密码不能相同

## 故障排除

### 服务无法启动

```bash
# 1. 检查 Docker 状态
docker compose ps

# 2. 查看后端日志
docker compose logs opsmind-server

# 3. 检查端口占用
netstat -an | grep 8080
netstat -an | grep 3000
netstat -an | grep 5432
netstat -an | grep 9000
```

### 数据库连接失败

```
[error] failed to connect to database
```

- 检查 `OPSMIND_DATABASE_HOST` 配置（Docker 内应为 `postgres`，本地开发应为 `localhost`）
- 检查 `OPSMIND_DATABASE_PASSWORD` 是否与 `POSTGRES_PASSWORD` 一致
- 检查 PostgreSQL 容器是否健康：`docker compose ps postgres`

### LLM 调用失败（错误码 20001）

```
{"code": 20001, "message": "AI 服务不可用"}
```

- **云端 API 模式：** 检查 API Key 和 Base URL 是否正确
- **本地模式：** 检查 llama-cpp 容器是否运行、模型文件是否存在
- 确认网络可达：`curl http://llama-cpp:8080/v1/models`
- 检查 `OPSMIND_LLM_BASE_URL` 是否包含 `/v1` 后缀

### RAG 检索失败（错误码 20002）

```
{"code": 20002, "message": "RAG 服务不可用"}
```

- 检查 PostgreSQL pgvector 扩展：`SELECT * FROM pg_extension WHERE extname='vector';`
- 检查知识库是否有已发布（且处理完成）的文章
- 确认 Embedding 服务可达

### SSE 流式输出不工作

- 检查反向代理配置：`proxy_buffering off;` 必须设置
- 检查 `proxy_read_timeout` 是否足够长（建议 300s）
- 检查浏览器控制台是否有 EventSource 错误

### 文档上传处理失败

- 检查 MinIO 服务是否正常：http://localhost:9001
- 查看后端日志中的 `process_error` 字段
- 尝试重试：后台管理 → 文章详情 → 重试处理
- 确认 Embedding 服务可达且向量维度配置正确

### 密码修改后无法登录

- Access Token 有效期 2 小时，修改密码后旧 Token 仍有效直到过期
- 如需立即失效所有会话，可使用 Refresh Token 刷新或重新登录

## 获取帮助

- **Bug 报告：** [GitHub Issues](https://github.com/int2t05/OpsMind/issues)
- **功能建议：** [GitHub Issues](https://github.com/int2t05/OpsMind/issues) — 使用 Feature Request 模板
- **安全问题：** 请勿公开 Issue，直接联系维护者
- **文档：** [GitHub Wiki](Home) | [API 文档](https://github.com/int2t05/OpsMind/tree/main/docs/API)
