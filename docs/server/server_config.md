# 服务端配置文件

配置文件路径：`<nps 可执行文件目录>/conf/nps.conf`


> 首次启动时若 `conf/nps.conf` 不存在会**自动生成默认配置**，其中 `web_username`（默认 `admin`）、`web_password`、`auth_key`、`auth_crypt_key` **均为随机值**并打印到终端，请在启动日志或配置文件中查看。

## 通用配置

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| bridge_type | 客户端与服务端连接方式 `tcp` 或 `kcp` | `tcp` |
| bridge_ip | 桥接监听 IP | `0.0.0.0` |
| bridge_port | 服务端客户端通信端口 | `8024` |
| tls_enable | 是否开启 TLS。开启后客户端可通过 `-tls_enable=true` 连接 `tls_bridge_port` | `true` |
| tls_bridge_port | TLS 桥接端口，默认 `8025`。**与 `bridge_port` 并存**：客户端可分别用 `bridge_port`（明文）或 `tls_bridge_port`（TLS）接入 | `8025` |
| disconnect_timeout | 客户端连接超时，单位为 5s，默认 `60`（即 5 分钟） | `60` |
| flow_store_interval | 流量数据持久化间隔，单位分钟；忽略表示不持久化| `1` |
| log_level | 日志级别 0~7 | `6` |
| log_path | 日志文件路径 | `nps.log` |

## HTTP(S) 代理

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| http_proxy_ip | http(s) 代理监听 IP，默认 `0.0.0.0` | `0.0.0.0` |
| http_proxy_port | 域名代理 http 监听端口（默认 `80`），留空关闭 | `80` |
| https_proxy_port | 域名代理 https 监听端口（默认 `443`），留空关闭。详见 [使用 https](nps_extend.html#使用-https) | `443` |
| http_cache | 是否开启静态文件缓存 | `false` |
| http_cache_length | 缓存条目数上限，`0` 表示不限制 | `100` |


## Web 管理后台

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| web_host | web 管理使用的二级域名，端口复用时区分用 | `a.o.com` |
| web_username | web 后台用户名（默认 `admin` | `admin` |
| web_password | web 后台密码 | 首次启动随机生成 |
| web_port | web 管理端口，留空关闭 web | `8081` |
| web_ip | web 管理监听 IP | `0.0.0.0` |
| web_base_url | web 管理子路径，例如 `/nps`，用于反代到子路径时使用 | （空） |
| web_open_ssl | web 管理是否启用 https | `false` |
| web_cert_file | web 管理 https 证书路径 | `conf/server.pem` |
| web_key_file | web 管理 https 私钥路径 | `conf/server.key` |
| web_welcome | 顶栏环境标识（如「DNS二线」）；空则显示默认欢迎文案 | （空） |
| web_subtitle | 侧栏副标题 | （空） |
| totp_secret | 管理员登录 TOTP 密钥；空=关闭。可用 `nps -gen2fa` 生成后填入并重启 | （空） |
| web_secure_entry | 安全入口路径（如 `/s/随机串`）；空=关闭。配置后须先访问该路径解锁会话，才显示登录页；未解锁访问面板/静态资源一律 404。**不**影响 `/api/v1`。改后需重启 | （空） |

## 运维扩展

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| allow_web_terminal | 是否开放面板 Web 终端（`/terminal/index`）。关闭时页面显示说明，接口仍拒绝 | `false` |
| http_error_page_path | 自定义 HTTP 错误页文件路径；空则用内置模板 | （空） |
| http_error_page_brand | 错误页品牌名 | `nps` |
| http_error_page_brand_url | 错误页品牌链接 | （空） |
| notify_enable | 是否启用运维通知（企微 / Telegram / Webhook） | `false` |
| notify_title_prefix | 通知标题前缀 | `[nps]` |
| notify_min_interval | 同类事件最短间隔（秒），用于客户端上下线 / 健康事件去抖 | `30` |
| notify_error_page_min_interval | 写出错误页触发通知的最短间隔（秒） | `300` |
| notify_queue_size | 通知异步队列长度；满则丢弃并打日志 | `64` |
| notify_http_timeout | 通道 HTTP 超时（秒） | `5` |
| notify_public_client | 是否对「公钥客户端」也发上下线通知 | `false` |
| notify_wecom_webhook | 企业微信群机器人 Webhook URL | （空） |
| notify_telegram_bot_token | Telegram Bot Token | （空） |
| notify_telegram_chat_id | Telegram Chat ID（可与 Bot Token 成对） | （空） |
| notify_webhook_url | 通用 Webhook URL（JSON POST） | （空） |
| notify_webhook_header | 额外请求头，多行 `Name: Value`（可用 `\n` 写在一行） | （空） |

> 上述运维项修改后需**重启** nps。`notify_enable=false` 或未配置任何通道时接近零开销。完整示例见 [增强功能](nps_extend.html#自定义错误页与运维通知) 与 [使用示例](../extend/example.html#运维通知示例)。

## API 鉴权

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| auth_key | web API 鉴权密钥，详见 [Web API](../extend/api.html) | 首次启动随机生成 |
| auth_crypt_key | `auth/getauthkey` 接口的 AES 加密密钥，**必须 16 位** | 首次启动随机生成 |

## P2P

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| p2p_ip | 服务端 IP，使用 p2p 模式必填 | - |
| p2p_port | p2p 模式开启的 UDP 起始端口。若设为 `6000`，请额外开放 `6001`、`6002` | - |
| p2p_holepunch_port | 可选 holepunch 协调端口；`0` 或不配表示不启用 | `0` |

## 全局参数（原限制开关）

以下配置已从 `nps.conf` 迁移到 `conf/global.json`，请在 **Web 后台 → 全局参数** 中修改，保存后立即生效：

`appname`、`web_welcome`/`web_subtitle`、`allow_*`、`ip_limit`、`open_captcha`、`allow_x_real_ip`、`allow_remote_update`、`allow_web_terminal`、`waf_*` 开关、`register_*`、`smtp_*`、`notify_*`、`max_*`、`release_*`、错误页品牌等（完整表见 [全局参数](nps_extend.html#全局参数)）

完整字段说明见 [全局参数](nps_extend.html#全局参数)。

## WAF / GeoIP

> WAF **功能开关**在「全局参数」热更新；规则文件 `waf.json` 由面板「WAF」页保存并热加载；`geoip_path` / `waf_rules_path` 改路径仍需重启。完整用法见 [增强功能 · WAF 防火墙](nps_extend.html#waf-防火墙)。

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| waf_enable | WAF 总开关；关则整套逻辑旁路 | `false` |
| waf_protect_panel | 总开关开启时是否对管理面板套用同一套规则 | `true` |
| waf_ua_blacklist_enable | 是否启用 UA 黑名单（仅 HTTP） | `false` |
| waf_block_foreign_enable | 是否禁止国外访问（依赖 GeoIP / 可信国家头） | `false` |
| waf_custom_rules_enable | 是否启用 `waf.json` 自定义规则 | `false` |
| waf_block_foreign_unknown | 禁国外时，国家未知是否拒绝 | `true` |
| waf_country_headers | 可信反代国家头名，逗号分隔（仅信任转发头时读取） | `CF-IPCountry` |
| waf_rules_path | 规则文件路径 | `conf/waf.json` |
| geoip_path | GeoIP 离线库路径（与 ACL / WAF 共用） | `conf/geoip.dat` |
| geosite_path | GeoSite 库路径（主要给 ACL `geosite:`） | 同目录 `geosite.dat` |

可用 `nps -updategeo` 或面板「全局参数 → 更新 Geo」拉取数据。域名 HTTP 真实 IP 由 `allow_x_real_ip`（全局参数）控制，内置多 CDN/反代头顺序，**无**单独的头列表配置项。

## 审计与其它可选开关

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| audit_log_path | 管理面 / 连接审计日志路径；空=关闭。相对路径相对运行目录 | （空） |
| audit_connection | 是否记录 npc 上下线（需同时配置 `audit_log_path`） | `false` |
| audit_connection_min_interval | 同一 client+动作最短写间隔（秒）；`0`=不去抖 | `30` |
| allow_remote_update | 面板是否允许对在线客户端发起远程升级 | `false` |
| register_email_verify | 注册是否要求邮箱验证码（需配 SMTP） | `false` |
| release_api_url | 自建更新检查 API；空=GitHub `cloudwebsite/nps`。国内访问 GitHub 困难时见 [自建更新源](../extend/release_mirror.html) | （空） |
| release_download_base | 自建下载基址（与 API 成对配置） | （空） |

## debug / pprof

| 名称 | 含义 | 默认值 |
| --- | --- | --- |
| pprof_ip | debug pprof 监听 IP | - |
| pprof_port | debug pprof 监听端口 | - |

## 配置片段示例（复制后按需取消注释）

```ini
# --- 面板展示 ---
# web_welcome=DNS二线
# web_subtitle=内网穿透控制台

# --- 管理员 TOTP（nps -gen2fa 生成）---
# totp_secret=

# --- 安全入口（空=关闭；请用强随机串，勿提交真实密钥到仓库）---
# web_secure_entry=/s/请换成长随机串

# --- 面板终端（默认关）---
# allow_web_terminal=false

# --- 自定义错误页 ---
# http_error_page_path=conf/error.html
# http_error_page_brand=nps
# http_error_page_brand_url=https://github.com/cloudwebsite/nps

# --- 运维通知（默认全关；至少配一个通道）---
# notify_enable=true
# notify_title_prefix=[nps]
# notify_min_interval=30
# notify_error_page_min_interval=300
# notify_wecom_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx
# notify_telegram_bot_token=
# notify_telegram_chat_id=
# notify_webhook_url=https://example.com/hooks/nps
# notify_webhook_header=Authorization: Bearer token

# --- WAF / GeoIP（默认全关；waf_protect_panel 默认开）---
# waf_enable=false
# waf_protect_panel=true
# waf_ua_blacklist_enable=false
# waf_block_foreign_enable=false
# waf_custom_rules_enable=false
# waf_block_foreign_unknown=true
# waf_country_headers=CF-IPCountry
# geoip_path=conf/geoip.dat
# geosite_path=conf/geosite.dat
# waf_rules_path=conf/waf.json
```
