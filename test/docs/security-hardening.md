# 生产环境安全加固清单

服务器上线前的安全基线。按维度组织，每条给出具体配置与验证命令。任何一条未满足即视为上线阻塞项。

## 1. SSH 加固

### 1.1 sshd 配置（/etc/ssh/sshd_config）

```
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
PermitEmptyPasswords no
AllowUsers deploy ops-admin
AllowGroups ssh-users
ClientAliveInterval 300
ClientAliveCountMax 2
MaxAuthTries 3
LoginGraceTime 30
X11Forwarding no
```

改后验证：

```bash
sshd -t                          # 语法检查
systemctl reload sshd            # 热加载（不踢现有会话）
ss -tlnp | grep :22              # 确认监听
```

### 1.2 fail2ban 暴力破解防护

```bash
apt install fail2ban
# /etc/fail2ban/jail.local
[sshd]
enabled = true
maxretry = 5
findtime = 600
bantime = 3600
bantime.increment = true         # 累犯递增封禁

systemctl enable --now fail2ban
fail2ban-client status sshd      # 查看封禁状态
```

### 1.3 密钥轮换

```bash
# 生成 ed25519 密钥（比 RSA 更短更安全）
ssh-keygen -t ed25519 -C "deploy@host-2026" -f ~/.ssh/id_ed25519

# 禁用旧密钥：编辑 authorized_keys 删除对应公钥行
# 强制轮换周期写入运维 SOP，建议每 90 天
```

`ClientAliveInterval 300` + `ClientAliveCountMax 2` 表示 10 分钟无响应踢出空闲会话，防止遗忘的 SSH 终端成为入侵入口。

## 2. 防火墙基线

### 2.1 ufw（Ubuntu/Debian）

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp         # SSH
ufw allow 443/tcp        # HTTPS
ufw enable
ufw status verbose
```

### 2.2 firewalld（RHEL/CentOS）

```bash
firewall-cmd --permanent --zone=public --add-service=https
firewall-cmd --permanent --zone=public --add-service=ssh
firewall-cmd --permanent --zone=internal --add-source=10.0.0.0/8
firewall-cmd --reload
firewall-cmd --list-all
```

区域划分：`public` 仅暴露必要端口，`internal` 放内网管理源，`drop` 拒绝所有入站。

### 2.3 nftables stateful rules

```nft
table inet filter {
    chain input {
        type filter hook input priority 0; policy drop;
        ct state established,related accept
        iif lo accept
        tcp dport { 22, 443 } ct state new accept
        counter drop
    }
}
```

默认策略 `DROP`，仅放行已建立连接、回环、22/443 新连接。`ct state` 实现有状态过滤，比无状态逐包匹配更高效。

## 3. 文件权限与完整性

### 3.1 关键路径

```bash
chmod 700 ~/.ssh
chmod 600 ~/.ssh/authorized_keys
chmod 600 ~/.ssh/id_*

ls -l /etc/shadow    # 期望: -r-------- 或 -rw-r----- root:shadow (640)
ls -l /etc/passwd    # 期望: -rw-r--r-- root:root (644)
```

### 3.2 SUID 审计

```bash
# 查找所有 SUID 文件
find / -perm -4000 -type f 2>/dev/null

# 与基线对比，新增 SUID 是入侵信号
# 正常应只有: su, sudo, passwd, mount, umount, ping 等
# 记录基线:
find / -perm -4000 -type f 2>/dev/null | sort > /var/log/suid_baseline.txt
```

定期运行 diff 比对：

```bash
diff /var/log/suid_baseline.txt <(find / -perm -4000 -type f 2>/dev/null | sort)
```

### 3.3 /etc/passwd 完整性

```bash
# 检查是否有 UID=0 的非 root 账户（提权后门）
awk -F: '$3 == 0 {print}' /etc/passwd

# 检查空口令账户
awk -F: '($2 == "" || $2 == "*") {print $1}' /etc/shadow
```

## 4. 容器安全

### 4.1 运行时加固

```bash
docker run \
  --read-only \                      # 根文件系统只读
  --tmpfs /tmp \                     # 临时目录单独挂载可写
  --user 1000:1000 \                 # 非 root 运行
  --cap-drop ALL \                   # 丢弃所有 capabilities
  --cap-add NET_BIND_SERVICE \       # 仅按需添加
  --security-opt no-new-privileges \ # 禁止提权
  --security-opt seccomp=/etc/docker/seccomp-profile.json \
  --memory="512m" --cpus="1.0" \
  myapp:tag
```

### 4.2 各参数原理

| 参数 | 作用 |
|------|------|
| `--read-only` | 防止攻击者写入恶意文件持久化 |
| `--user 1000:1000` | 即使容器内逃逸也是非 root，降低主机影响 |
| `--cap-drop ALL` | 内核 capability 默认全部丢弃，按需 `--cap-add` |
| `no-new-privileges` | 阻止子进程通过 setuid 提权 |
| `seccomp` | 限制可调用的系统调用集 |

### 4.3 镜像基线

```dockerfile
FROM alpine:3.19
RUN addgroup -S app && adduser -S app -G app -u 1000
USER 1000:1000
# 不以 root 构建运行
```

构建后扫描：

```bash
trivy image myapp:tag
docker scout cves myapp:tag
```

## 5. 密钥与凭据管理

### 5.1 禁止硬编码

凭据不得出现在：
- 环境变量（`docker inspect` 可见明文）
- Dockerfile / 镜像层（每层都可 `docker history` 还原）
- 源码仓库（git 历史不可擦除）
- CI 日志明文输出

```bash
# 验证镜像无密钥残留
docker history --no-trunc myapp:tag | grep -iE 'key|secret|password|token'

# 扫描源码
git secrets --scan
trufflehog filesystem .
```

### 5.2 密钥分发方案

- **Kubernetes:** 用 [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) 或 [External Secrets Operator](https://external-secrets.io/) 对接 Vault
- **HashiCorp Vault:** 运行时动态获取短期凭据，应用通过 sidecar 或 SDK 拉取
- **.env 文件:** 必须在 `.gitignore` 中：

```gitignore
# .gitignore
.env
.env.*
*.pem
*.key
```

验证 `.env` 未被跟踪：

```bash
git ls-files | grep -E '\.env'
# 应无输出
```

## 6. 审计日志（auditd）

### 6.1 规则配置

```bash
# /etc/audit/rules.d/audit.rules
-w /etc/passwd -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/sudoers -p wa -k identity
-w /var/log/auth.log -p wa -k logins
-a always,exit -F arch=b64 -S chmod,chown,setuid -k perm_mod
-a always,exit -F arch=b64 -S unlink,unlinkat,rename -k delete
```

加载与验证：

```bash
augenrules --load
auditctl -l                  # 查看已加载规则
auditctl -e 2                # 锁定规则（需重启才能改，防篡改）
```

### 6.2 查询与报表

```bash
# 按关键字查事件
ausearch -k identity

# 按用户
ausearch -ua deploy

# 汇总报表（登录尝试、命令执行等）
aureport --auth --summary
aureport --comm              # 命令执行统计
aureport -x                  # 可执行文件调用

# 实时跟踪
tail -f /var/log/audit/audit.log | aureport -i --interpret
```

auditd 日志会增长快，配置日志轮转与远程转发（rsyslog 到中央 SIEM）。

## 7. 内核参数加固

### 7.1 sysctl 配置

```bash
# /etc/sysctl.d/99-hardening.conf

# 内核指针不泄露（防内核地址泄露）
kernel.kptr_restrict = 2

# dmesg 仅 root 可读
kernel.dmesg_restrict = 1

# 非特权用户不能使用 bpf
kernel.unprivileged_bpf_disabled = 1

# 禁止非特权用户看他人进程
kernel.unprivileged_userns_clone = 0

# 禁用核心转储（防内存泄露）
fs.suid_dumpable = 0

# SYN flood 缓解
net.ipv4.tcp_syncookies = 1
net.ipv4.tcp_max_syn_backlog = 4096

# 反向路径过滤
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1

# 禁止源路由
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0

# 禁 ICMP 重定向
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0

# 忽略 ICMP 广播（防 smurf）
net.ipv4.icmp_echo_ignore_broadcasts = 1
```

应用：

```bash
sysctl --system
# 或单独加载
sysctl -p /etc/sysctl.d/99-hardening.conf

# 验证
sysctl kernel.kptr_restrict
sysctl kernel.dmesg_restrict
```

### 7.2 /proc/sys 直读

```bash
# 等价于 sysctl kernel.kptr_restrict
cat /proc/sys/kernel/kptr_restrict

# 列出所有可调参数
ls /proc/sys/net/ipv4/ | head
```

## 8. TLS 配置

### 8.1 协议与密码套件

仅启用 TLS 1.2+，禁用旧版本：

**Nginx:**

```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers on;

# TLS 1.2 密码套件（按强度排序，PFS 优先）
ssl_ciphers 'ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256';
ssl_session_timeout 1d;
ssl_session_cache shared:SSL:50m;
ssl_session_tickets off;

# OCSP stapling
ssl_stapling on;
ssl_stapling_verify on;
resolver 1.1.1.1 valid=300s;
resolver_timeout 5s;
```

**Apache:**

```apache
SSLProtocol all -SSLv3 -TLSv1 -TLSv1.1
SSLCipherSuite ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305
SSLHonorCipherOrder on
```

### 8.2 HSTS

强制浏览器后续都用 HTTPS，防降级与中间人：

```nginx
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
```

`max-age` 至少 6 个月；确认全站 HTTPS 后再加入 [HSTS preload list](https://hstspreload.org/)。

### 8.3 OCSP stapling

开启后服务端主动获取 OCSP 响应并下发给客户端，免去浏览器单独查询 CA，降低握手延迟与隐私泄露。

```bash
# 验证 stapling 是否生效
openssl s_client -connect example.com:443 -status </dev/null 2>&1 | grep -A2 "OCSP Response Status"
# 期望: OCSP Response Status: successful
```

### 8.4 评级验证

配置完成后用 [SSL Labs](https://www.ssllabs.com/ssltest/) 跑一次完整评级，目标 A+。重点项：协议版本、密码套件强度、证书链完整性、HSTS、OCSP stapling。本地可用 `testssl.sh` 离线扫描：

```bash
./testssl.sh --severity HIGH example.com
```
