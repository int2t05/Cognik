# Kubernetes Pod 调试手册

本手册覆盖 Kubernetes 集群中 Pod 级别故障的定位与排查流程,适用于 1.24+ 版本。所有命令均基于 kubectl,需具备 cluster read 权限。

## CrashLoopBackOff

Pod 反复崩溃重启时,状态会进入 `CrashLoopBackOff`,kubelet 按指数退避策略延迟重启(10s → 20s → 40s → … 上限 5min)。

### 定位步骤

1. 查看当前容器日志:

```bash
kubectl logs <pod-name> -n <namespace>
```

2. 查看上一次崩溃时的日志(关键,多数崩溃原因在这里):

```bash
kubectl logs <pod-name> -n <namespace> --previous
```

`--previous` 也可写作 `--prev` 或 `-p`,它会读取容器上次终止前写入 stdout/stderr 的内容。如果容器在打印日志前就崩溃,这里会是空的——需要进一步看 Events。

3. 查看 Pod 详情,重点看 Events 段和容器的 Last State:

```bash
kubectl describe pod <pod-name> -n <namespace>
```

输出中关注:

- `Containers:` 下的 `Last State:` —— 包含 `Reason`(OOMKilled / Error / Completed)、`Exit Code`、`Started`、`Finished` 时间。
- `Events:` 段 —— 按时间倒序排列,显示 Pulling、Created、Started、Failed 等事件。

### 退出码含义

| Exit Code | 含义 | 常见原因 |
| --------- | ---- | -------- |
| 0 | 正常退出 | 主进程主动退出(短时任务 Pod 误配为 Deployment) |
| 1 | 应用错误 | 代码异常、配置文件语法错误、依赖初始化失败 |
| 2 | 命令误用 | shell 内置错误,参数不匹配或 shell builtin 误用 |
| 125 | 容器启动失败 | image 不存在、entrypoint 错误 |
| 126 | 命令不可执行 | 缺少可执行权限 |
| 127 | 命令未找到 | entrypoint 路径错误或 image 内缺少该二进制 |
| 137 | 被 SIGKILL | OOMKilled(超 memory limit)或人工 `docker kill` |
| 139 | 段错误 SIGSEGV | 二进制与运行时不匹配、内存越界 |
| 143 | 被 SIGTERM | 正常优雅退出,通常由 preStop hook 或 deletion 触发 |

Exit 137 需重点区分 OOMKilled 与外部 kill——看 `kubectl describe` 中 `Last State.Reason` 是否为 `OOMKilled`。

### Init 容器检查

Pod 若有 init 容器,它们必须按顺序全部成功才会启动主容器。任一 init 容器失败,Pod 会陷入 `Init:CrashLoopBackOff`。

```bash
# 列出 Pod 所有容器(含 init)
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.initContainers[*].name}{"\n"}{.spec.containers[*].name}'

# 查看指定 init 容器日志
kubectl logs <pod-name> -n <namespace> -c <init-container-name>
```

init 容器与主容器的区别:`initContainers` 字段定义,无 readiness/liveness probe,不参与服务发现,资源请求按 max(各 init 容器)与 sum(主容器)的较大值参与调度。

## OOMKilled

容器因超出 memory limit 被 cgroup OOM killer 杀死,`kubectl describe` 的 `Last State` 显示 `Reason: OOMKilled`、`Exit Code: 137`。

### 根因定位

1. 确认 OOMKilled:

```bash
kubectl describe pod <pod-name> -n <namespace> | grep -A5 "Last State"
```

2. 检查 resources requests/limits 是否与实际负载匹配:

```bash
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[*].resources}'
```

3. 对照节点实际内存压力——Pod 实际 RSS 可能远低于 limit,但节点本身内存不足触发系统级 OOM:

```bash
# 节点内存使用
kubectl describe node <node-name> | grep -A10 "Allocated resources"

# 实时监控容器内存(需 metrics-server)
kubectl top pod <pod-name> -n <namespace> --containers
```

### node allocatable 计算

节点可分配给 Pod 的内存 = Node Allocatable = Capacity - system-reserved - kube-reserved - eviction-hard。

```bash
kubectl describe node <node-name> | grep -E "Capacity|Allocatable|Memory"
```

若 Pod 的 memory request 与 limit 差距过大(如 request=256Mi, limit=2Gi),Pod 可能在突发时被 OOMKilled,但调度时只占 256Mi——这是 request/limit 设计的取舍。生产建议 Java/Node.js 等堆型应用将 request 与 limit 设为接近值,避免因节点超卖导致批量 OOM。

## Init 容器失败

init 容器用于初始化(下载配置、等待依赖、数据库迁移)。失败时 Pod 卡在 `Init:N/M` 或 `Init:Error`。

### 调试流程

```bash
# 1. 查看 Pod 状态,确认哪个 init 容器在跑/失败
kubectl get pod <pod-name> -n <namespace>

# 2. 查看 init 容器日志(必须指定 -c,否则默认查第一个主容器)
kubectl logs <pod-name> -n <namespace> -c <init-container-name>

# 3. 查看 init 容器的退出码(通过 describe)
kubectl describe pod <pod-name> -n <namespace> | grep -A15 "Init Containers"
```

### 依赖顺序问题

init 容器按 `spec.initContainers` 数组顺序执行,前一个成功(exit 0)才执行下一个。若 init 容器 A 依赖服务 B,而 B 尚未就绪,A 会失败重启。

常见错误模式:用 init 容器 `ping` 或 `curl` 探测依赖服务,但未处理 DNS 尚未解析的情况。正确做法是带重试与超时:

```yaml
initContainers:
  - name: wait-for-db
    image: busybox:1.36
    command:
      - sh
      - -c
      - "until nc -z postgres 5432; do echo waiting; sleep 2; done"
```

initContainers vs containers 的关键区别:init 容器没有 liveness/readiness probe,不接收流量,资源请求取所有 init 容器中的最大值参与调度(而非累加)。

## Service 网络

Service 无法访问或后端 Pod 不接收流量时,按 数据面 → 控制面 → DNS 三层排查。

### 检查 Endpoints

```bash
# 1. Service 是否有后端端点
kubectl get endpoints <service-name> -n <namespace>

# 2. EndpointSlice(K8s 1.21+ 默认)
kubectl get endpointslice -n <namespace> -l kubernetes.io/service-name=<service-name>
```

若 `ENDPOINTS` 列为空 `<none>`,说明 Service 的 selector 没匹配到任何 Pod。检查:

- Service selector 与 Pod label 是否一致(label key + value 完全匹配)。
- Pod 是否处于 Running 且 readiness probe 通过(未 Ready 的 Pod 不进 Endpoints)。

### kube-proxy 与数据面

```bash
# 进入节点查看 kube-proxy 模式(iptables 或 ipvs)
kubectl -n kube-system get pods -l k8s-app=kube-proxy -o wide
kubectl -n kube-system exec <kube-proxy-pod> -- iptables-save | grep <service-name>

# ipvs 模式
kubectl -n kube-system exec <kube-proxy-pod> -- ipvsadm -L -n
```

### CoreDNS

```bash
# CoreDNS Pod 状态
kubectl -n kube-system get pods -l k8s-app=kube-dns

# 测试 DNS 解析(从集群内 Pod)
kubectl exec -it <debug-pod> -- nslookup <service-name>.<namespace>.svc.cluster.local

# CoreDNS 日志
kubectl -n kube-system logs <coredns-pod>

# 检查 CoreDNS ConfigMap(自定义 hosts / rewrite 规则)
kubectl -n kube-system get configmap coredns -o yaml
```

常见问题:Pod 的 `/etc/resolv.conf` 的 `search` 域缺少 namespace、`ndots:5` 导致短域名多次查询、CoreDNS Pod 被调度到压力节点导致解析超时。

## PVC stuck Pending

PersistentVolumeClaim 停留在 Pending 状态,通常因 StorageClass 或绑定策略问题。

### 定位

```bash
kubectl describe pvc <pvc-name> -n <namespace>
```

Events 段会显示具体原因,常见:

- `storageclass.storage.k8s.io "xxx" not found` —— StorageClass 名称错误或不存在。
- `WaitForFirstConsumer` —— 延迟绑定,需 Pod 调度后才会分配 PV(检查 Pod 是否已创建且可调度)。
- `no volume plugin matched` —— StorageClass 的 provisioner 与节点上的 CSI 驱动不匹配。

### StorageClass 与绑定模式

```bash
# 列出所有 StorageClass
kubectl get storageclass

# 查看 volumeBindingMode
kubectl get storageclass <sc-name> -o jsonpath='{.volumeBindingMode}'
```

`volumeBindingMode` 两种取值:

- `Immediate`(默认):PVC 创建时立即尝试绑定 PV。
- `WaitForFirstConsumer`:延迟到 Pod 调度确定节点后才绑定,用于拓扑感知(如 local volume、跨可用区 EBS)。

若 PVC 使用 `WaitForFirstConsumer` 却卡在 Pending,检查是否有 Pod 引用该 PVC 且 Pod 处于可调度状态——没有消费者就不会绑定。

## 节点压力

节点资源不足时,kubelet 触发 eviction,驱逐 Pod 以保护节点。

### 查看节点状态

```bash
kubectl describe node <node-name>
```

关注 `Conditions` 段:

| Condition | True 含义 |
| --------- | --------- |
| `MemoryPressure` | 节点可用内存低于 eviction threshold |
| `DiskPressure` | 节点磁盘空间不足(节点根分区或镜像盘) |
| `PIDPressure` | 节点进程数过多 |
| `Ready` | 节点是否就绪(False/Unknown 时 Pod 会被驱逐) |

### eviction thresholds

kubelet 默认 eviction 硬阈值(可在 kubelet config `--eviction-hard` 调整):

- `memory.available < 100Mi`
- `nodefs.available < 10%`
- `nodefs.inodesFree < 5%`
- `imagefs.available < 15%`

触发 eviction 后,kubelet 按 QoS 等级驱逐:BestEffort → Burstable(超 request 部分) → Guaranteed。

```bash
# 节点资源使用
kubectl describe node <node-name> | grep -A20 "Allocated resources"

# 实时节点指标
kubectl top node <node-name>
```

## 调试工具

### kubectl exec

进入容器排查(需容器内有 shell):

```bash
kubectl exec -it <pod-name> -n <namespace> -- /bin/sh
kubectl exec -it <pod-name> -n <namespace> -c <container-name> -- /bin/bash
```

distroless 或 scratch 镜像无 shell,需用 ephemeral debug container。

### kubectl port-forward

本地访问 Pod 端口,绕过 Service/Ingress:

```bash
kubectl port-forward pod/<pod-name> 8080:8080 -n <namespace>
kubectl port-forward svc/<service-name> 5432:5432 -n <namespace>
```

### ephemeral debug containers

K8s 1.25+ 稳定特性,在运行中的 Pod 注入临时调试容器(无 shell 镜像救星):

```bash
kubectl debug -it <pod-name> -n <namespace> --image=busybox:1.36 --target=<container-name>
```

`--target` 让 debug 容器共享目标容器的 process namespace,可查看其进程与文件。

### kubectl top

需集群部署 metrics-server:

```bash
kubectl top pod -n <namespace> --sort-by=memory
kubectl top pod <pod-name> -n <namespace> --containers
kubectl top node
```

无 metrics-server 时,可直接进 Pod 读 cgroup:

```bash
kubectl exec <pod-name> -- cat /sys/fs/cgroup/memory/memory.usage_in_bytes
kubectl exec <pod-name> -- cat /proc/1/status | grep VmRSS
```
