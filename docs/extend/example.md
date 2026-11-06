# 使用示例
## 统一准备工作
- 开启服务端，假设公网服务器ip为1.1.1.1，配置文件中`bridge_port`为8024，配置文件中`web_port`为8080
- 访问1.1.1.1:8080
- 在客户端管理中创建一个客户端，记录下验证密钥
- 内网客户端运行（windows使用cmd运行加.exe）

```shell
./npc -server=1.1.1.1:8024 -vkey=客户端的密钥
```
**注意：运行服务端后，请确保能从客户端设备上正常访问配置文件中所配置的`bridge_port`端口，telnet，netcat这类的来检查**

## 域名解析

**适用范围：** 小程序开发、微信公众号开发、产品演示

**注意：域名解析模式为http反向代理，不是dns服务器，在web上能够轻松灵活配置**

**假设场景：**
- 有一个域名proxy.com，有一台公网机器ip为1.1.1.1
- 两个内网开发站点127.0.0.1:81，127.0.0.1:82
- 想通过（http|https://）a.proxy.com访问127.0.0.1:81，通过（http|https://）b.proxy.com访问127.0.0.1:82

**使用步骤**
- 将*.proxy.com解析到公网服务器1.1.1.1
- 点击刚才创建的客户端的域名管理，添加两条规则规则：1、域名：`a.proxy.com`，内网目标：`127.0.0.1:81`，2、域名：`b.proxy.com`，内网目标：`127.0.0.1:82`

现在访问（http|https://）`a.proxy.com`，`b.proxy.com`即可成功

**https:** 如需使用https请进行相关配置，详见 [使用https](../server/nps_extend.html#使用-https)

## tcp隧道


**适用范围：**  ssh、远程桌面等tcp连接场景

**假设场景：**
 想通过访问公网服务器1.1.1.1的8001端口，连接内网机器10.1.50.101的22端口，实现ssh连接

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条tcp隧道，填写监听的端口（8001）、内网目标ip和目标端口（10.1.50.101:22），保存。
- 访问公网服务器ip（1.1.1.1）,填写的监听端口(8001)，相当于访问内网ip(10.1.50.101):目标端口(22)，例如：`ssh -p 8001 root@1.1.1.1`

## udp隧道

**适用范围：**  内网dns解析等udp连接场景

**假设场景：**
内网有一台dns（10.1.50.102:53），在非内网环境下想使用该dns，公网服务器为1.1.1.1

**使用步骤**
- 在刚才创建的客户端的隧道管理中添加一条udp隧道，填写监听的端口（53）、内网目标ip和目标端口（10.1.50.102:53），保存。
- 修改需要使用的dns地址为1.1.1.1，则相当于使用10.1.50.102作为dns服务器

## socks5代理


**适用范围：**  在外网环境下如同使用vpn一样访问内网设备或者资源

**假设场景：**
想将公网服务器1.1.1.1的8003端口作为socks5代理，达到访问内网任意设备或者资源的效果

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条socks5代理，填写监听的端口（8003），保存。
- 在外网环境的本机配置socks5代理(例如使用proxifier进行全局代理)，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8003)，即可畅享内网了

**注意**
经过socks5代理，当收到socks5数据包时socket已经是accept状态。表现是扫描端口全open，建立连接后短时间关闭。若想同内网表现一致，建议远程连接一台设备。

## http正向代理

**适用范围：**  在外网环境下使用http正向代理访问内网站点

**假设场景：**
想将公网服务器1.1.1.1的8004端口作为http代理，访问内网网站

**使用步骤**

- 在刚才创建的客户端隧道管理中添加一条http代理，填写监听的端口（8004），保存。
- 在外网环境的本机配置http代理，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8004)，即可访问了

**注意：对于私密代理与p2p，除了统一配置的客户端和服务端，还需要一个客户端作为访问端提供一个端口来访问**

## 私密代理

**适用范围：**  无需占用多余的端口、安全性要求较高可以防止其他人连接的tcp服务，例如ssh。

**假设场景：**
无需新增多的端口实现访问内网服务器10.1.50.2的22端口

**使用步骤**
- 在刚才创建的客户端中添加一条私密代理，并设置唯一密钥secrettest和内网目标10.1.50.2:22
- 在需要连接ssh的机器上以执行命令

```
./npc -server=1.1.1.1:8024 -vkey=vkey -type=tcp -password=secrettest -local_type=secret
```
如需指定本地端口可加参数`-local_port=xx`，默认为2000

**注意：** password为web管理上添加的唯一密钥，具体命令可查看web管理上的命令提示

假设10.1.50.2用户名为root，现在执行`ssh -p 2000 root@127.0.0.1`即可访问ssh


## p2p服务

**适用范围：**  大流量传输场景，流量不经过公网服务器，但是由于p2p穿透和nat类型关系较大，不保证100%成功，支持大部分nat类型。[nat类型检测](../client/npc_extend.html#nat类型检测)

**假设场景：**

想通过访问使用端机器（访问端，也就是本机）的2000端口---->访问到内网机器 10.2.50.2的22端口

**使用步骤**
- 在`nps.conf`中设置`p2p_ip`（nps服务器ip）和`p2p_port`（nps服务器udp端口）
> 注：若 `p2p_port` 设置为6000，请在防火墙开放6000~6002(额外添加2个端口)udp端口
- 在刚才刚才创建的客户端中添加一条p2p代理，并设置唯一密钥p2pssh
- 在使用端机器（本机）执行命令

```
./npc -server=1.1.1.1:8024 -vkey=123 -password=p2pssh -target=10.2.50.2:22
```
如需指定本地端口可加参数`-local_port=xx`，默认为2000

**注意：** password为web管理上添加的唯一密钥，具体命令可查看web管理上的命令提示

假设内网机器为10.2.50.2的ssh用户名为root，现在在本机上执行`ssh -p 2000 root@127.0.0.1`即可访问机器2的ssh，如果是网站在浏览器访问127.0.0.1:2000端口即可。

## 多实例面板环境标识

**适用范围：** 同一运维同时管理多套 nps（测试 / 生产 / 分站）

```ini
# 生产
web_welcome=生产-华东
web_subtitle=内网穿透 · 生产

# 测试（另一台机器另一份 conf）
web_welcome=测试机
web_subtitle=内网穿透 · 测试
```

重启后顶栏显示环境名，侧栏显示副标题，避免进错面板。

## 管理员开启 TOTP

**适用范围：** 管理口暴露公网，需二次验证

```text
nps -gen2fa
```

把输出的 `totp_secret=...` 写入 `nps.conf`，用手机 Authenticator 扫描打印的 `otpauth://` URI，重启 nps。之后管理员登录需额外输入 6 位动态码。

关闭：删除或清空 `totp_secret=` 后重启。

## 开启安全入口

**适用范围：** 管理口在公网，希望未持有密钥路径时连登录页都扫不到

```ini
web_secure_entry=/s/请换成长随机串
```

重启后：

1. 直接打开 `/login/index` → **404**  
2. 浏览器先访问 `/s/请换成长随机串` → 自动跳转登录页  
3. 之后同会话内可正常使用面板；`/api/v1` 不受影响  
4. 可与 `totp_secret` 同时开启  

关闭：清空或删除 `web_secure_entry=` 后重启。请勿把真实随机串写进公开文档或仓库。

## 开启面板终端

**适用范围：** 运维需要在浏览器里进内网主机 Shell（务必配合可信网络或 TOTP）

```ini
allow_web_terminal=true
```

重启 → 侧栏「终端」→ 选择在线客户端 → 操作。快捷命令会写入 `conf/quick_cmds.json`。

未开启时进入终端页只会看到配置说明，不会出现空白 403 页。

## 自定义错误页

**适用范围：** 域名反代目标宕机时，对外展示统一品牌页

1. 新建 `conf/error.html`（示例见 [增强功能 · 错误页](../server/nps_extend.html#错误页)）  
2. `nps.conf`：

```ini
http_error_page_path=conf/error.html
http_error_page_brand=ACME穿透
http_error_page_brand_url=https://www.example.com
```

3. 重启 nps；故意访问一个目标不可达的域名，应看到自定义页。

## 运维通知示例

**适用范围：** 客户端掉线、健康检查失败、访客打到错误页时推到群

### 仅企微

```ini
notify_enable=true
notify_wecom_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=你的key
```

### 企微 + Telegram

```ini
notify_enable=true
notify_title_prefix=[nps-prod]
notify_wecom_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=你的key
notify_telegram_bot_token=123456:ABCDEF
notify_telegram_chat_id=-1001234567890
```

### 通用 Webhook

```ini
notify_enable=true
notify_webhook_url=https://hooks.example.com/nps
notify_webhook_header=Authorization: Bearer secret
```

重启后让一台 npc 上下线，或打一个会出错误页的域名，群里应收到消息。错误页通知默认最少间隔 300 秒，避免刷屏。

## WAF 防火墙

**适用范围：** 拦扫描器 UA、WordPress 路径探测；可选只允许国内 IP 访问业务口 / 面板

1. 更新 Geo（禁国外需要）：`nps -updategeo`  
2. 面板侧栏「WAF」→ 应用「拦扫描器 UA」等场景，或自行加路径 / IP 例外 → 保存  
3. `nps.conf`（或服务端配置页）开启后重启：

```ini
waf_enable=true
waf_protect_panel=true
waf_ua_blacklist_enable=true
waf_block_foreign_enable=false
waf_custom_rules_enable=true
```

4. 用异常 UA 或命中路径访问域名，应返回 `403 Forbidden`。  
5. 若同时开禁国外且人在境外：先在「访问例外」加办公网 IP，或设 `waf_protect_panel=false` 仅护业务口。

完整说明见 [增强功能 · WAF](../server/nps_extend.html#waf-防火墙)。

## 隧道闲置自动停用（SSH）

**适用范围：** 公网临时开 SSH，用完或闲置后自动收回暴露面

1. 建 TCP 隧道：服务端端口如 `22022`，目标 `127.0.0.1:22`，「闲置自动停用」选 `60` 分钟；保存后保持**停用**。  
2. 需要登录时：隧道列表 → 开启 → `ssh -p 22022 user@公网IP`。  
3. 断开 SSH 后满 60 分钟无新连接 → 隧道自动停用；下次再开。

说明见 [增强功能 · 隧道闲置自动停用](../server/nps_extend.html#隧道闲置自动停用)。
