# 更新日志
- 2026-11-05  v0.26.41.1（重构融合版）
  - 新增：
    - **升级与兼容**
      - 配置只增迁移：升级启动时自动为 `nps.conf` / `global.json` 补齐新版本新增的配置项，保留原有注释与取值，不再「升级丢配置」
      - 数据文件向前兼容：`clients.json` / `tasks.json` / `hosts.json` 增字段不影响旧文件加载，vkey 与流量数据保持不变
      - 旧客户端兼容：核心握手版本保持 `0.26.0`，先升级服务端，旧版 npc 仍可正常连接
    - **安全加固**
      - 面板密码加密存储：管理员 / 用户密码改用 bcrypt 保存，旧明文密码在首次登录成功后自动升级为密文，原密码继续可用
      - 登录会话硬化：登录成功即更换会话 ID；Cookie 加 `SameSite=Strict`，HTTPS 下再加 `Secure`；会话闲置默认 2 小时失效
      - 权限归属校验：普通用户只能管理自己名下的客户端 / 隧道 / 域名 / 终端，伪造 `client_id` 一律拒绝
      - 隧道模式白名单：仅允许面板公开的隧道模式，避免异常模式写入导致服务端异常
      - 全局资源上限：新增 `max_clients`、`max_tunnels_per_client`、`max_global_conn`（默认 `0` = 不限制）
      - 桥接 TLS 加固：证书页展示桥接证书 SHA-256 指纹；客户端可用 `tls_fingerprint` 校验、`tls_strict` 强制（默认关闭）
      - 管理员 TOTP 双因素登录：开启后登录面板需再输入动态验证码，可用 `nps -gen2fa` 生成密钥（`totp_secret`，默认关闭）
      - 面板安全入口：`web_secure_entry` 隐藏登录页，未带正确路径访问不显示登录表单（默认关闭）
    - **传输内核**
      - 多协议桥接旁路：在原有 TCP / TLS 基础上，可选开启 KCP / WebSocket（WS / WSS）端口（`bridge_kcp_port` / `bridge_ws_port` / `bridge_wss_port`，未配置则不监听，不影响旧 npc）
      - 安全挑战握手：可选 `bridge_secure_handshake` 时间窗与防重放（默认关闭，阈值 `0.26.41`）
      - 一 vkey 多节点：`bridge_select_mode=primary|roundrobin|random`，同密钥多节点按策略转发
      - 现代 P2P：`p2p_mode=legacy|stun|modern`，`modern` 走 STUN 打洞后 QUIC，可选 `p2p_holepunch_port` 协调器（默认 legacy）
      - 客户端多服务器：`npc -server=a,b -vkey=... -type=tcp,tls` 支持多入口并行重连
      - 远程升级客户端：管理员可对在线客户端下发升级（`allow_remote_update`，默认关闭；旧客户端不识别该指令）
      - 自建下载源：`release_api_url` / `release_download_base` 可指向自建源（留空 = GitHub cloudwebsite/nps）
    - **代理与域名**
      - HTTP/3 域名反代：可选 QUIC 入口（`http3_proxy_port`，默认不监听）
      - TLS 直通 / SNI 终止：域名可配 `HttpsJustProxy`，按 SNI 原样转发或解密后再转发
      - 自动证书：支持 Let's Encrypt 与 ZeroSSL 自动申请 / 续签（`ssl_email` / `ssl_path` / `force_auto_ssl` 与域名 `AutoSSL`，默认关闭）
      - 混合代理：新增 `mixProxy` 模式，同一端口同时承载 HTTP 正向代理与 SOCKS5
      - 嵌套转发：隧道目标支持 `tunnel://`、`bridge://web`、`file://`（需本地代理与全局开关）
      - 域名增强：支持 CORS、请求 / 响应头改写、路径前缀重写，健康检查更完善（HTTPS、恢复清零等）
      - 真实 IP：隧道可选 Proxy Protocol v1/v2；域名 HTTP 在 `allow_x_real_ip` 下按内置顺序读取 CF/阿里/EdgeOne 等 CDN 头及 `X-Real-IP` / `X-Forwarded-For` / RFC `Forwarded`
      - ACL 与 GeoIP：IP / CIDR 黑白名单；GeoIP 离线库与在线更新（`nps -updategeo` 或「全局参数」页），与 WAF 共用数据
      - 隧道闲置自动停用：tcp/udp 可设闲置分钟数，无活动连接满 N 分钟后自动停用（适合 SSH 等临时开放；默认关）
      - WAF 防火墙：UA 黑名单、禁止国外访问、自定义规则；面板「WAF」页分块编辑规则并热加载；可选防护管理面板（`waf_protect_panel`，默认开；总开关默认关）
    - **多用户 / API / 审计**
      - 用户体系：独立用户表与客户端归属，支持客户端 / 隧道配额与到期时间（无 `users.json` 时行为与旧版一致）
      - 统一 API：新增 `/api/v1`，覆盖客户端、隧道、域名、用户、全局参数、Geo、审计、状态；旧 `/auth` 接口保留
      - 操作与连接审计：可开启 `audit_log_path` 与 `audit_connection`（默认关闭，30 秒去抖），管理员侧栏「审计」页可查看 / 筛选 / 搜索
      - 注册增强：可开启自助注册、注册限频与验证码，及 SMTP 邮箱验证码（默认关闭）
    - **面板与配置**
      - 管理面板全新界面：新增浅色 / 深色主题，一键切换；登录、注册页同步改版；列表与表单更清爽，操作更顺手
      - 顶栏环境标识与侧栏副标题：可自定义面板顶栏名称（如「DNS二线」）和侧栏副标题，多台服务器一眼区分（`web_welcome` / `web_subtitle`，改后重启）
      - 服务端配置页：管理员可在面板按分组修改 `nps.conf`（白名单键、敏感项脱敏、保存前自动备份、提示是否需重启）
      - 证书管理页：上传 / 粘贴 / 校验证书；浏览 `conf/webssl`、`conf/bridgessl`、`conf/certs`（颁发者 / 到期 / 指纹），可绑定面板或 Bridge、删除未引用文件
      - Web 终端：可开启面板内设备终端（`allow_web_terminal`，默认关闭），快捷命令持久化、Windows 优先 ConPTY
      - Shadowsocks 隧道：新增 `shadowsocks` 模式（SS2022），面板可一键生成 PSK
      - 自定义错误页与运维通知：可自定义访问失败时的错误页；客户端上下线、健康检查异常、访客访问到错误页时可推送通知，支持企业微信 / Telegram / 通用 Webhook（默认关闭，`http_error_page_*` / `notify_*`）
      - 语言串补齐：修复面板中英切换时若干「Missing language string」，侧栏与列表标题、隧道模式等统一走语言包
    - **客户端与运维**
      - 桌面 GUI 双发：Fyne（`gui/npc`，含 Android）与 Wails（`cmd/npc/npc-gui`）正式双发，TLS / 指纹 / P2P / holepunch 能力对齐
  - 优化：
    - 未启用面板终端时展示说明页，不再直接返回生硬的 403
    - 列表操作按钮统一为图标按钮，界面更紧凑；语言下拉仅保留带国旗的完整语言项；修复表单折叠后「新增 / 保存」按钮被隐藏的问题
  - 说明：
    - 核心握手版本仍为 `0.26.0`；展示版本 `0.26.41.1`

- 2026-11-02  v0.26.40
  - 新增：
    - 全局参数支持热更新：`nps.conf` 中的 `allow_ports`、`ip_limit`、`show_http_proxy_port`、`http_add_origin_header`、`allow_flow_limit`、`allow_rate_limit`、`allow_connection_num_limit`、`allow_tunnel_num_limit`、`allow_multi_ip`、`allow_local_proxy`、`allow_user_login`、`allow_user_register`、`allow_user_change_username`、`open_captcha`、`system_info_display` 迁移到 `conf/global.json`，新生成的 `nps.conf` 不再包含这些参数；升级时首次启动自动迁移，Web 后台保存后立即生效；「全局参数」页面同时按基础设置、安全设置、客户端能力、登录与账号、展示与高级分组重构

      <img src="/image/new/setting.png" alt="全局参数" style="max-width: 620px; width: 100%;" />
    - 新增客户端、隧道、域名解析必填校验，并兼容隧道各模式下的动态字段
    - 客户端交互菜单新增「查看系统服务」，可列出本机已注册的 `nps-client-<vkey>` 服务及运行状态；「更新客户端」自动重启已注册的客户端服务

      <img src="/image/new/npc_service.png" alt="查看系统服务" style="max-width: 420px; width: 100%;" />
    - Web 管理首页自动检测服务端新版本，并提示当前版本与最新版本
    - Web 管理端客户端、隧道、域名列表每页条数支持 `20/50/100/200`，并记住用户选择 [#363](https://github.com/yisier/nps/issues/363)
  - 优化：
    - 新增/编辑客户端、隧道、域名解析表单改为紧凑分组布局，减少页面高度并修复字段对齐
    - 优化 `nps_mux.receiveWindowQueue.Pop`，避免并发释放后持续空转 [#364](https://github.com/yisier/nps/pull/364)
  - 修复：
    - 客户端、隧道、域名解析 ID 在删除后继续增长导致跳跃的问题 [#362](https://github.com/yisier/nps/issues/362)

- 2026-10-10  v0.26.39
  - 修复：
    - 同时开启 `web_base_url` 与 `open_captcha` 时，验证码图片路径未跟随 `web_base_url`，反向代理到子路径后无法登录 [#359](https://github.com/yisier/nps/pull/359)
    - Web 面板语言机制：浏览器语言按前缀归一化匹配（`zh*`→`zh-CN`、`en*`→`en-US`），匹配不到时回退默认 `en-US`；用户手动切换语言后长期持久化，不再因关闭浏览器而失效 [#356](https://github.com/yisier/nps/issues/356)
    - SOCKS5（私密代理）UDP ASSOCIATE 回复的中继地址 `BND.ADDR` 优先使用客户端实际连接到的服务端地址，避免外网 IP 探测被运营商 HTTP 劫持返回错误 IP 导致 UDP 不通；外网探测改为 HTTPS 端点并加超时与校验 [#355](https://github.com/yisier/nps/issues/355)
    - 开启 `web_open_ssl=true` 后以系统服务方式启动时 Web 面板无法访问：证书相对路径按进程工作目录解析失败，改为基于 nps 运行目录解析 `web_cert_file` / `web_key_file` [#353](https://github.com/yisier/nps/issues/353)
    - `nps` 服务端升级后自动重启服务 [#347](https://github.com/yisier/nps/issues/347)

- 2026-08-24  v0.26.38
  - 新增：
    - GUI 客户端（npc-gui）迁移到 Wails v3
    - GUI 客户端新增版本自动更新：启动时自动检查 GitHub 最新 release，支持一键下载并热替换当前可执行文件后自动重启  
    ![img.png](/image/new/gui.png)
  - 修复：
    - 客户端内网地址为 192.169.x.x 等非标准私网段时，Web 面板客户端列表不显示本机 IP [#348](https://github.com/yisier/nps/issues/348)

- 2026-08-08  v0.26.37
  - 修复：
    - NPC 安装到系统服务失败 BUG [#338](https://github.com/yisier/nps/issues/338)  
    - 隧道实际未运行但面板提示「启动成功」、运行状态始终为关闭的问题 [#339](https://github.com/yisier/nps/issues/339)
    - 配置 `allow_ports` 后随机生成端口问题 [#339](https://github.com/yisier/nps/issues/339)
    - Web 面板`languages.xml`使用 NPS 版本号作为缓存标识 [#342](https://github.com/yisier/nps/issues/342)
  - 新增：
     - 客户端、隧道、域名解析修改接口支持修改流量数据 [#341](https://github.com/yisier/nps/issues/341)

- 2026-07-12  v0.26.36
  - 新增：
    - 新增客户端内网地址上报功能,客户端列表展示 npc 内网地址 [#333](https://github.com/yisier/nps/issues/333)
    - Web 管理端客户端/隧道/域名列表业务字段支持服务端排序（备注、状态、在线等）[#254](https://github.com/yisier/nps/issues/254)
  - 优化：
    - 不再依赖外部 `web` 目录的静态文件
  - 修复：
    - 客户端/服务端在线更新失败 BUG
    - `nps -server` 安装成功后 `web_port` 未正常显示BUG [#317](https://github.com/yisier/nps/issues/317)
    - Bridge 握手增加 10s 读超时，校验失败/读失败立即关闭连接，减轻端口扫描导致的连接与 goroutine 堆积 [#281](https://github.com/yisier/nps/issues/281)
    - 解决私密代理和P2P连接的TLS启用问题 [#143](https://github.com/yisier/nps/issues/143)
    - HTTP 正向代理 Basic 认证 BUG[#133](https://github.com/yisier/nps/issues/133)
    - SOCKS5 用户名密码认证 BUG [#133](https://github.com/yisier/nps/issues/133)
    - 客户端建连失败后半连接假死、需手动重启 npc BUG [#115](https://github.com/yisier/nps/issues/115)

- 2026-07-04  v0.26.35
  - 新增：
    - 客户端列表、隧道列表、域名解析列表支持多选批量删除 [#287](https://github.com/yisier/nps/issues/287)
  - 修复：
    - HTTP 代理在 SVN 客户端无法正常检出 [#336](https://github.com/yisier/nps/issues/336)
    - 新增/编辑隧道时无提示且不跳转，重复点击会产生多条记录 [#334](https://github.com/yisier/nps/issues/334)
    - mips/mipsle 架构运行出现 Illegal instruction 错误 [#331](https://github.com/yisier/nps/issues/331)
  - 优化：
    - 隧道、域名解析编辑页的客户端 ID 改为下拉选择 [#334](https://github.com/yisier/nps/issues/334)

- 2026-06-04  v0.26.34
  - 新增：
    - `nps -server` 管理脚本欢迎语显示当前版本号,更新版本时比对版本，已是最新则直接退出，避免无谓覆盖 [#328](https://github.com/yisier/nps/issues/328)
    - `npc` 管理脚本欢迎语显示当前版本号，新增 `[5] 更新客户端` 选项，更新并替换 npc 二进制
    - 客户端新增到期时间字段，创建/修改客户端时可选填写，到期后自动暂停客户端 [#322](https://github.com/yisier/nps/issues/322)
  - 修复：
    - Dashboard IO 速率采集阻塞请求 500ms，改为后台采集缓存，消除 Sleep 等待
    - ServerStatus 切片并发读写不安全，添加 RWMutex 保护
    - flowSession goroutine 泄漏，改为仅在 StartNewServer 启动一次
    - GetTunnel 双重遍历 sync.Map 性能问题，合并为单次 Range
    - Bridge Client 字段并发读写不安全，添加 Mutex 保护
    - TLS ClientHello recordLen 无上限，添加 16KB 限制防止内存耗尽
    - SOCKS5 地址解析未检查 Read 返回值，改用 io.ReadFull
    - 文件存储 panic 改为错误日志，defer 清理临时文件防止泄漏
    - GenerateServerPort 可能无限循环，限制 1000 次重试
    - P2P UDP 无超时永久阻塞，添加 30s ReadDeadline
    - P2P goroutine 退出控制改用 context，修复潜在泄漏
    - 客户端 UDP ReadFrom 错误后未退出，修复无效数据继续处理
    - [#324](https://github.com/yisier/nps/issues/324) 隧道/域名解析/UDP 流量始终为 0
    - muxPackager buffer pool 泄漏：UnPack() 错误路径未归还 windowBuff 导致内存持续增长
    - bridge 客户端重连时旧 WORK_CHAN/WORK_FILE mux 未关闭，底层连接和 goroutine 永不释放
    - pmux 连接超时泄漏：process() channel 发送超时后连接未关闭，且 ACCEPT_TIME_OUT 单位错误（10ns 应为 10s）
    - pmux 关闭流程 panic：加 done channel + WaitGroup 保证 Close() 等所有 process() 退出后再 close conn channel；process() 入口设 ReadDeadline 防止阻塞读导致 wg.Wait() 死锁；PortListener 加 done channel 唤醒 Accept()
  - 优化：ioutil.WriteFile → os.WriteFile、rand.Seed → rand.New 本地随机源

- 2026-05-23  v0.26.33
  - 新增：
    - 配置文件自动生成：启动时若 conf 目录或 nps.conf 不存在，自动创建并写入默认配置，方便 Docker 部署
    - 首次启动默认 web_username/admin、web_password、auth_key、auth_crypt_key 均改为随机生成，并打印到终端，提升安全性
    - Web 静态文件打包进可执行文件 + 静态文件 URL 加入版本号参数，升级后自动刷新缓存，部署无需单独拷贝 web 目录
    - 精简发布包：去除打包时冗余的 conf/nps.conf、web 目录
    - 清理失效配置项：移除 appname、runmode、https_default_cert_file、https_default_key_file、https_just_proxy
  - 修复：
    - [#184](https://github.com/yisier/nps/issues/184) 限速导致隧道中断：UpdateClient 在 RateLimit=0 时创建零速率令牌桶导致永久阻塞
    - [#159](https://github.com/yisier/nps/issues/159) 关闭隧道端口仍可用：StopServer 错误时提前返回未更新状态 + 重复 OpenTask case 导致隧道被自动重启
    - [#170](https://github.com/yisier/nps/issues/170) TCP 负载均衡只有一个后端可达：GetRandomTarget 未处理 \r 换行符和尾部空行
    - [#319](https://github.com/yisier/nps/issues/319) 客户端列表不显示实时网速：NowRate 计算逻辑错误，改为统计每秒实际消耗字节数
    - [#306](https://github.com/yisier/nps/issues/306) [#316](https://github.com/yisier/nps/issues/316) TCP Basic 认证与域名解析冲突：移除 TCP 隧道 Basic 认证探测，域名解析继续使用客户端 Basic 认证

- 2026-03-27  v0.26.32
  - 修复：
    - 客户端注册参数未正确处理[快捷启动命令]和[TLS快捷启动命令] [#303](https://github.com/yisier/nps/issues/303)
    - 修复https反向代理bug，感谢[okawao](https://github.com/okawao) 的 PR

- 2026-03-23  v0.26.31
  - 新增：
    - 域名解析记录开关功能 [#314](https://github.com/yisier/nps/issues/314)
    - TCP隧道增加Basic认证 [306](https://github.com/yisier/nps/issues/306)
  - 修复：
    - 新增隧道出现secret mode keys must be unique [301](https://github.com/yisier/nps/issues/301)

- 2026-03-12  v0.26.30
  - 修复：
    - http 协议 websocket 错误，感谢[xiaozonglong](https://github.com/xiaozonglong) 提供的 home assistant 环境 ,[#268](https://github.com/yisier/nps/issues/268),[#296](https://github.com/yisier/nps/issues/296)
    - 域名解析中，主机url点击无法跳转非80端口 [311](https://github.com/yisier/nps/issues/311)
    - 新增的主机配置页面，客户端ID异常 [#310](https://github.com/yisier/nps/issues/310)
    - 自动 HTTPS(301) 按照 `https_proxy_port` 配置的端口号跳转

- 2026-01-24 v0.26.29
  - 新增：
    - GUI 客户端，基于 Wails 开发，[点击查看了解更多](https://github.com/cloudwebsite/nps/blob/master/cmd/npc/npc-gui/README.md)，需要 WebView2 运行时，可通过两种方式添加客户端。
      - 方式1：直接输入快捷启动命令即可
      - 方式2：手动添加客户端，输入-server -vkey等参数

      ![img.png](/image/new/gui.png)
  - 修复：
    - Android 包未构建到 Release [72](https://github.com/yisier/nps/issues/72)

- 2025-12-06 v0.26.28
  - 【全局参数】页面新增`服务地址`配置项，用于客户端命令地址显示、隧道访问地址显示 [293](https://github.com/yisier/nps/issues/293)
  - 【IP授权功能优化】，通过穿透的端口去提交 IP 认证，感谢 [Aqamoe](https://github.com/Aqamoe) 的建议 [291](https://github.com/yisier/nps/issues/291)
  - 【重构限速器实现】提升性能和准确性

- 2025-11-05 v0.26.27
  - 新增：
    - 客户端新增【IP白名单】，可通过两种方式添加IP：
      - 方式1️⃣：在【客户端】新建/修改页面，配置IP授权密码，不在IP白名单的外网访问将被跳转到 IP 授权页面，输入正确的授权密码，即可完成添加 IP。
      - 方式2️⃣：在【客户端】新建/修改页面，直接添加IP到白名单中。

      ![img.png](/image/new/ip.png)
    - 客户端配置移除【压缩】和【加密】功能，此功能会显著提高内存和CPU占用率，"利大于弊"。
    - 客户端默认不再提供配置文件，***强烈推荐使用无配置文件模式启动客户端***

- 2025-08-15  v0.26.26
  - 修复：
    - windows注册服务 使用tls无法连接 [269](https://github.com/yisier/nps/issues/269)
    - 域名解析自动HTTPS逻辑bug [273](https://github.com/yisier/nps/issues/273)

- 2025-05-28  v0.26.25
  - 新增：
    - nps增加`nps(.exe) -server` 命令，用于管理NPS服务，安装和卸载服务在 Linux 下需要有 sudo 权限，Windows 下需要有管理员权限。

      ![img.png](/image/new/server.png)
    - 增加【TLS快捷启动命令】，可用于在tls模式下，快速启动客户端。 [257](https://github.com/yisier/nps/issues/257)

- 2025-04-16  v0.26.24
  - 新增：
    - 隧道支持复制功能，在隧道页面增加复制按钮，除了端口号为随机生成外，其他参数全部拷贝。[251](https://github.com/yisier/nps/issues/251)
  - 修复：
    - 私密代理连接时服务端会报错退出，[250](https://github.com/yisier/nps/issues/250)

- 2025-04-11  v0.26.23
  - 新增：
    - TCP 隧道支持 Proxy Protocol 协议来传递经过请求的真实 IP。

      ![img.png](/image/new/protocol.png)
  - 修复：
    - 客户端较多时，协程增长过快，感谢 [@huanglei288766](https://github.com/yisier/nps/pull/244) 的PR

- 2025-01-23  v0.26.22
  - 优化：
    - 客户端注册系统服务时，（新版客户端注册方式，非 install 命令） 将按照 `npc-[vkey].log` 格式保存日志，每个客户端VKEY单独一个日志文件，windows 日志位于 `npc.exe` 同级目录下，linux 位于 `/var/log/`目录。
    - 域名模式，HTTPS证书支持上传文件和文件路径，系统将自动识别路径还是证书内容。 [#175](https://github.com/yisier/nps/issues/175)
  - 修复：
    - ipv6地址显示不全 [#237](https://github.com/yisier/nps/issues/237)

- 2025-01-07  v0.26.21
  - 新增：
    - 客户端列表页面新增【快捷启动命令】，此命令为连接地址+链接秘钥的base64编码，方便一键启动、一键安装客户端。
    - 域名解析和隧道列表增加访问地址，可以点击隧道链接，直接网页打开。 [#234](https://github.com/yisier/nps/issues/234)
  - 优化：
    - 修改vkey生成方式，由16位缩短至10位，截取uuid前10位，避免重复。
    - 优化客户端启动方式，当npc文件目录下无配置文件时，可直接双击运行客户端，输入命令完成启动、安装、卸载客户端，直接启动和安装服务时需要输入【快捷启动命令】，卸载服务、启动服务、停止服务时，只需输入隧道秘钥【vkey】即可。安装服务以 nps-client-vkey 方式命名 ，实现注册多个服务。

      ![image](/image/new/cmd.png)

  > **注意**：强烈推荐使用无配置文件模式启动客户端，所有数据应该在服务端保存和配置，而客户端只做连接转发。客户端配置文件对小白极不友好，配置繁琐，容易出错。

- 2024-11-07  v0.26.20
  - 新增：
    - 客户端增加创建时间
  - 修复：
    - 客户端限速单位不统一 [#185](https://github.com/yisier/nps/issues/185)
    - 增加从下拉选择客户端,隧道列表排序,新增编辑后不会刷新界面 [#183](https://github.com/yisier/nps/issues/183)
    - 隧道数量限制无法统计域名映射 [#209](https://github.com/yisier/nps/issues/209)

- 2024-06-01  v0.26.19
  - golang 版本升级到 1.22。
  - 增加自动https，自动将http 重定向（301）到 https。
  - 客户端命令行方式启动支持多个隧道ID，使用逗号拼接，示例：`npc -server=xxx:8024 -vkey=ytkpyr0er676m0r7,iwnbjfbvygvzyzzt`。
  - 移除 nps.conf 参数 `https_just_proxy` ，调整 https 处理逻辑，如果上传了 https 证书，则由nps负责SSL（此方式可以获取真实IP），否则走端口转发模式（使用本地证书，nps 获取不到真实IP），如下图所示。

    ![image](/image/new/https.png)

- 2024-02-27  v0.26.18
  - ***新增***：nps.conf 新增 `tls_bridge_port=8025` 参数，当 `tls_enable=true` 时，nps 会监听8025端口，作为 tls 的连接端口。客户端可以选择连接 tls 端口或者非 tls 端口：`npc.exe -server=xxx:8024 -vkey=xxx` 或 `npc.exe -server=xxx:8025 -vkey=xxx -tls_enable=true`

- 2024-01-31  v0.26.17
  - ***说明***：考虑到 npc 历史版本客户端众多，版本号不同旧版本客户端无法连接，为了兼容，仓库版本号将继续沿用 0.26.xx

- 2024-01-02  v0.27.01  (已作废，功能移动到 v0.26.17 版本)
  - ***新增***：tls 流量加密，(客户端忽略证书校验，谨慎使用，客户端与服务端需要同时开启，或同时关闭)，使用方式：
    - 服务端：nps.conf `tls_enable=true`；
    - 客户端：npc.conf `tls_enable=true` 或者 `npc.exe -server=xxx -vkey=xxx -tls_enable=true`

- 2023-06-01  v0.26.16
  - ***修复***：https 流量不统计 Bug 修复。
  - ***新增***：新增全局黑名单IP，用于防止被肉鸡扫描端口或被恶意攻击。
  - ***新增***：新增客户端上次在线时间。

- 2023-02-24  v0.26.15
  - ***修复***：更新程序 url 更改到当前仓库中
  - ***修复***：nps 在外部路径启动时找不到配置文件
  - ***新增***：增加 nps 启动参数，`-conf_path=D:\test\nps`，可用于加载指定nps配置文件和web文件目录。
  - ***window 使用示例：***
    - 直接启动：`nps.exe -conf_path=D:\test\nps`
    - 安装：`nps.exe install -conf_path=D:\test\nps`
    - 安装启动：`nps.exe start`
  - ***linux 使用示例：***
    - 直接启动：`./nps -conf_path=/app/nps`
    - 安装：`./nps install -conf_path=/app/nps`
    - 安装启动：`nps start -conf_path=/app/nps`

- 2022-12-30  v0.26.14
  - ***修复***：API 鉴权漏洞修复

- 2022-12-19
  - ***修复***：某些场景下丢包导致服务端意外退出
  - ***优化***：新增隧道时，不指定服务端口时，将自动生成端口号
  - ***优化***：API返回ID，`/client/add/`, `/index/addhost/`, `/index/add/`
  - ***优化***：域名解析、隧道页面，增加[唯一验证密钥]，方便搜查

- 2022-10-30
  - ***新增***：在管理面板中新增客户端时，可以配置多个黑名单IP，用于防止被肉鸡扫描端口或被恶意攻击。
  - ***优化***：0.26.12 版本还原了注册系统功能，使用方式和以前一样。无论是否注册了系统服务，直接执行 nps 时只会读取当前目录下的配置文件。

- 2022-10-27
  - ***新增***：在管理面板登录时开启验证码校验，开启方式：nps.conf `open_captcha=true`，感谢 [@dongFangTuring](https://github.com/dongFangTuring) 提供的PR

- 2022-10-24
  - ***修复***：HTTP协议支持WebSocket（稳定性待测试）

- 2022-10-21
  - ***修复***：HTTP协议下实时统计流量，能够精准的限制住流量（上下行对等）
  - ***优化***：删除HTTP隧道时，客户端已用流量不再清空

- 2022-10-19
  - ***BUG***：在TCP协议下，流量统计有问题，只有当连接断开时才会统计流量。例如，限制客户端流量20m,当传输100m的文件时，也能传输成功。
  - ***修复***：TCP协议下实时统计流量，能够精准的限制住流量（上下行对等）
  - ***优化***：删除TCP隧道时，客户端已用流量不再清空

    ![image](/image/new/tcp_limit.png)

- 2022-09-14
  - 修改NPS工作目录为当前可执行文件目录（即配置文件和nps可执行文件放在同一目录下，直接执行nps文件即可），去除注册系统服务，启动、停止、升级等命令
