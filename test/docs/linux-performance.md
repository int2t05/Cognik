# Linux 性能分析与诊断工具

本文档覆盖 Linux 系统性能分析的六大维度:CPU、内存、磁盘 I/O、网络、进程追踪、内核。所有工具基于 procfs 和 sysfs,适用于内核 4.x / 5.x / 6.x。

## CPU

### load average 含义

```bash
uptime
# 输出示例: load average: 1.50, 1.20, 0.80
```

三个数字分别为 1/5/15 分钟内系统的平均运行队列长度(正在运行 + 可中断睡眠等待 CPU 的进程数)。含义:

- 等于 CPU 核数 —— 满载,无空闲。
- 小于核数 —— 有空闲。
- 大于核数 —— 有进程在排队等 CPU。

注意:load average 包含 `D` 状态(不可中断睡眠,通常是 I/O 等待)的进程。高 load 但 CPU idle 高,通常是 I/O 瓶颈而非 CPU 瓶颈。

```bash
# 查看核数
nproc
cat /proc/cpuinfo | grep -c processor

# 实时 load
cat /proc/loadavg
# 示例输出: 1.50 1.20 0.80 3/1024 12345
# 前三个是 load average,第四个"分子/分母"是运行进程数/总进程数,最后是最近创建的 PID
```

### top / htop

```bash
top -d 1          # 每 1 秒刷新
top -p <pid>      # 监控指定进程
top -H            # 显示线程(默认显示进程)
```

top 关键列:

- `us` —— 用户态 CPU(应用代码)。
- `sy` —— 内核态 CPU(系统调用、上下文切换)。
- `wa` —— I/O wait(等磁盘/网络,高说明 I/O 瓶颈)。
- `si` —— 软中断(网络收包等)。
- `ni` —— nice 值调整过的进程占用。

`sy` 持续高于 30% 通常是上下文切换过多或系统调用频繁。

### perf

```bash
# 系统级 CPU 热点(实时,类似 top)
sudo perf top

# 采样 10 秒,记录到文件
sudo perf record -F 99 -p <pid> -g -- sleep 10
sudo perf report

# 查看指定进程的调用图
sudo perf record -F 99 -p <pid> -g -- sleep 10
sudo perf report --no-children
```

`-F 99` 采样频率 99Hz(避开 100Hz 与某些定时器共振)。`-g` 记录调用栈。

### flamegraph 生成

需 Brendan Gregg 的 FlameGraph 工具集:

```bash
# 1. perf 采样
sudo perf record -F 99 -p <pid> -g -- sleep 30
sudo perf script > out.perf

# 2. 折叠调用栈
git clone https://github.com/brendangregg/FlameGraph.git
./FlameGraph/stackcollapse-perf.pl out.perf > out.folded

# 3. 生成火焰图 SVG
./FlameGraph/flamegraph.pl out.folded > flame.svg
```

火焰图横向宽度 = 函数采样占比,纵向 = 调用栈深度。宽的"平台"是优化重点。

### pidstat

```bash
pidstat 1              # 每秒输出所有进程 CPU 使用
pidstat -p <pid> 1     # 指定进程
pidstat -d 1           # 磁盘 I/O 维度
pidstat -r 1           # 内存维度
pidstat -w 1           # 上下文切换
```

`pidstat` 来自 sysstat 包,比 top 更适合持续记录与定位单进程。

## 内存

### free -h

```bash
free -h
#               total   used   free   shared  buff/cache  available
# Mem:          16Gi    4Gi   2Gi    256Mi       10Gi        11Gi
# Swap:         2Gi     0B     2Gi
```

关键列:

- `buff/cache` —— 内核用作缓冲区和页缓存的内存,可回收。
- `available` —— 应用实际可用的内存(= free + 可回收的 buff/cache)。

判断内存压力看 `available`,不是 `free`。`free` 低是正常的——内核用空闲内存做缓存提升 I/O 性能。

### /proc/meminfo

```bash
cat /proc/meminfo
```

关键字段区分:

- `Cached` —— 页缓存(文件内容),回收成本低。
- `Buffers` —— 块设备 I/O 缓冲(元数据等)。
- `SReclaimable` —— 可回收的 slab(dentry/inode 缓存)。
- `SUnreclaim` —— 不可回收 slab,持续增长可能内核泄漏。
- `Slab` —— SReclaimable + SUnreclaim。

### vmstat

```bash
vmstat 1
# procs ----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
#  r  b  swpd  free  buff  cache   si   so    bi    bo   in   cs us sy id wa st
#  2  0     0 2Gi  1Gi   9Gi       0    0    10    20 1000 2000 10  5 80  5  0
```

关键列:

- `si`/`so`(swap in/out)—— 持续大于 0 说明内存不足在换页,性能会严重下降。
- `r` —— 运行队列长度,持续大于 CPU 核数说明 CPU 瓶颈。
- `b` —— D 状态(不可中断睡眠)进程数,通常等 I/O。
- `in` —— 中断数/秒。
- `cs` —— 上下文切换数/秒,异常高可能锁竞争或进程数过多。

### OOM killer

```bash
# 查看内核 OOM 记录
dmesg | grep -i "killed process"
# 或
sudo grep -i "out of memory" /var/log/messages
sudo grep -i "oom-killer" /var/log/syslog
journalctl -k | grep -i oom
```

OOM killer 日志包含:触发进程、被杀进程、各进程的 oom_score(基于内存占用 + 进程优先级)。

### 进程级内存

```bash
cat /proc/<pid>/status | grep -E "VmRSS|VmSize|VmHWM|VmPeak"
# VmRSS   —— 实际驻留物理内存
# VmSize  —— 虚拟内存大小
# VmHWM   —— 历史峰值 RSS
# VmPeak  —— 历史峰值虚拟内存
```

## 磁盘 I/O

### iostat

```bash
iostat -xz 1
# Linux ... (all CPU stats)
# Device  rrqm/s  wrqm/s  r/s  w/s  rkB/s  wkB/s  avgrq-sz  avgqu-sz  await  %util
# sda       0.0     1.0  2.0 5.0   32.0  64.0     13.7       0.5    71.4   50.0
```

关键列:

- `await` —— I/O 平均响应时间(ms),包含队列等待 + 服务时间。机械盘 > 20ms、SSD > 5ms 异常。
- `svctm`(部分版本已废弃)—— 平均服务时间。`await - svctm` ≈ 队列等待时间。
- `%util` —— 设备利用率,接近 100% 说明饱和。注意:对多队列设备(NVMe),%util 单线程顺序读也会接近 100% 但设备未饱和,需结合队列深度判断。
- `avgqu-sz` —— 平均 I/O 队列长度,持续 > 1 说明排队。
- `r/s`、`w/s` —— 每秒读/写 IOPS。

`-x` 扩展统计,`-z` 省略全零设备。

### iotop

```bash
sudo iotop -o          # 只显示有 I/O 的进程
sudo iotop -oP -d 1    # 每秒刷新,显示进程级
```

iotop 需要 root,基于内核 I/O accounting。无 iotop 时可用 pidstat 替代:`pidstat -d 1`。

### fio benchmark

```bash
# 顺序读写测试(吞吐)
fio --name=seqread --ioengine=libaio --rw=read --bs=1M --size=1G --numjobs=1 --runtime=60 --time_based

# 随机读写测试(IOPS)
fio --name=randrw --ioengine=libaio --rw=randrw --bs=4k --size=1G --numjobs=4 --runtime=60 --time_based --group_reporting
```

`bs=4k` 测 IOPS,`bs=1M` 测吞吐。`numjobs` 模拟并发。对照 SSD 规格书判断设备是否达标。

### /proc/diskstats

```bash
cat /proc/diskstats
# 字段含义见内核文档 Documentation/admin-guide/iostats.rst
```

字段 9(已完成的写请求数)和字段 10(写耗时)可手算 IOPS 与延迟。

## 网络

### ss

```bash
ss -tlnp              # 监听端口 + 进程
ss -tan               # 所有 TCP 连接状态
ss -s                 # 连接数统计
ss -i                 # TCP 内部信息(rtt、cwnd)
```

`ss` 已取代 netstat,性能更好。`-t` TCP,`-u` UDP,`-l` listen,`-n` 不解析端口名,`-p` 显示进程。

### tcpdump

```bash
# 抓取指定网卡流量
sudo tcpdump -i eth0 -nn

# 抓取指定端口
sudo tcpdump -i eth0 -nn port 80

# 抓取并写入文件(Wireshark 分析)
sudo tcpdump -i eth0 -w capture.pcap -s 0

# 抓取双向完整握手
sudo tcpdump -i eth0 -nn host 10.0.0.1 and port 443
```

`-nn` 禁止 DNS 与端口名解析,减少抓包开销。`-s 0` 抓取完整包(默认 96 字节)。

### nstat

```bash
nstat -z              # TCP/IP 协议栈计数器(零值不显示)
nstat -a              # 所有计数器
nstat -s              # 仅显示非零
```

nstat 读取 `/proc/net/netstat` 与 `/proc/net/snmp`,比 netstat -s 更原始。关注 `TcpExtListenOverflows`(listen backlog 满)、`TcpExtListenDrops`(连接被丢弃)。

### sar

```bash
sar -n DEV 1          # 网卡流量
sar -n TCP,ETCP 1     # TCP 统计
sar -d 1              # 磁盘
sar -u 1              # CPU
sar -r 1              # 内存
```

sar 来自 sysstat 包,数据由后台 `sysstat` cron 采集到 `/var/log/sa/`。历史数据回看:`sar -f /var/log/sa/sa06 -s 14:00:00 -e 15:00:00`。

### /proc/net/dev

```bash
cat /proc/net/dev
# Inter-|   Receive                                                |  Transmit
#  face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed
#    eth0: 12345  100   0    0    0     0          0         0  6789   80    0    0    0     0       0          0
```

`drop` 列持续增长说明网卡或内核丢包,需检查 ring buffer(`ethtool -g eth0`)或软中断负载。

## 进程追踪

### strace

```bash
# 追踪进程的所有系统调用
strace -p <pid>

# 追踪指定类型的系统调用
strace -p <pid> -f -e trace=network

# 追踪并统计系统调用耗时
strace -p <pid> -c -f

# 追踪进程启动(在启动时附加)
strace -f -e trace=openat,read,write -o strace.log ./myapp
```

`-f` 跟踪子进程,`-e trace=network` 只看网络相关调用,`-c` 汇总统计。

注意:strace 有性能开销(单步中断模式),生产环境慎用,高并发服务可能放大延迟。

### ltrace

```bash
ltrace -p <pid>             # 追踪库函数调用
ltrace -e malloc+free ./myapp  # 只看 malloc/free
```

ltrace 追踪动态库(libc 等)函数调用,适合定位库层问题。对静态链接程序无效。

### /proc/<pid>/fd

```bash
ls -l /proc/<pid>/fd        # 查看进程打开的文件描述符
ls -l /proc/<pid>/fd | wc -l  # fd 数量(排查 fd 泄漏)
cat /proc/<pid>/limits       # 进程资源限制
```

fd 数量接近 `ulimit -n` 默认值(1024)会报 `Too many open files`。查看系统级限制:`cat /proc/sys/fs/file-max`。

## 内核

### sysctl

```bash
# 查看所有内核参数
sysctl -a

# 查看指定参数
sysctl net.ipv4.tcp_max_tw_buckets
sysctl vm.swappiness

# 临时修改
sudo sysctl -w vm.swappiness=10

# 永久修改(写入配置文件)
echo "vm.swappiness=10" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

常见调优参数:

- `vm.swappiness`(0-100)—— 越低越倾向回收页缓存而非换出匿名页,数据库节点建议 10。
- `vm.dirty_ratio` —— 脏页占内存比例触发同步写,默认 20。
- `net.core.somaxconn` —— listen backlog 上限。
- `net.ipv4.tcp_tw_reuse` —— TIME_WAIT 复用(高并发短连接场景)。
- `fs.file-max` —— 系统级最大 fd 数。

### dmesg

```bash
dmesg -T            # 人类可读时间戳
dmesg --follow      # 持续输出(类似 tail -f)
dmesg -l err,warn   # 只看 error 和 warning
```

dmesg 读取内核环形缓冲区,包含硬件错误、驱动日志、OOM 记录、panic 栈。

### /proc/interrupts 与 softirqs

```bash
cat /proc/interrupts     # 硬中断统计(按 CPU 分列)
cat /proc/softirqs       # 软中断统计
watch -n1 "cat /proc/interrupts"
```

软中断类型:`NET_RX`(网络收包)、`NET_TX`(网络发包)、`TIMER`(定时器)、`SCHED`(调度)、`RCU`(RCU 回收)。`NET_RX` 异常高可能是网卡流量过大或 NAPI 轮询效率低,可检查 RPS/RFS 是否启用(`/sys/class/net/eth0/queues/rx-*/rps_cpus`)。

### cpufreq

```bash
cat /proc/cpuinfo | grep MHz          # 当前各 CPU 频率
cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor  # 调度策略
```

governor 为 `powersave` 时 CPU 频率被限制,服务器应设为 `performance`:

```bash
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
```

性能模式避免动态调频引入的延迟抖动,对延迟敏感型服务(数据库、消息队列)尤其重要。
