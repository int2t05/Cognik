# PostgreSQL 性能调优与优化指南

> 面向生产环境的 PostgreSQL 调优参考，覆盖内存、WAL、Autovacuum、查询分析、索引、连接池与存储层。所有参数均需结合负载与硬件实测，本文给出的是经验起点。

## 1. 内存参数

PostgreSQL 的内存模型是「多进程 + 共享内存 + 每连接私有内存」，参数调整的核心是平衡共享池与并发会话的私有开销。

### 1.1 shared_buffers

共享缓冲池，所有后端进程共用的数据页缓存。经验值为物理内存的 25%，超过 40% 收益递减（操作系统页缓存同样会缓存数据页，二者重复）。

```sql
-- 查看当前值
SHOW shared_buffers;

-- 16GB 内存机器的典型配置（postgresql.conf）
shared_buffers = 4GB
```

判断是否够用：监控 `pg_stat_database` 的 `blks_hit` 与 `blks_read`，命中率应稳定在 99% 以上：

```sql
SELECT datname,
       blks_hit,
       blks_read,
       round(blks_hit::numeric / nullif(blks_hit + blks_read, 0) * 100, 2) AS hit_ratio
FROM pg_stat_database
WHERE blks_hit + blks_read > 0;
```

命中率低于 95% 且磁盘读持续偏高时，优先扩 `shared_buffers`，而非盲目加索引。

### 1.2 work_mem

每个排序、哈希、归并操作的私有内存。注意是「每个操作」而非「每个会话」——一条含 3 个 sort 的 SQL 最多申请 `3 * work_mem`。高并发下设置过大易触发 OOM。

```sql
-- 全局起点
work_mem = 16MB

-- 针对特定会话/查询临时调大，避免全局影响
SET LOCAL work_mem = '256MB';
SELECT ... ORDER BY large_col;
RESET work_mem;
```

排错信号：`EXPLAIN ANALYZE` 中出现 `Sort Method: external merge Disk`，说明 work_mem 不足，排序溢写到磁盘。

### 1.3 maintenance_work_mem

VACUUM、CREATE INDEX、REINDEX、ALTER TABLE 等维护操作的内存。与 work_mem 不同，它通常只有少量并发维护任务，可设得更大以加速索引构建：

```sql
maintenance_work_mem = 512MB   -- 常规
maintenance_work_mem = 2GB     -- 大表建索引前临时调大
SET maintenance_work_mem = '2GB';
CREATE INDEX CONCURRENTLY idx_orders_created ON orders(created_at);
```

### 1.4 effective_cache_size

这是给查询规划器的「提示」，告诉它操作系统页缓存大约有多少可用，**不会真正分配内存**。设低会导致规划器误以为内存紧张，倾向走全表扫描而非索引扫描。设为物理内存的 50%~75%：

```sql
effective_cache_size = 12GB   -- 16GB 机器
```

常见误区：把 `effective_cache_size` 当成 `shared_buffers` 来调——前者只影响执行计划，后者才是真实分配。

## 2. WAL 配置

Write-Ahead Log 决定写入吞吐与崩溃恢复时间。

### 2.1 核心参数

```conf
wal_buffers = 16MB                   # 共享内存中 WAL 缓冲，写密集型可提到 64MB
checkpoint_completion_target = 0.9   # 让 checkpoint 在间隔的 90% 内完成，平滑 IO
max_wal_size = 4GB                   # checkpoint 之间允许的最大 WAL 量
min_wal_size = 1GB                   # WAL 保留下限，避免频繁 checkpoint 回收
checkpoint_timeout = 15min           # 两次 checkpoint 最大间隔（默认 5min 偏激进）
```

`max_wal_size` 过小会导致 checkpoint 频繁触发，写 IO 抖动；过大则崩溃恢复时间长。监控 `pg_stat_bgwriter`：

```sql
SELECT checkpoints_timed, checkpoints_req,
       round(checkpoints_req::numeric / nullif(checkpoints_timed + checkpoints_req, 0) * 100, 2) AS req_pct,
       buffers_checkpoint, buffers_clean
FROM pg_stat_bgwriter;
```

`checkpoints_req`（因 WAL 量超限被迫触发）占比高，说明 `max_wal_size` 偏小。

### 2.2 恢复目标

```sql
wal_level = replica            # 主从复制；仅备份用 minimal
archive_mode = on              # 开启归档
archive_command = 'test ! -f /archive/%f && cp %p /archive/%f'
```

## 3. Autovacuum

Postgres 的 MVCC 采用「多版本 + 标记删除」，死元组靠 VACUUM 回收。Autovacuum 是后台自动回收机制，关闭它是生产事故的常见根因。

### 3.1 关键阈值

```conf
autovacuum = on                       # 必开（默认开），切勿关闭
autovacuum_vacuum_scale_factor = 0.2  # 死元组达 20% 触发 VACUUM
autovacuum_analyze_scale_factor = 0.1 # 变更达 10% 触发 ANALYZE 统计刷新
autovacuum_vacuum_threshold = 50      # 小表保底阈值（死元组绝对数）
autovacuum_naptime = 1min             # 轮询间隔
autovacuum_max_workers = 3            # 并发 worker 数
```

写密集大表可单独调低触发阈值，防止死元组堆积：

```sql
ALTER TABLE events SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);
```

### 3.2 何时手动 VACUUM ANALYZE

Autovacuum 是「尽力而为」，以下场景需手动介入：

- **大批量 DELETE/UPDATE 后立即查询**：统计信息未刷新，执行计划可能劣化，手动 `VACUUM ANALYZE big_table;`。
- **长事务阻塞了 autovacuum**：`SELECT pid, age(datfrozenxid) FROM pg_stat_activity WHERE xact_start < now() - interval '1 hour';` 长事务持有旧快照，autovacuum 无法回收其后的死元组。
- **膨胀严重**：`SELECT schemaname, relname, n_live_tup, n_dead_tup, last_autovacuum FROM pg_stat_user_tables WHERE n_dead_tup > 10000 ORDER BY n_dead_tup DESC;` 死元组持续高位需手动 `VACUUM (VERBOSE, ANALYZE)`。

```sql
-- 仅刷新统计信息（轻量，不锁表）
ANALYZE orders;

-- 回收死元组 + 刷新统计
VACUUM ANALYZE orders;

-- 锁表重建（修复严重膨胀，会锁表）
VACUUM FULL orders;
```

## 4. 查询分析

### 4.1 EXPLAIN

```sql
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, SETTINGS)
SELECT o.id, c.name
FROM orders o JOIN customers c ON o.customer_id = c.id
WHERE o.created_at > '2025-01-01'
ORDER BY o.total DESC
LIMIT 50;
```

读执行计划要点：
- `Seq Scan` + 大估算行数 → 缺索引或统计过期。
- `actual rows` 与 `estimated rows` 差一个数量级 → `ANALYZE` 该表。
- `Buffers: shared hit=12 read=340` → `read` 多说明未命中缓存。
- `Sort Method: external merge Disk` → work_mem 不足。
- `Nested Loop` 配合内层大表扫描 → 常是缺失连接键索引。

### 4.2 pg_stat_statements

```sql
-- 启用（需在 shared_preload_libraries 配置后重启）
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- 重启后创建扩展
CREATE EXTENSION pg_stat_statements;

-- 慢 SQL Top 10（按总耗时）
SELECT queryid,
       calls,
       round(total_exec_time::numeric, 2) AS total_ms,
       round(mean_exec_time::numeric, 2) AS mean_ms,
       rows,
       shared_blks_hit + shared_blks_read AS blks
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 10;

-- 高 IO 的查询
SELECT queryid, calls, shared_blks_read, shared_blks_hit
FROM pg_stat_statements
ORDER BY shared_blks_read DESC
LIMIT 10;
```

定期 `SELECT pg_stat_statements_reset();` 清空统计，聚焦新窗口。

### 4.3 pg_stat_user_indexes（未使用索引）

```sql
SELECT schemaname, relname, indexrelname,
       idx_scan AS scans,
       idx_tup_read, idx_tup_fetch,
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
WHERE idx_scan = 0
  AND indexrelname NOT LIKE '%_pkey'
ORDER BY pg_relation_size(indexrelid) DESC;
```

`idx_scan = 0` 且体积大的索引是候选删除对象。删除前确认不是仅在月结/季度报表中才触发的索引——先观察一个完整业务周期。

## 5. 索引策略

### 5.1 索引类型选型

| 类型 | 适用场景 | 示例 |
| --- | --- | --- |
| B-tree | 等值、范围、排序、唯一 | `CREATE INDEX ON orders(created_at);` |
| GIN | 全文检索、JSONB、数组包含 | `CREATE INDEX ON docs USING gin(to_tsvector('simple', body));` |
| GiST | 几何、范围、最近邻、排除约束 | `CREATE INDEX ON regions USING gist(geom);` |
| BRIN | 有序大表（时间序列） | `CREATE INDEX ON logs USING brin(ts);` |
| Hash | 仅等值（PG 10+ WAL 支持） | `CREATE INDEX ON users USING hash(token);` |

### 5.2 部分索引

只为高频查询子集建索引，体积小、命中快：

```sql
CREATE INDEX idx_orders_unshipped ON orders(customer_id)
WHERE status = 'pending';
```

### 5.3 覆盖索引

把高频返回列纳入索引，实现 Index-Only Scan：

```sql
CREATE INDEX idx_orders_cust_total
ON orders(customer_id)
INCLUDE (total, status);
```

执行计划出现 `Index Only Scan` 即命中。注意 `INCLUDE` 列不参与索引排序。

### 5.4 表达式索引

```sql
CREATE INDEX idx_users_lower_email ON users(lower(email));
SELECT * FROM users WHERE lower(email) = 'a@b.com';  -- 命中
```

## 6. 连接池

### 6.1 PgBouncer

Postgres 每个连接对应一个后端进程，连接数高时内存与上下文切换开销显著。PgBouncer 在应用与数据库间复用连接。

```ini
[databases]
mydb = host=127.0.0.1 port=5432 dbname=mydb

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
pool_mode = transaction   ; 事务级复用：事务结束归还连接
max_client_conn = 1000    ; 应用侧最大连接
default_pool_size = 25    ; 每数据库/用户的后端连接数
reserve_pool_size = 5
server_idle_timeout = 600 ; 空闲后端 10 分钟回收
```

`pool_mode = transaction` 是通用推荐；`session` 模式复用率低；`statement` 模式禁用事务跨语句。

### 6.2 max_connections vs pool size

```sql
SHOW max_connections;  -- 默认 100
```

经验公式：`PgBouncer pool_size × 数据库数 + 预留(10) ≤ max_connections`。应用并发数可远超 pool size，由 PgBouncer 排队复用。

### 6.3 空闲连接清理

```sql
SELECT state, count(*), max(now() - state_change) AS max_idle
FROM pg_stat_activity
WHERE state = 'idle'
GROUP BY state;
```

空闲连接长期不释放会占用后端进程与 `work_mem` 预留。`idle_in_transaction` 尤其危险——它持有锁与快照，阻塞 autovacuum。配置 `idle_in_transaction_session_timeout`：

```sql
idle_in_transaction_session_timeout = '10min'
```

## 7. SSD 优化

机械盘与 SSD 的随机读代价差异巨大，规划器成本常数需调整：

```sql
-- SSD 调整
random_page_cost = 1.1        -- 默认 4.0（适合机械盘）
effective_io_concurrency = 200 -- SSD 可并发 IO，机械盘保持 1
default_statistics_target = 100 -- 默认 100，分布不均列可单独调高
```

针对分布倾斜的列拉高统计精度（代价是 ANALYZE 变慢）：

```sql
ALTER TABLE orders ALTER COLUMN status SET STATISTICS 500;
ANALYZE orders(status);
```

`random_page_cost` 设过低会让规划器在非选择性查询上偏好索引扫描反而更慢；1.1 是 SSD 的公认起点，切勿设为 1.0 或更低。
