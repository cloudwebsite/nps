# 🔀 NPS 重构融合增强版

>  展示版 `0.26.41.1` · 核心握手 **`0.26.0`（勿抬高）**

中文 | [English](./README_en.md)

一款轻量、高性能的 **内网穿透** 代理（tcp / udp / http(s) / socks5 / 混合代理 / p2p / Shadowsocks / HTTP/3 等），带 Web 管理端。  
本目录是**重构融合增强版**：在兼容契约下，把多条分支的能力合入同一棵树，并做了安全、传输、面板与运维上的系统性加固——既保留原版简单易用的体验，又补齐生产可用的高级能力，**不是上游原版镜像**。

---

## ✨ 融合亮点

- **无损升级**：启动自动补齐新增配置项（保留注释与原值），不丢配置；先升服务端，旧版客户端照常连接，核心握手锁定 `0.26.0`
- **全协议覆盖**：tcp / udp / http(s) / socks5 / 混合代理（HTTP 与 SOCKS5 同端口）/ p2p / Shadowsocks(SS2022) / HTTP/3
- **现代传输**：TLS 桥接加密、KCP / WebSocket(WS+WSS) 旁路、STUN 打洞 + QUIC 的现代 P2P、PROXY Protocol 传递真实 IP
- **安全加固**：密码 bcrypt 存储、登录会话硬化、越权归属校验、管理员 TOTP 双因素、隐藏安全入口、WAF 防火墙、隧道闲置自动停用、IP/CIDR + GeoIP 黑白名单
- **多用户与开放 API**：独立用户体系与配额、统一 `/api/v1`、操作与连接审计
- **面板即运维**：现代化控制台（浅/深色主题）、服务端配置在线修改、证书管理与自动签发、Web 终端
- **📣 运维通知**：可选企微 / Telegram / Webhook 推送（默认关）——客户端上下线、健康检查异常、错误页命中即时告警
- **多形态客户端**：命令行、Fyne 轻量桌面（含 Android）、Wails 完整桌面，正式双发

---

## 🧩 功能清单

### 🔀 升级与兼容

- 配置只增迁移：升级启动自动补齐 `nps.conf` / `global.json` 新增键，保留注释与取值
- 数据文件向前兼容：`clients.json` / `tasks.json` / `hosts.json` 增字段不影响旧文件加载
- 旧客户端兼容：核心握手保持 `0.26.0`，先升 nps 后旧 npc 不断连

### 🌐 代理与域名

- 协议：tcp / udp / http(s) / socks5 / 混合代理 / p2p / Shadowsocks(SS2022) / HTTP/3
- TLS 直通与 SNI 终止（`HttpsJustProxy`，按域名选择）
- 自动证书：Let's Encrypt 与 ZeroSSL 自动申请 / 续签（`ssl_email` / `ssl_path` / `force_auto_ssl`，域名 `AutoSSL`；默认关）
- 域名增强：CORS、请求 / 响应头改写、路径前缀重写，健康检查更完善
- 嵌套转发：隧道目标支持 `tunnel://`、`bridge://web`、`file://`
- 真实 IP：隧道可选 Proxy Protocol v1/v2；域名可选注入真实 IP 头
- 自定义错误页与品牌（`http_error_page_*`）

### 🛡️ 安全

- 面板密码 bcrypt 存储，旧明文密码登录后自动升级
- 登录会话硬化：换会话 ID + `SameSite=Strict`（HTTPS 加 `Secure`），闲置 2 小时失效
- 权限归属校验：普通用户只能管理自己的客户端 / 隧道 / 域名 / 终端
- 管理员 TOTP 双因素（`totp_secret` / `nps -gen2fa`）、面板安全入口（`web_secure_entry`）
- WAF 防火墙：UA 黑名单、禁止国外访问、自定义规则（默认关）
- 隧道闲置自动停用：tcp/udp 无连接满 N 分钟自动停用（默认关）
- ACL / GeoIP：IP / CIDR 黑白名单 + GeoIP 离线库与在线更新（`nps -updategeo`）
- 全局资源上限：`max_clients` / `max_tunnels_per_client` / `max_global_conn`（默认 0 = 不限）

### 🚚 传输内核

- TLS 桥接加密（`tls_enable` / `tls_bridge_port`）
- 多协议旁路：KCP / WebSocket（WS / WSS）可选端口，未配置则不监听
- 一 vkey 多节点：`bridge_select_mode=primary|roundrobin|random`
- 安全挑战握手（`bridge_secure_handshake`，默认关）
- 现代 P2P：`p2p_mode=legacy|stun|modern`，`modern` = STUN 打洞后 QUIC，可选 holepunch 协调器
- 客户端多服务器：`npc -server=a,b -vkey=... -type=tcp,tls`
- 远程升级客户端（`allow_remote_update`，默认关）、自建下载源（`release_api_url`）

### 👥 多用户 / API / 审计

- 独立用户体系与客户端归属，支持客户端 / 隧道配额与到期时间
- 统一 `/api/v1`：覆盖客户端、隧道、域名、用户、全局参数、Geo、审计、状态（旧 `/auth` 保留）
- 操作审计与连接审计（`audit_log_path` / `audit_connection`，默认关），面板「审计」页可查
- 注册增强：自助注册、限频与验证码、SMTP 邮箱验证码（默认关）

### 🎛️ 面板与运维

- 现代化控制台：Bootstrap 5 壳层、浅 / 深色主题、响应式列表与表单
- 环境标识：顶栏 `web_welcome` / 侧栏 `web_subtitle`，多实例一眼区分
- 服务端配置页：面板按分组改 `nps.conf`（白名单键、敏感项脱敏、保存前备份、重启提示）
- 证书管理页：上传 / 粘贴 / 校验，浏览托管目录（颁发者 / 到期 / 指纹）
- Web 终端：`allow_web_terminal`（默认关），快捷命令持久化、Windows 优先 ConPTY
- 📣 运维通知：可选企微 / Telegram / Webhook 推送（`notify_enable`，默认关）
  - 触发事件：客户端上下线、健康检查异常、错误页命中
  - 通道：企微机器人 Webhook、Telegram Bot（Token + Chat ID）、自定义 Webhook（可加自定义请求头）
  - 节流与队列：通用最小间隔 `notify_min_interval`、错误页专属间隔 `notify_error_page_min_interval`、队列长度 `notify_queue_size`、请求超时 `notify_http_timeout`
  - 隐私：标题前缀 `notify_title_prefix`；`notify_public_client` 控制是否附带客户端公网信息（默认不发）

### 🖥️ 客户端与 GUI

- 命令行：面板复制启动命令，或 `npc -server=主机:8024 -vkey=密钥`
- 🪶 **Fyne**（轻量 / Android）：`gui/npc`
- 🖥️ **Wails**（托盘 / 多连接）：`cmd/npc/npc-gui`（Release 打 Win / macOS / Linux）

---

## 🚀 快速开始

### 1. 构建（CLI）

需本机 **Go 1.24+**（holepunch / 部分 GUI 相关依赖建议 **1.26+**），并加入 `PATH`：

```powershell
cd "f:\go_lang\nps"
go mod tidy
go test ./client/ ./lib/crypt/ ./lib/mail/ ./lib/notify/ ./server/proxy/ ./cmd/npc/ ./lib/install/ -count=1
go build -o nps.exe ./cmd/nps
go build -o npc.exe ./cmd/npc
```

可选升级演练：`.\scripts\upgrade-drill.ps1`

### 2. 服务端

```text
# Linux / macOS
sudo ./nps install && sudo nps start

# Windows（管理员）
nps.exe install
nps.exe start
```

默认相关端口（以 `conf` 为准）：

| 端口 | 用途 |
|------|------|
| 80 / 443 | 域名解析 |
| 8080 | Web 管理 |
| 8024 | 网桥（客户端 ↔ 服务端） |

配置目录：Windows 安装后多为 `C:\Program Files\nps`；Linux / Darwin 多为 `/etc/nps`。  
首次登录请立刻改掉默认账号密码。

### 3. 客户端

- **命令行**：面板复制启动命令，或  
  `npc -server=主机:8024 -vkey=密钥`
- **图形（双发）**  
  - 🪶 **Fyne**（轻量 / Android）：`gui/npc`  
  - 🖥️ **Wails**（托盘 / 多连接）：`cmd/npc/npc-gui`（Release 打 Win / macOS / Linux）

---

## 🛡️ 升级口诀（无损优先）

```text
备份 conf → 先升 nps（旧 npc 仍能连）→ 观察 → 分批升 npc → 最后才开强制策略
```

- 二进制名保持 **`nps` / `npc`**
- 核心握手版本锁定 **`0.26.0`**，直至旧客户端清完
- 新能力（挑战握手、指纹强制、邮箱验证等）默认关，避免现网批量掉线

---

## 📖 文档

站点源码在 [`docs/`](./docs/)（VuePress）。常用入口：

- [服务端配置](./docs/server/server_config.md)
- [增强功能](./docs/server/nps_extend.md)
- [使用示例](./docs/extend/example.md)
- [更新日志](./docs/changelog/index.md)

---

## 🔧 本地开发提示

```powershell
# Wails 桌面 GUI（需 Node / Yarn / wails3）
cd cmd\npc\npc-gui
wails3 dev

# Fyne 简易 GUI
go build -o npc-fyne.exe ./gui/npc
```

Release：Fyne Android APK + Wails 桌面（Win / macOS / Linux）双发。

---

## 🤝 贡献与问题

- 行为变更须遵守现网无损升级约定
- 协议 / 握手相关改动务必保持旧 `npc` 默认可连
