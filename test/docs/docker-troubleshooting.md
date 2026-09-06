# Docker 容器故障排查手册

> 生产环境 Docker 容器故障的系统性排查路径，覆盖日志、重启循环、OOM、磁盘、网络、Volume 与构建层。按现象定位章节，逐层下钻。

## 1. 日志与状态

容器异常的第一现场是日志与运行时状态，排查从这里开始。

### 1.1 容器日志

```bash
# 查看最近 100 行并持续跟踪，附带细节（环境变量、退出码等）
docker logs --details --tail 100 -f <container>

# 按时间过滤（排查特定时刻故障）
docker logs --since "2026-09-06T08:00:00" --until "2026-09-06T09:00:00" <container>

# 仅看 stderr
docker logs <container> 2>&1 1>/dev/null

# 导出到文件供分析
docker logs <container> > /tmp/container.log 2>&1
```

若日志为空，可能是：日志驱动非 `json-file`（如 `journald`/`syslog`），用 `docker inspect --format '{{.HostConfig.LogConfig.Type}}' <container>` 确认；或应用日志直接写文件未走 stdout。

### 1.2 容器详情

```bash
docker inspect <container>

# 提取关键字段
docker inspect --format 'ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}} Error={{.State.Error}} RestartCount={{.RestartCount}}' <container>

# 查看 IP、挂载、端口映射
docker inspect --format 'IP={{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}} Mounts={{range .Mounts}}{{.Source}}:{{.Destination}} {{end}}' <container>
```

### 1.3 资源占用

```bash
# 单次快照（适合脚本采集）
docker stats --no-stream

# 持续监控特定容器
docker stats <container>

# 输出格式
CONTAINER ID   NAME       CPU %   MEM USAGE / LIMIT   MEM %   NET I/O         BLOCK I/O
abc123         web        12.3%   256MiB / 512MiB      50%     1.2MB/3.4MB     5.1MB/0B
```

`MEM USAGE / LIMIT` 接近上限需警惕 OOM；`BLOCK I/O` 写量异常可能指向日志或临时文件未限速。

## 2. 重启循环

容器反复重启（`Restarting` 状态）是最常见的生产事故。

### 2.1 重启策略

```bash
# 查看重启策略与重启次数
docker inspect --format '{{.HostConfig.RestartPolicy.Name}} MaxRetry={{.HostConfig.RestartPolicy.MaximumRetryCount}} Count={{.RestartCount}}' <container>
```

策略说明：
- `no`：退出不重启。
- `always`：总是重启（含手动 stop，docker stop 后下次 daemon 启动会拉起）。
- `unless-stopped`：总是重启，但手动 stop 后不自动拉起（推荐）。
- `on-failure`：仅非零退出码重启，可设 `MaximumRetryCount`。

### 2.2 调试无法启动的容器

镜像内入口点崩溃时，容器秒退，日志来不及输出。覆写 entrypoint 进 shell 手动排查：

```bash
# 用原镜像启动一个交互式 shell，绕过崩溃的 entrypoint
docker run -it --entrypoint sh <image> -c "ls -la /app && ./start.sh"

# 或在原容器基础上覆写
docker run -it --entrypoint /bin/bash --name debug <image>
```

### 2.3 退出码含义

| 退出码 | 含义 | 常见原因 |
| --- | --- | --- |
| 0 | 正常退出 | 收到 SIGTERM 优雅关闭 |
| 1 | 应用错误 | 未捕获异常、配置错误、依赖不可达 |
| 125 | Docker 自身错误 | run 参数非法 |
| 126 | 命令不可执行 | 权限不足 |
| 127 | 命令未找到 | entrypoint 路径错误 |
| 137 | 被 SIGKILL | OOM killer 或 `docker kill`/`kill -9` |
| 139 | 段错误 SIGSEGV | 应用崩溃，多为内存访问越界/原生库 bug |
| 143 | 被 SIGTERM | `docker stop` 默认发送，15s 后升级为 SIGKILL |

137 是最需警惕的——结合 `OOMKilled` 字段区分是 OOM 还是外部强制 kill：

```bash
docker inspect --format '{{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}' <container>
# 137 + OOMKilled=true  → 内存不足被内核杀
# 137 + OOMKilled=false → 外部 kill 或 stop 超时
```

## 3. OOM Killer

### 3.1 确认 OOM

```bash
docker inspect --format '{{.State.OOMKilled}}' <container>
```

`OOMKilled=true` 表示内核 cgroup OOM killer 杀死了容器主进程。宿主机系统日志可进一步验证：

```bash
# Linux 宿主机
dmesg | grep -i "out of memory"
grep -i "killed process" /var/log/syslog
journalctl -k | grep -i oom
```

### 3.2 内存限制

```bash
# 运行时限制
docker run -d --name web \
    --memory=512m \
    --memory-swap=1g \
    --memory-reservation=256m \
    <image>

# 已运行容器无法直接改限制，用 update
docker update --memory=1g --memory-swap=2g <container>
```

- `--memory`：硬上限，超过触发 OOM。
- `--memory-swap`：内存+交换总量上限，设为 `-1` 表示不限 swap。
- `--memory-reservation`：软上限，宿主机内存紧张时回收到此值。

### 3.3 cgroup 视角

```bash
# cgroup v1（传统）
cat /sys/fs/cgroup/memory/docker/<container_id>/memory.usage_in_bytes
cat /sys/fs/cgroup/memory/docker/<container_id>/memory.max_usage_in_bytes
cat /sys/fs/cgroup/memory/docker/<container_id>/memory.limit_in_bytes
cat /sys/fs/cgroup/memory/docker/<container_id>/memory.oom_control

# cgroup v2（新内核）
cat /sys/fs/cgroup/system.slice/docker-<container_id>.scope/memory.current
cat /sys/fs/cgroup/system.slice/docker-<container_id>.scope/memory.max
cat /sys/fs/cgroup/system.slice/docker-<container_id>.scope/memory.events
```

`memory.events` 中的 `oom` 与 `oom_kill` 计数器可确认 OOM 发生次数，无需翻日志。

应用级排查：`java -XX:MaxRAMPercentage=75`、Node `--max-old-space-size=384`，务必让应用堆小于容器 `--memory` 留出非堆空间，否则 JVM/Node 看不到 cgroup 限制会自我膨胀触发 OOM。

## 4. 磁盘空间

Docker 默认存储于 `/var/lib/docker`，镜像层、容器可写层、Volume、日志都堆积于此。

### 4.1 空间占用

```bash
# 整体占用
docker system df

# 详细到每个镜像/容器/Volume 的可回收空间
docker system df -v
```

输出含 `RECLAIMABLE` 列，标识悬挂资源（无容器引用的镜像、停止容器的可写层、未使用的 Volume）。

### 4.2 清理

```bash
# 清理所有悬挂资源（停止容器 + 无用镜像 + 无用网络）
docker system prune

# 连同未使用的 Volume 一起清理（危险，确认无数据要保留）
docker system prune --volumes

# 精准清理
docker image prune -a          # 删除所有未被容器使用的镜像
docker container prune          # 删除所有停止的容器
docker volume prune             # 删除所有未挂载的 Volume
docker network prune            # 删除所有未使用的网络
```

### 4.3 overlay2 驱动清理

```bash
# 查看 /var/lib/docker 结构
du -sh /var/lib/docker/* | sort -h
du -sh /var/lib/docker/overlay2/* | sort -h | tail -20

# 查看容器可写层占用（哪个容器在疯狂写）
docker ps --format '{{.ID}} {{.Names}}' | while read id name; do
    size=$(docker inspect --format '{{.SizeRw}}' "$id")
    echo "$name $size"
done | sort -k2 -h
```

容器可写层异常增长通常是应用往容器内（非 Volume）写日志或临时文件所致——把日志目录挂成 Volume 或日志驱动改为 `json-file` + `max-size` 限制：

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
```

配在 `/etc/docker/daemon.json`，重启 docker 生效。

## 5. 网络问题

### 5.1 网络检查

```bash
# 查看网络配置与容器接入
docker network inspect <network>

# 列出所有网络
docker network ls

# 默认 bridge 网络下容器间不能用容器名互访
# user-defined bridge 才有内置 DNS
docker network create app-net
docker run -d --name web --network app-net <image>
docker run -it --network app-net busybox nslookup web   # 可解析
```

### 5.2 bridge vs host

- `bridge`（默认）：容器经 `docker0` 网桥 NAT 访问外部，端口需 `-p` 映射。隔离性好。
- `host`：容器直接用宿主机网络栈，无 NAT、无端口映射，性能最高但无隔离。`--network host` 仅 Linux 有效，macOS/Windows 不支持。
- `none`：无网络，仅 loopback，用于离线计算或自定义网络栈。

### 5.3 DNS 排查

```bash
# 默认 bridge 网络无内置 DNS，只能用 IP
# user-defined bridge 才有 DNS，容器名即主机名

# 进入容器测试 DNS
docker exec -it <container> sh -c "cat /etc/resolv.conf; nslookup db; ping -c1 db"

# 宿主机 DNS 透传问题：容器内 /etc/resolv.conf 的 nameserver
# user-defined 网络默认为 127.0.0.11（Docker 内嵌 DNS）
# 默认 bridge 网络复制宿主机 /etc/resolv.conf
```

容器解析外部域名失败常见原因：宿主机 `/etc/resolv.conf` 指向 `127.0.0.1` 的本地 DNS，容器内无法访问该地址。解决：在 `daemon.json` 配 `dns: ["8.8.8.8", "114.114.114.114"]`，或为 user-defined 网络单独设 `docker network create --dns 8.8.8.8 app-net`。

### 5.4 --network 配置与多网络

```bash
docker run -d \
    --network app-net \
    --network-alias web.internal \
    --network db-net \
    --network-alias web.db \
    <image>
```

容器可接入多个网络，每个网络内可有独立别名。排查跨网络不通时，确认双方在同一 user-defined 网络。

## 6. Volume 问题

### 6.1 Volume vs Bind Mount

| 类型 | 命令 | 生命周期 | 适用场景 |
| --- | --- | --- | --- |
| Volume | `-v myvol:/data` | Docker 管理，独立于容器 | 持久数据、跨容器共享 |
| Bind Mount | `-v /host/path:/data` | 依赖宿主机路径 | 开发挂源码、配置注入 |
| tmpfs | `--tmpfs /cache` | 内存，容器停止即失 | 敏感数据、临时缓存 |

### 6.2 检查 Volume

```bash
docker volume inspect <volume>

# 查看挂载点
docker volume inspect --format '{{.Mountpoint}}' <volume>
# 默认位于 /var/lib/docker/volumes/<volume>/_data

# 列出所有 Volume 与占用
docker volume ls
docker system df -v | grep -A50 "VOLUME NAME"
```

### 6.3 Bind Mount 权限问题

最常见故障：宿主机目录属主与容器内运行用户 UID 不一致，导致 Permission denied。

```bash
# 容器内查看运行用户 UID
docker exec <container> id

# 宿主机查看挂载目录属主
ls -ln /host/path
```

解决方案：
1. 容器以宿主机目录属主 UID 运行：`docker run -u $(id -u):$(id -g)`。
2. 修改宿主机目录属主匹配容器用户：`chown -R 1000:1000 /host/path`。
3. 应用启动脚本中 `chown` 后再启动（需 root 运行，降权用 `gosu`/`su-exec`）。

### 6.4 SELinux 标签

在启用 SELinux 的发行版（RHEL/CentOS/Fedora）上，bind mount 需加标签后缀避免 `permission denied`：

```bash
# :z — 共享标签，多个容器可读写该挂载
docker run -v /host/path:/data:z <image>

# :Z — 私有标签，仅当前容器可访问
docker run -v /host/path:/data:Z <image>

# :ro — 只读
docker run -v /host/config:/etc/app:ro,z <image>
```

`avc: denied` 出现在 `/var/log/audit/audit.log` 即 SELinux 拦截。开发环境可临时 `setenforce 0` 验证是否 SELinux 问题，但生产切勿关闭。

### 6.5 常见症状

```bash
# 容器内看到空目录但宿主机有文件 → 挂载路径写错或 Volume 名冲突
docker inspect --format '{{range .Mounts}}{{.Type}} {{.Source}} -> {{.Destination}}{{"\n"}}{{end}}' <container>

# Volume 被多个容器共享导致写冲突 → 检查引用
docker ps -a --filter volume=<volume> --format '{{.Names}} {{.Status}}'

# 删除仍被引用的 Volume 报错
docker volume rm <volume>
# Error response from daemon: unable to remove volume: volume is in use
# 先停掉引用容器，或用 -f（不推荐，会留下孤儿挂载）
```

## 7. 构建优化

### 7.1 Multi-stage Build

最终镜像只含运行时所需，构建工具与中间产物不进生产镜像：

```dockerfile
# 阶段 1：构建
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app -ldflags="-s -w" ./cmd

# 阶段 2：运行时
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

distroless / scratch / alpine 三选一，按依赖体积权衡。Go 静态二进制用 distroless，Node/Python 用 alpine。

### 7.2 .dockerignore

减少构建上下文传输量，避免敏感文件与无关目录进入镜像：

```
.git
node_modules
*.md
.env
.env.*
coverage/
dist/
test/
```

`COPY . .` 不带 ignore 会把整个目录上传给 daemon，慢且可能把 `.env` 打进镜像层（即使后续 COPY 不取，仍留在中间层历史里）。

### 7.3 Layer Caching

把变化频率低的层放前面：

```dockerfile
# 差：源码一变就重装依赖
COPY . .
RUN npm install

# 好：依赖层先建，源码变化不触发重装
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY . .
```

查看镜像各层大小定位臃肿层：

```bash
docker history --no-trunc <image>
```

### 7.4 BuildKit cache mounts

构建期缓存依赖目录，跨构建复用，不进最终镜像：

```dockerfile
# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /app ./cmd

FROM node:20-alpine
RUN --mount=type=cache,target=/root/.npm \
    npm ci
```

```bash
# 启用 BuildKit
export DOCKER_BUILDKIT=1
docker build -t app .

# 或 daemon.json 永久启用
# { "features": { "buildkit": true } }
```

`type=cache` 对 Go module、npm/pnpm、apt 缓存效果显著，二次构建可从分钟级降到秒级。注意缓存是 per-host 的，CI 不同 runner 不共享（需配合 `type=cache` 的远程后端或 `--cache-from/--cache-to`）。
