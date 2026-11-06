# 增强功能

## 使用 https

> nps 会根据域名是否上传了证书自动选择处理方式：

| 是否在 web 上传证书 | 行为 | 真实 IP |
| --- | --- | --- |
| 是 | nps 直接接管 SSL 终结，再转发到内网 | ✅ 可拿到 |
| 否 | nps 透传 TLS（端口转发），由内网服务器自行解密 | ❌ 拿不到 |

![image](/image/new/https.png)

## 自动 HTTPS

在 web 域名编辑页勾选「自动 HTTPS」后，nps 会把对应域名的 http 请求 301 重定向到 https。
重定向会按 `https_proxy_port` 配置的端口跳转（例如 `https_proxy_port=8443` 时会跳到 `https://<host>:8443/...`），方便非标准端口部署。

## HTTPS 证书来源

web 域名编辑页的「证书」/「私钥」字段同时支持：

- **直接粘贴证书 / 私钥内容**
- **填写服务器上的文件路径**（例如 `/etc/letsencrypt/live/proxy.com/fullchain.pem`）

nps 会自动识别是文件路径还是内容。文件路径方式便于配合 acme.sh / certbot 自动续期，无需每次手工拷贝。



## TLS 桥接加密

为防止客户端到服务端的桥接流量被防火墙识别和拦截，可启用 TLS：

```ini
# nps.conf
tls_enable=true
tls_bridge_port=8025      # v0.26.18 引入，与 bridge_port 并存
```

客户端任选一种方式接入：

```shell
npc.exe -server=1.1.1.1:8024 -vkey=xxx                       # 明文桥接（bridge_port）
npc.exe -server=1.1.1.1:8025 -vkey=xxx -tls_enable=true      # TLS 桥接（tls_bridge_port）
```

> 客户端忽略证书校验，请勿暴露在不可信网络里。

## 全局参数

> 全局参数保存在 `conf/global.json`，通过 Web 后台保存后**立即生效，无需重启**。全新安装会直接生成默认文件；升级时首次启动会自动把 `nps.conf` 中对应的开关迁移过来，之后以 Web 页面为准。

web 后台 → 全局参数 页面（对应 `conf/global.json`）：

| 字段 | 原 nps.conf 参数 | 含义 |
| --- | --- | --- |
| 全局 IP 黑名单 | - | 一行一个 IPv4，不支持范围匹配。命中后所有 TCP/HTTP/HTTPS/UDP 入口请求都会被直接关闭。用于防止扫描和暴力破解 |
| 产品名称 | `appname`（仅首次迁移种子） | 控制面板侧栏 / 登录页品牌名，默认 `NPS`；存 `global.json` 的 `AppName`，后台改完立即生效，无需重启 |
| 顶栏环境标识 / 侧栏副标题 | `web_welcome` / `web_subtitle` | 面板展示文案；热更新 |
| 信任真实 IP 头 / 远程升级 / Web 终端 | `allow_x_real_ip` / `allow_remote_update` / `allow_web_terminal` | 运行门闩；热更新 |
| WAF 功能开关 | `waf_enable` 等 | 总开关与分项开关热更新；规则仍在 WAF 页 / `waf.json`；`geoip_path` 改路径仍需重启 |
| 注册限频 / 邮箱验证 / SMTP | `register_*` / `smtp_*` | 热更新 |
| 运维通知 / 错误页品牌 / Release 源 / 资源上限 | `notify_*` / `http_error_page_brand*` / `release_*` / `max_*` | 热更新（错误页**模板路径**仍需重启）。`release_*` 自建镜像见 [自建更新源](../extend/release_mirror.html) |
| 服务地址 | - | 用于在 web 上生成 [客户端快捷启动命令]、隧道访问 URL 时拼接的服务端地址。留空则使用浏览器当前访问地址 |
| 允许端口 | `allow_ports` | 限制可开放的隧道端口范围，例如 `9001-9009,10001,11000-12000`，留空不限制 |
| IP 校验 | `ip_limit` | 开启后，访客 IP 须先用 `npc register` 登记才能经隧道转发；详见 [IP 校验（npc register）](#ip-校验npc-register) |
| 显示 HTTP 代理端口 | `show_http_proxy_port` | 是否在管理面板展示 HTTP 代理端口 |
| 允许流量限制 | `allow_flow_limit` | 是否在客户端编辑页开放流量上限设置 |
| 允许限速 | `allow_rate_limit` | 是否在客户端编辑页开放限速设置 |
| 允许连接数限制 | `allow_connection_num_limit` | 是否在客户端编辑页开放最大连接数设置 |
| 允许隧道数限制 | `allow_tunnel_num_limit` | 是否在客户端编辑页开放最大隧道数设置 |
| 一隧道多 IP | `allow_multi_ip` | 是否允许为单个隧道配置多个服务端 IP |
| 允许本地代理 | `allow_local_proxy` | 是否允许隧道直接使用服务端本地代理 |
| 客户端账号登录 | `allow_user_login` | 是否允许客户端使用自己的账号登录管理面板 |
| 允许注册 | `allow_user_register` | 是否允许在登录页注册新账号 |
| 允许修改用户名 | `allow_user_change_username` | 是否允许客户端修改自己的登录用户名 |
| 登录验证码 | `open_captcha` | 登录时是否需要输入图形验证码 |
| 系统监控图表 | `system_info_display` | 是否在仪表盘展示 CPU、内存、负载等图表 |
| 注入真实 IP 头 | `http_add_origin_header` | 转发请求时是否写入 `X-Forwarded-For` 和 `X-Real-IP` |

> 端口、监听地址、证书路径、日志、TLS、`geoip_path`、`web_secure_entry`、`totp_secret` 等仍只支持在 `nps.conf` 中配置，修改后需重启服务。上表所列开关与文案类参数请走「全局参数」热更新。

## IP 校验（npc register）

全局参数里的「IP 校验」（`ip_limit` / `IpLimit`，默认关）是一套**访客源 IP 临时放行表**，和客户端编辑页的「IP 白名单 / 授权密码」、以及 ACL 黑白名单**不是同一功能**。

### 要解决什么问题

你把 SSH、RDP 等通过隧道暴露到公网后，希望**只有事先打过招呼的出口 IP** 才能连进来。  
做法：在你当前上网的机器上跑一次 `npc register`，把「此刻服务端看到的你的公网 IP」写进 nps 内存里的登记表，并设一个有效期。开启 IP 校验后，未登记或过期的 IP 访问隧道会被拒绝。

注意：

- 拦的是**访问隧道的访客 IP**（`link.RemoteAddr`），不是面板里「新建隧道」操作。  
- npc 正常在线转发（`npc -server=... -vkey=...`）**不必**先 register；只有打开了全局 IP 校验，才要求访客 IP 已登记。  
- 登记表在**内存**里：nps **重启后清空**，需要重新 `npc register`。  
- 家宽 / 手机热点换出口 IP 后，旧登记失效，须在新出口再登记一次。

### 工作流程

```text
1) 面板：全局参数 → 打开「IP 校验」并保存（立即生效）
2) 在「将要访问隧道」的那台电脑上执行 npc register
3) npc 连到 nps 的 bridge 口，校验 vkey 后走 WORK_REGISTER
4) 服务端记下：该连接的对端 IP → 当前时间 + N 小时
5) 之后有人访问你的公网隧道端口时：
   - 访客 IP 在表里且未过期 → 正常转发
   - 未登记或已过期 → 拒绝（日志类似 The ip x.x.x.x is not in the validation list）
```

服务端登记代码把 `RemoteAddr` 的 IP 与过期时间存入 `Bridge.Register`；转发前在 `SendLinkInfo` 里若 `IpLimit=true` 则查这张表。

### 怎么登记

在**访客本机**（要放行的那台机器）执行，`vkey` 用面板里任意有效客户端密钥（登记前会做客户端校验）：

```shell
# 明文 bridge（默认端口以 nps.conf 的 bridge_port 为准，常见 8024）
npc register -server=公网IP或域名:8024 -vkey=客户端密钥 -time=24

# 若服务端开了 TLS 桥接，且你平时用 tls 连
npc register -server=公网IP或域名:8025 -vkey=客户端密钥 -type=tls -tls_enable=true -time=24
```

| 参数 | 含义 | 默认 |
| --- | --- | --- |
| `-server` | nps 桥接地址 | 必填 |
| `-vkey` | 客户端验证密钥 | 必填 |
| `-time` | 有效期（**小时**） | `2` |
| `-type` | 连接类型，如 `tcp` / `tls` | 与日常 npc 一致 |
| `-tls_enable` | TLS 桥接 | 与日常 npc 一致 |
| `-proxy` | 可选 SOCKS5 代理 | 空 |

成功时控制台类似：`Successful ip registration for local public network, the validity period is N hours.` 然后进程退出（register **不是**常驻客户端）。

也可：`npc register` 后跟与平时相同的 `-server` / `-vkey` 等参数（子命令形式以本机 `npc` 帮助为准）。

### 和另外两套 IP 能力对比

| 能力 | 入口 | 存哪儿 | 典型用途 |
| --- | --- | --- | --- |
| **IP 校验**（本节） | 全局开关 + `npc register` | 服务端内存，按时长过期 | 临时放行「我当前出口」访问隧道 |
| **客户端 IP 白名单 + 授权页** | 客户端编辑页 `IpWhite` / 授权密码 | 持久化到客户端配置 | 域名/HTTP 场景下网页输密码加白 |
| **ACL 黑白名单** | 全局 / 客户端 / 隧道·域名 | 持久化；支持 IP、CIDR、`geoip:` 等 | 长期策略拦截或只放某网段/国家 |

判定顺序上：业务连接会先走 ACL 等既有源地址策略，IP 校验是在向内网客户端要链路（`SendLinkInfo`）时额外检查登记表。

### 使用建议

- 默认保持 **关闭**；只有需要「临时公网暴露 + 白名单出口」时再开。  
- 开了以后，所有依赖 `SendLinkInfo` 的隧道访客都要能登记，否则别人（以及你自己换网后）都会连不上。  
- 长期固定办公室出口，更适合用 **ACL 白名单（CIDR）** 或客户端白名单，不必每次 register。  
- 高风险口（如 SSH）也可配合 [隧道闲置自动停用](#隧道闲置自动停用)：平时停用，用时开启并视情况再开 IP 校验。

## 客户端黑名单
在客户端编辑页可填写多个 IP，命中后该客户端下所有隧道都会拒绝服务，用于针对单个客户端阻断。

## 客户端白名单 + IP 授权

在客户端编辑页：

1. 直接添加 IP 到白名单
2. 或填写「IP 授权密码」。当外网 IP 不在白名单时，会被重定向到 IP 授权页面，输入正确密码即可把该 IP 自动加入白名单。

> 这与上一节「IP 校验 / npc register」不同：白名单持久化在客户端配置里；register 是全局内存临时表。

![img](/image/new/ip.png)


##  Proxy Protocol

在 web 上 TCP 隧道编辑页开启「Proxy Protocol」开关，nps 会以 PROXY v1/v2 协议向后端转发原始来源 IP。后端服务（nginx、HAProxy 等）只需正常解析 PROXY 协议即可拿到真实客户端 IP。

![img](/image/new/protocol.png)


## WEB登录验证码

在 **Web 后台 → 全局参数** 中开启「登录验证码」（对应 `conf/global.json` 的 `open_captcha`）。

开启后 web 登录页会要求图形验证码。结合「web 管理保护」（连续 10 次失败封禁 1 分钟），可以有效缓解暴力破解。

## 管理员 TOTP 双因素

在 `nps.conf` 配置 `totp_secret` 后，**管理员**登录除账号密码外还需填写身份验证器（Google Authenticator / 微软 Authenticator 等）6 位动态码。空值或不配表示关闭。

**步骤示例：**

```text
# 1. 生成密钥（在 nps 可执行文件目录或 PATH 中执行）
nps -gen2fa

# 输出类似：
# TOTP secret: ABCD...
# Authenticator URI: otpauth://totp/NPS:admin?secret=ABCD...
# Write into conf/nps.conf and restart nps:
# totp_secret=ABCD...
```

```ini
# 2. 写入 conf/nps.conf
totp_secret=ABCD...
```

```text
# 3. 重启 nps，用 Authenticator 扫描 URI 或手工录入 secret
# 4. 登录页出现「验证码」输入框，填入当前 6 位数字
```

注意：

- 仅约束管理员（及无认证面板入口）；普通用户 / 客户端 Web 登录不受影响  
- 启用 TOTP 后禁止空凭据隐式登录  
- 改密钥或关闭（清空 `totp_secret`）后都需重启  

## 安全入口（隐藏登录页）

公网暴露管理口时，可把登录页藏在「密钥路径」后面：未先访问该路径的人只会看到 **404**，不易扫到面板。

```ini
# conf/nps.conf（空或不配 = 关闭，行为与原来一致）
web_secure_entry=/s/请换成长随机串
```

行为约定：

1. 先访问 `{web_base_url}{web_secure_entry}`（例如 `https://host:8081/s/请换成长随机串`）→ 写入 HttpOnly Session 解锁标记 → 302 到登录页  
2. 未解锁时，面板 HTML 与 `/static/...` 一律 404  
3. 已登录 Session 仍有效；登出后若 Cookie 还在，可直接再开登录页；否则需再访一次密钥路径  
4. **不**拦截 `/api/v1`（`auth_key+timestamp` / Bearer 照旧）  
5. 可与 `web_base_url`、`totp_secret` 叠加；不替代防火墙 / 反代 IP 限制  

注意：

- 改后需**重启** nps  
- 请用足够长的随机串；勿把真实路径提交到公开仓库  
- 面板「服务端配置」可改该键（敏感项留空保存表示不修改）；关闭请编辑 conf 清空该行后重启  

## 面板展示文案

多实例部署时可在 `nps.conf` 设置环境名，顶栏一眼区分：

```ini
web_welcome=DNS二线
web_subtitle=内网穿透控制台
```

| 项 | 位置 | 说明 |
| --- | --- | --- |
| `web_welcome` | 顶栏 | 环境标识；空则显示默认「欢迎使用」类文案 |
| `web_subtitle` | 侧栏 | 副标题；空则用内置默认 |

改后需重启。不需要编号或特殊前缀，直接写可读文字即可。

## 面板 Web 终端

**默认关闭**（安全起见）。开启后可在浏览器连接**在线客户端**的交互式 Shell。

```ini
# conf/nps.conf
allow_web_terminal=true
```

重启后：

1. 侧栏进入「终端」  
2. 左侧选在线主机，右侧打开 xterm  
3. 快捷命令可增删，持久化到 `conf/quick_cmds.json`  
4. Windows 客户端优先 ConPTY，失败回退管道  

**未开启时：** 打开 `/terminal/index` 会显示说明卡片（如何改配置、如何重启），**不再**是生硬的 `403 Forbidden`；相关 API / WebSocket 仍返回 403。

仅应在可信网络或已开启强认证（如 TOTP）的环境打开。

## 自定义错误页与运维通知

域名 / HTTP 转发失败时，访客看到错误页；服务端可异步推送运维通知。**默认全关**，未配通道时零开销。

### 错误页

```ini
# 使用自定义文件（相对运行目录或绝对路径）
http_error_page_path=conf/error.html
http_error_page_brand=我的穿透
http_error_page_brand_url=https://example.com
```

不配 `http_error_page_path` 时使用内置模板。自定义 HTML **仅支持**下列占位符（其它 `{{...}}` 不会替换）：

| 占位符 | 含义 |
| --- | --- |
| `{{status}}` | HTTP 状态码，如 `502` |
| `{{host}}` | 请求 Host |
| `{{message}}` | 简短说明 |
| `{{time}}` | 服务端时间 |
| `{{brand}}` | 品牌名（来自 `http_error_page_brand`） |
| `{{brand_url}}` | 品牌链接 |

最小示例 `conf/error.html`：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>{{status}} · {{brand}}</title></head>
<body>
  <h1>{{status}}</h1>
  <p>{{message}}</p>
  <p>Host: {{host}} · {{time}}</p>
  <p>powered by <a href="{{brand_url}}">{{brand}}</a></p>
</body>
</html>
```

### 运维通知

触发（可多选通道并行发送；单通道失败只记日志）：

| 事件 | 说明 |
| --- | --- |
| 客户端上线 / 下线 | 受 `notify_min_interval` 去抖；公钥客户端默认不通知（`notify_public_client=false`） |
| 健康检查失败 / 恢复 | 同上 |
| 写出错误页 | 受 `notify_error_page_min_interval` 限流（默认 300s） |

```ini
notify_enable=true
notify_title_prefix=[nps-prod]
notify_min_interval=30
notify_error_page_min_interval=300

# 企微群机器人（任选）
notify_wecom_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=你的key

# Telegram（任选；token 与 chat_id 成对）
notify_telegram_bot_token=123456:ABCDEF
notify_telegram_chat_id=-1001234567890

# 通用 Webhook（任选；JSON POST）
notify_webhook_url=https://hooks.example.com/nps
notify_webhook_header=Authorization: Bearer secret\nX-Source: nps
```

通用 Webhook 正文字段包括：`version`、`type`、`title`、`body`、`time`、`client_id`、`host`、`target`、`addr`、`details`。  
通知通道与错误页**品牌**请在「全局参数」修改（热更新）；仅自定义错误页**模板路径** `http_error_page_path` 改后需重启。  

> 上例 ini 键仍可作为首次迁移种子写入 `global.json`；日常改面板即可。

## WAF 防火墙

默认**全部关闭**（`waf_enable=false`）。开启后可对域名 HTTP 入口、经 `DealClient` 的 TCP 等业务口，以及（可选）管理面板做统一防护。与 ACL / GeoIP 共用 `geoip.dat`，**不互相替代**。

### 能力一览

| 能力 | 作用入口 | 说明 |
| --- | --- | --- |
| UA 黑名单 | 仅 HTTP（域名 + 可选面板） | 命中 User-Agent 子串或正则则 403 |
| 禁止国外访问 | HTTP + TCP 等 | 非允许国家码拒绝；主路径查 `geoip:cn`；私网始终放行 |
| 自定义规则 | HTTP 可按 path/method/header；TCP 仅 IP 相关 | `allow` 短路 / `deny` 拦截，按 `priority`（越小越先） |
| 防护管理面板 | `web_port` | `waf_protect_panel`（默认 `true`）：总开关开启后与业务口共用 `waf.json` |

判定顺序（业务连接）：先既有 ACL，再 WAF。HTTP 匹配顺序：IP 例外 / allow → UA → 禁国外 → deny。

### 启用步骤

1. 准备 Geo（禁国外需要）：`nps -updategeo`，或 **全局参数** 页「更新 Geo」。  
2. 在 **Web 后台 → WAF** 编辑规则并保存（热加载）；也可复制 `conf/waf.json.example` 为 `conf/waf.json`。  
3. 在 **全局参数 → WAF 功能开关** 打开需要的项并保存（立即生效，无需重启）。

### 面板怎么用

侧栏「WAF」（管理员）：

1. 总览：显示当前开关与 Geo 状态；改开关请到「全局参数」。  
2. 一键防护：应用内置场景（扫描器 UA、SEO 采集、WordPress 路径等）。  
3. UA 黑名单 / 访问例外（IP·网段）/ 路径拦截：分块表单，保存即热加载。  
4. 高级：折叠区可编辑原始 JSON。

「只允许国内」场景会提示到全局参数打开「禁止国外访问」（不写进 json）。

### 注意

- WAF **开关**与 `waf.json` 规则均可热更新；仅 `geoip_path` / `waf_rules_path` 改路径需重启。  
- 开禁国外且防护面板时，境外运维务必先加 `ip_allowlist`（访问例外），否则可能锁住面板。仅护业务口时可关「防护管理面板」。  
- 「信任代理真实 IP 头」开启且 nps 直接暴露公网时，访客可伪造 CDN/反代头骗过 ACL/WAF；前面应有可信 CDN/反代，或关闭该开关。  
- 配置项表见 [服务端配置 · WAF / GeoIP](server_config.html#waf-geoip)；场景示例见 [使用示例 · WAF](../extend/example.html#waf-防火墙)。

## 隧道闲置自动停用

适用于公网临时暴露的高风险口（如 SSH）：用时在面板开启，无活动连接满设定分钟后**自动停用**（与手动停用同一路径：关监听、`Status=false` 持久化）。默认关闭（分钟数 `0`）。**无需升级 npc**。

| 项 | 约定 |
| --- | --- |
| 作用范围 | 一期仅 **tcp / udp** 隧道 |
| 闲置定义 | 该隧道上**活动业务连接数为 0** 持续满 N 分钟（不是「没人点面板」） |
| 到期行为 | 立即停用并断开仍在的连接 |
| 开启瞬间无连接 | 从开启时刻起算闲置（避免开了忘用一直挂着） |
| 进程重启 | 按持久化状态恢复；正在跑且已配置分钟数时重新计时（不恢复重启前已走过的分钟） |

### 面板用法（SSH 例）

1. 新建 TCP 隧道，目标如 `127.0.0.1:22`，填写「闲置自动停用（分钟）」如 `60`，平时保持停用。  
2. 需要登录时：隧道列表 → 开启。  
3. SSH 连上期间有活动连接，不会因闲置关掉。  
4. 断开后满 60 分钟无新连接 → 自动停用；下次再用再开。

长驻 SSH 会话会一直占连接，**不会**自动停——这是预期行为。域名转发（Host）本期不做闲置停用。

## 关闭代理

如需关闭 http 代理，将 `http_proxy_port` 留空；如需关闭 https 代理，将 `https_proxy_port` 留空。

## 流量数据持久化
服务端支持将流量数据持久化，默认关闭。在 `nps.conf` 中设置 `flow_store_interval`（单位分钟）即可开启。

**注意：** nps 不会持久化通过公钥连接的客户端。

## 系统信息显示
nps 服务端支持在 web 上显示和统计服务器相关信息。如需开启请在 **Web 后台 → 全局参数** 中开启「系统监控图表」（对应 `conf/global.json` 的 `system_info_display`）。


## 关闭 web 管理
将 `nps.conf` 中的 `web_port` 设置为空或删除。

## 服务端多用户登录
在 **Web 后台 → 全局参数** 中开启「客户端账号登录」（对应 `conf/global.json` 的 `allow_user_login`）。登录用户名 `user`，密码为对应客户端的验证密钥。登录后可进入客户端编辑修改 web 登录用户名密码，默认开启。

## 用户注册功能
在 **Web 后台 → 全局参数** 中开启「允许注册」（对应 `conf/global.json` 的 `allow_user_register`）后，登录页会出现注册入口。

## 监听指定 IP

nps 支持每个隧道监听不同的服务端 IP。在 **Web 后台 → 全局参数** 中开启「一隧道多 IP」（对应 `conf/global.json` 的 `allow_multi_ip`）后，可在 web 中控制，或 npc 配置文件中指定：

```ini
server_ip=xxx
```

## 客户端到期时间

在创建 / 修改客户端时可填写「到期时间」（可留空表示永不过期）。到期后该客户端会被**自动暂停**，所有隧道停止服务，直到管理员手工延长或清空到期时间。
支持格式：`2006-01-02 15:04:05` / `2006-01-02 15:04` / `2006-01-02T15:04:05` / `2006-01-02T15:04` / `2006-01-02`。

## 首次启动随机凭据

为避免默认密码被恶意扫描，**首次启动时** `web_username`（默认 `admin`）、`web_password`、`auth_key`、`auth_crypt_key` 全部随机生成，并直接打印到终端：

```text
[INFO] generated random web_password: <xxxxxxxx>
[INFO] generated random auth_key: <xxxxxxxx>
[INFO] generated random auth_crypt_key: <xxxxxxxxxxxxxxxx>
```

请在第一次启动时妥善记录。后续可在 `nps.conf` 中手工修改。



## 与 nginx 配合

有时需要在云服务器上跑 nginx 做静态缓存或多服务复用 80/443。把 `http_proxy_port` 设为非 80 端口（例如 8010），由 nginx 反代：

```
server {
    listen 80;
    server_name *.proxy.com;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8010;
    }
}
```

如需 https 也由 nginx 终结，将 `https_proxy_port` 留空关闭，由 nginx 监听 443：

```
server {
    listen 443;
    server_name *.proxy.com;
    ssl on;
    ssl_certificate  certificate.crt;
    ssl_certificate_key private.key;
    ssl_session_timeout 5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_prefer_server_ciphers on;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8020;
    }
}
```

## web 管理使用 https
在 `nps.conf` 中设置 `web_open_ssl=true`，并配置 `web_cert_file` 与 `web_key_file`。

## web 使用 Caddy 代理

实现子路径访问 nps，假设想通过 `http://caddy_ip:caddy_port/nps` 访问后台：

```nginx
caddy_ip:caddy_port/nps {
  ##server_ip 为 nps 服务器IP
  ##web_port 为 nps 后台端口
  proxy / http://server_ip:web_port/nps {
	transparent
  }
}
```

`nps.conf` 中设置：
```
web_base_url=/nps
```