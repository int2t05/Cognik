# 新员工 IT 入职指南

## 欢迎入职！

本文档帮助你快速完成 IT 环境配置，预计耗时 30 分钟。

## 一、账号信息

入职当天，IT 部门会为你创建以下账号（初始密码通过短信发送）：

| 账号类型 | 格式 | 用途 |
|----------|------|------|
| 域账号 | `domain\工号` | 电脑登录、WiFi、VPN、邮箱 |
| 邮箱 | `姓名拼音@company.com` | 邮件通信、日历、Teams |
| OA 账号 | 工号 | 审批、考勤、报销 |
| 企业微信 | 绑定手机号 | 即时通讯、移动办公 |

**首次登录务必修改密码！** 密码要求：
- 长度至少 8 位，最多 32 位
- 必须包含大写字母、小写字母、数字
- 不能包含用户名或工号
- 90 天强制更换

## 二、电脑初始化

### 2.1 标准配置

公司标配设备：
- **笔记本**：ThinkPad T14 Gen 5（i7-1365U / 32GB RAM / 1TB SSD）
- **显示器**：Dell U2723QE 27" 4K USB-C
- **外设**：罗技 MX Keys 键盘 + MX Master 3S 鼠标
- **耳机**：Jabra Evolve2 65

### 2.2 系统初始化步骤

1. 使用域账号登录 Windows
2. 连接公司 WiFi（SSID: `Corp-WiFi`，使用域账号认证）
3. 等待组策略自动安装基础软件（约 10 分钟）
4. 打开「公司软件中心」（Software Center），手动安装以下软件：
   - Microsoft Office 365
   - Visual Studio Code
   - Google Chrome Enterprise
   - 7-Zip
   - 企业微信桌面版

### 2.3 开发环境配置（仅研发人员）

```bash
# 安装 Windows Terminal（从 Microsoft Store）
# 安装 WSL2
wsl --install -d Ubuntu-22.04

# 安装开发工具
sudo apt update && sudo apt install -y git curl build-essential

# 配置 Git（使用公司 GitLab）
git config --global user.name "你的姓名"
git config --global user.email "你的邮箱@company.com"
git config --global url."https://gitlab.company.com/".insteadOf "git@gitlab.company.com:"
```

## 三、常用系统访问

| 系统 | 地址 | 用途 |
|------|------|------|
| OA 系统 | https://oa.company.com | 审批、考勤、报销 |
| 企业邮箱 | https://mail.company.com | 邮件、日历 |
| GitLab | https://gitlab.company.com | 代码管理 |
| Confluence | https://wiki.company.com | 文档协作 |
| Jira | https://jira.company.com | 项目管理 |
| VPN | https://vpn.company.com | 远程办公接入 |
| IT 服务台 | https://itsm.company.com | 故障申告、设备申请 |

## 四、打印机配置

公司打印机通过打印服务器统一管理：

1. 打开「设置 → 蓝牙和其他设备 → 打印机和扫描仪」
2. 点击「添加设备」→「通过 IP 地址或主机名添加打印机」
3. 输入打印服务器地址：`print-server.company.com`
4. 按楼层选择对应打印机：

| 楼层 | 打印机型号 | 队列名称 |
|------|-----------|----------|
| 1F | HP LaserJet M507 | `PRT-1F-East` |
| 2F | HP LaserJet M507 | `PRT-2F-Center` |
| 3F | HP LaserJet M404 | `PRT-3F-East` |
| 4F | HP Color LaserJet M454 | `PRT-4F-West`（彩色） |

## 五、安全规范（必读）

### 5.1 数据分类

| 级别 | 示例 | 存储要求 |
|------|------|----------|
| 公开 | 产品手册、公开公告 | 无限制 |
| 内部 | 项目文档、部门报告 | 需公司设备访问 |
| 机密 | 财务数据、人事档案 | 加密存储，禁止外传 |
| 绝密 | 商业机密、战略规划 | 专用加密设备，审计追踪 |

### 5.2 日常安全要求

- 离开工位时锁定屏幕（快捷键 Win+L）
- 不得将公司设备连接非授权的 USB 存储设备
- 不得将公司文档通过个人微信/QQ/网盘传输
- 收到可疑邮件请通过 Outlook「报告钓鱼」按钮上报
- 发现安全事件立即联系信息安全组（分机 1111）

## 六、常见问题

### 6.1 忘记密码怎么办？

1. 在登录界面点击「重置密码」
2. 按提示通过绑定的手机号或备用邮箱验证
3. 若自助重置失败，联系 IT 服务台（分机 8888）

### 6.2 新申请的软件/权限需要多久？

- 软件安装：1-2 个工作日（需上级审批）
- 系统权限：1 个工作日（需部门负责人审批）
- 特殊权限（如数据库读写权限）：3 个工作日（需数据 owner 审批）

### 6.3 IT 设备故障找谁？

直接提交申告到 IT 服务台：https://itsm.company.com
或拨打分机 8888（工作时间 9:00-18:00）。

---

> **文档版本**：v3.0 | **最后更新**：2026-06-01 | **适用对象**：全体新入职员工
