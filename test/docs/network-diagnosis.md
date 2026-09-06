# 网络诊断与故障排查工具集

生产网络故障的排查需要从底层 TCP 状态到应用层 TLS 全链路覆盖。本文档汇总运维实战中高频使用的诊断命令与排障路径，按 OSI 层级与故障域组织。

## 1. TCP 连接状态分析

### 1.1 状态统计

```bash
ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
```

输出示例：

```
   5325 ESTAB
    842 TIME-WAIT
    127 CLOSE-WAIT
     45 SYN-RECV
     12 LISTEN
```

重点关注 `TIME-WAIT`、`CLOSE-WAIT`、`SYN-RECV`。`CLOSE-WAIT` 堆积通常意味着应用层未正确关闭连接（fd 泄漏或未调用 `close()`）。

### 1.2 TIME_WAIT 堆积处理

高并发短连接场景下 TIME-WAIT 可达数万。排查与缓解：

```bash
# 查看当前 TIME-WAIT 数量
ss -tan state time-wait | wc -l

# 内核参数调优（仅做缓解，根因是短连接）
sysctl net.ipv4.tcp_tw_reuse=1       # 允许复用 TIME-WAIT 连接
sysctl net.ipv4.tcp_max_tw_buckets=50000  # 限制上限，超出直接释放

# 根因解决：长连接 + 连接池，避免频繁建连
```

注意：`tcp_tw_reuse=1` 仅对出站连接（客户端侧）安全；`tcp_tw_recycle` 已在内核 4.12 移除，因 NAT 环境下会丢包。

### 1.3 /proc/net/tcp 直查

当 `ss`/`netstat` 不可用时直接读取内核状态：

```bash
# 字段：sl local_address rem_address st tx_queue rx_queue ...
# st=06 表示 TIME_WAIT
cat /proc/net/tcp | awk '{print $4}' | sort | uniq -c

# 地址转 IP（十六进制倒序）
# 0100007F:01BB → 127.0.0.1:443
```

## 2. DNS 解析故障

### 2.1 dig 全链路追踪

```bash
# 从根域逐级解析，定位哪一环节断裂
dig +trace example.com @8.8.8.8

# 指定记录类型与 DNS 服务器
dig A api.example.com @1.1.1.1
dig MX example.com +short

# 查看 TTL 与解析路径
dig example.com +stats
```

`+trace` 会从根服务器开始迭代查询，能看到 NS 委托链在哪一层断裂。

### 2.2 nslookup 交叉验证

```bash
nslookup example.com 8.8.8.8
nslookup -type=SRV _ldap._tcp.example.com
```

### 2.3 解析配置

```bash
# 查看 resolver 配置
cat /etc/resolv.conf

# systemd-resolved 状态（Ubuntu/CentOS 现代发行版）
resolvectl status
systemd-resolve --status   # 旧接口

# 检查是否 split-horizon DNS（内/外返回不同）
# 同一域名在 VPN 内外对比
dig @internal-dns.corp example.com
dig @8.8.8.8 example.com
```

### 2.4 /etc/hosts 优先级

`/etc/hosts` 优先级高于 DNS（由 `nsswitch.conf` 中 `hosts:` 行决定）。排查时先检查：

```bash
cat /etc/hosts
grep "^hosts:" /etc/nsswitch.conf
# 默认: hosts: files dns
```

调试时临时加行可快速覆盖解析，但不要遗留为永久方案。

## 3. 路由与路径探测

### 3.1 路由表

```bash
ip route show              # 查看主路由表
ip route get 10.0.0.5      # 查询到具体地址走哪条路由
ip -6 route show           # IPv6 路由
```

直读内核：

```bash
cat /proc/net/route
# 字段: Iface Destination Gateway Flags ...
```

### 3.2 路径追踪

`traceroute` 默认用 UDP，会被防火墙或 NAT 干扰。生产排查用 TCP 模式：

```bash
# TCP SYN 模式，贴近真实业务流量（HTTPS）
traceroute -T -p 443 api.example.com

# mtr 持续探测，统计每跳丢包率与延迟
mtr --report --report-cycles 10 --tcp --port 443 api.example.com

# IPv6
mtr -6 --report example.com
```

`mtr --report` 跑 10 个周期后输出汇总；关注中间跳高丢包是否持续（单跳丢包可能是 ICMP 限速，不算故障）。

## 4. 防火墙与连接跟踪

### 4.1 iptables / nftables 查询

```bash
# iptables: 按链查看计数器
iptables -L -n -v --line-numbers
iptables -t nat -L -n -v

# nftables: 导出完整规则集
nft list ruleset

# firewalld（RHEL/CentOS）
firewall-cmd --list-all
firewall-cmd --get-active-zones
firewall-cmd --zone=public --list-services
```

### 4.2 conntrack 表耗尽

NAT/防火墙场景下 conntrack 满会导致新连接被丢弃：

```bash
# 当前条目数
cat /proc/net/nf_conntrack | wc -l

# 查看上限
sysctl net.netfilter.nf_conntrack_max

# 统计各状态分布
cat /proc/net/nf_conntrack | awk '{print $4}' | sort | uniq -c

# 观察丢包计数
grep conntrack /proc/net/stat/nf_conntrack
```

conntrack 满的症状：`dmesg` 出现 `nf_conntrack: table full, dropping packet`。缓解：调大 `nf_conntrack_max`、减少短连接、启用 `NOTRACK` 规则豁免高吞吐流量。

## 5. MTU / MSS 与分片

### 5.1 PMTUD 探测

```bash
# 发送不分片的包，逐步缩小找出路径 MTU
ping -M do -s 1472 target.example.com
# -s 1472 = MTU 1500 - 28(IP+ICMP 头)
# 若 "Frag needed and DF set" → 路径 MTU < 1500

# 逐步缩小到找到最大不报错的值
ping -M do -s 1400 target.example.com
ping -M do -s 1200 target.example.com
```

### 5.2 MTU 设置

```bash
ip link set dev eth0 mtu 1400
ip link show eth0   # 验证
```

PMTUD 失败的典型症状：小包（SSH、DNS）通，大包（HTTPS 大响应）卡住或超时。这是 ICMP "Frag needed" 被防火墙拦截导致。

### 5.3 tcpdump 抓分片包

```bash
# 抓 IP 分片包
tcpdump -i eth0 -n 'ip[6:2] & 0x1fff != 0'

# 抓 MSS 协商
tcpdump -i eth0 -n 'tcp[tcpflags] & tcp-syn != 0' -vv
```

## 6. TLS / 证书诊断

### 6.1 证书链与握手

```bash
# 连接并查看证书链 + 协商版本与密码套件
openssl s_client -connect example.com:443 -servername example.com -showcerts </dev/null

# 仅看证书有效期
echo | openssl s_client -connect example.com:443 -servername example.com 2>/dev/null \
  | openssl x509 -noout -dates -issuer -subject

# 指定协议版本测试
openssl s_client -connect example.com:443 -tls1_2 </dev/null
openssl s_client -connect example.com:443 -tls1_3 </dev/null
```

### 6.2 curl 详细握手

```bash
curl -vI https://example.com 2>&1 | grep -E 'SSL|TLS|certificate|subject|issuer'
```

### 6.3 外部评级

`openssl` 只看本地验证；用 [SSL Labs](https://www.ssllabs.com/ssltest/) 评级链路完整性与信任链。重点检查：是否 TLS 1.0/1.1 仍启用、是否 OCSP stapling、是否 HSTS。

## 7. 抓包分析

### 7.1 tcpdump 抓取

```bash
# 抓 443 流量到文件，后续用 Wireshark 分析
tcpdump -i eth0 -w capture.pcap port 443 -s 0

# 抓特定主机双向流量
tcpdump -i eth0 host 10.0.0.5 and port 443 -w cap.pcap -s 0

# 实时过滤查看
tcpdump -i eth0 -nn 'tcp port 443 and (tcp[((tcp[12:1] & 0xf0) >> 2):1] = 0x18)' -A
```

### 7.2 BPF 过滤速查

```bash
# 只抓 SYN 包
tcpdump -i eth0 'tcp[tcpflags] & tcp-syn != 0 and tcp[tcpflags] & tcp-ack == 0'

# 只抓 RST
tcpdump -i eth0 'tcp[tcpflags] & tcp-rst != 0'

# 排除 SSH 噪声
tcpdump -i eth0 'not port 22'

# VLAN 流量
tcpdump -i eth0 vlan
```

### 7.3 Wireshark 分析要点

将 pcap 拖入 Wireshark 后：右键报文 → Follow → TCP Stream 查看完整交互；用 `tcp.analysis.retransmission` 显示过滤器定位重传；用 `http.response.code >= 500` 定位服务端错误。

抓包是排障的终极手段——当所有指标都"正常"但业务不通时，抓包能看到真实字节流，暴露中间设备篡改、TLS 版本协商失败、半开连接等隐蔽问题。
