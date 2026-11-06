# 自建更新源（国内镜像）

默认安装 / 在线升级会访问 GitHub：

| 用途 | 默认地址 |
| --- | --- |
| 查最新版本 | `https://api.github.com/repos/cloudwebsite/nps/releases/latest` |
| 下安装包 | `https://github.com/cloudwebsite/nps/releases/download/<tag>/<文件名>` |

国内部分地区访问 GitHub / `api.github.com` 不稳定或超时，可把这两处改成**自建镜像**。本页给出协议约定与可复制示例。

> 当前仅切换下载源；安装包 **SHA256 / 签名强制校验尚未落地**（见仓库「融合未完成清单」F-33）。镜像站请自管完整性与 HTTPS。

## 协议约定

### 1. 版本检查 API（`release_api_url`）

`GET` 返回 **JSON**，至少包含 GitHub Release 同名字段：

```json
{
  "tag_name": "v0.26.41.2"
}
```

其它字段可忽略。程序只解析 `tag_name`（可带或不带前缀 `v`）。

### 2. 下载前缀（`release_download_base`）

实际下载地址拼接为：

```text
{release_download_base}/{tag_name}/{GOOS}_{GOARCH}_{server|client}.tar.gz
```

示例（Linux amd64 服务端）：

```text
https://mirror.example.com/nps/download/v0.26.41.2/linux_amd64_server.tar.gz
```

常见文件名（与 Releases 附件一致）：

| 角色 | 模式 | 例 |
| --- | --- | --- |
| 服务端 | `{os}_{arch}_server.tar.gz` | `linux_amd64_server.tar.gz` |
| 客户端 | `{os}_{arch}_client.tar.gz` | `windows_amd64_client.tar.gz` |

`os` / `arch` 与 Go 的 `GOOS` / `GOARCH` 相同（如 `linux`/`windows`/`darwin`，`amd64`/`arm64`）。

## 配置方式

### 服务端（面板 / nps）

推荐：**Web 后台 → 全局参数 → 运维** 填写并保存（写入 `conf/global.json`，**热更新，无需重启**）：

| 面板字段 | 全局参数键 | 含义 |
| --- | --- | --- |
| Release API URL | `ReleaseAPIURL` / 旧 conf `release_api_url` | 版本检查 JSON 地址 |
| Release 下载前缀 | `ReleaseDownloadBase` / 旧 conf `release_download_base` | 安装包下载基址（无尾斜杠也可） |

`global.json` 示例：

```json
{
  "ReleaseAPIURL": "https://mirror.example.com/nps/api/latest",
  "ReleaseDownloadBase": "https://mirror.example.com/nps/download"
}
```

首次迁移种子也可写在 `nps.conf`（升级后以面板 / `global.json` 为准）：

```ini
# release_api_url=https://mirror.example.com/nps/api/latest
# release_download_base=https://mirror.example.com/nps/download
```

影响：`nps update`、面板触发的服务端自更等走 `lib/install` 下载逻辑时使用上述源。

### 客户端（npc）

独立升级（`npc update` / 安装脚本更新）不读服务端全局参数，需在客户端侧指定：

```bash
# 命令行
npc -release_api=https://mirror.example.com/nps/api/latest \
    -release_download_base=https://mirror.example.com/nps/download \
    update

# 或环境变量（与 flag 二选一；环境变量后写会覆盖 flag）
export NPC_RELEASE_API=https://mirror.example.com/nps/api/latest
export NPC_RELEASE_DOWNLOAD_BASE=https://mirror.example.com/nps/download
npc update
```

Windows（PowerShell）：

```powershell
$env:NPC_RELEASE_API = "https://mirror.example.com/nps/api/latest"
$env:NPC_RELEASE_DOWNLOAD_BASE = "https://mirror.example.com/nps/download"
.\npc.exe update
```

## 自建镜像示例

下面用域名 `mirror.example.com`，目录约定：

```text
/var/www/nps/
  api/latest.json          → 提供版本 JSON
  download/
    v0.26.41.2/
      linux_amd64_server.tar.gz
      linux_amd64_client.tar.gz
      windows_amd64_server.tar.gz
      windows_amd64_client.tar.gz
      ...（按需同步各平台）
```

对外：

- API：`https://mirror.example.com/nps/api/latest`
- 下载前缀：`https://mirror.example.com/nps/download`

### A. 从 GitHub 同步（机器能访问 GitHub 时）

在可访问 GitHub 的机器上定时拉取最新 Release，再 rsync / 对象存储到国内 CDN：

```bash
#!/usr/bin/env bash
# sync-nps-release.sh — 需 curl、jq；可选 gh
set -euo pipefail
REPO="cloudwebsite/nps"
ROOT="/var/www/nps"
API="https://api.github.com/repos/${REPO}/releases/latest"

meta=$(curl -fsSL "$API")
tag=$(echo "$meta" | jq -r .tag_name)
test -n "$tag" && test "$tag" != null

mkdir -p "$ROOT/api" "$ROOT/download/$tag"
echo "$meta" | jq '{tag_name}' > "$ROOT/api/latest.json"

# 按需只拉常用架构；也可 jq 遍历 .assets[].browser_download_url
for name in \
  linux_amd64_server.tar.gz linux_amd64_client.tar.gz \
  linux_arm64_server.tar.gz linux_arm64_client.tar.gz \
  windows_amd64_server.tar.gz windows_amd64_client.tar.gz \
  darwin_amd64_server.tar.gz darwin_amd64_client.tar.gz \
  darwin_arm64_server.tar.gz darwin_arm64_client.tar.gz
do
  url="https://github.com/${REPO}/releases/download/${tag}/${name}"
  dest="$ROOT/download/$tag/$name"
  if [[ ! -f "$dest" ]]; then
    curl -fL --retry 3 -o "$dest" "$url" || echo "skip missing $name"
  fi
done

echo "synced $tag -> $ROOT"
```

cron 示例（每天一次）：

```cron
30 3 * * * root /usr/local/bin/sync-nps-release.sh >>/var/log/nps-mirror.log 2>&1
```

### B. Nginx 静态托管

```nginx
server {
    listen 443 ssl http2;
    server_name mirror.example.com;
    # ssl_certificate / ssl_certificate_key ...

    # 版本 JSON：/nps/api/latest → latest.json
    location = /nps/api/latest {
        alias /var/www/nps/api/latest.json;
        default_type application/json;
        add_header Cache-Control "public, max-age=300";
    }

    # 安装包：/nps/download/<tag>/<file>
    location /nps/download/ {
        alias /var/www/nps/download/;
        add_header Cache-Control "public, max-age=86400";
    }
}
```

校验：

```bash
curl -fsSL https://mirror.example.com/nps/api/latest
# {"tag_name":"v0.26.41.2"}

curl -I https://mirror.example.com/nps/download/v0.26.41.2/linux_amd64_server.tar.gz
# HTTP/1.1 200
```

### C. 最小 Go 静态 API（可选）

若不想用 `alias` 映射 JSON，可用几行服务：

```go
package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func main() {
	tag := os.Getenv("NPS_LATEST_TAG") // 例：v0.26.41.2
	if tag == "" {
		tag = "v0.26.41.2"
	}
	http.HandleFunc("/nps/api/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
	})
	// 安装包仍建议用 Nginx / OSS 静态目录挂到 /nps/download/
	_ = http.ListenAndServe(":8088", nil)
}
```

### D. 公网加速网关（临时）

仅当自建尚未就绪、且你信任第三方加速时，可临时把 API / 下载指到可用的 GitHub 代理（地址因服务商而异，需自行验证可用性与安全性）。**生产环境更建议 A+B 自建**，避免依赖第三方代理可用性与供应链风险。

## 配置后自检

```bash
# 1）API 可解析
curl -fsSL "$RELEASE_API_URL" | jq -r .tag_name

# 2）拼出本机包 URL（以 linux/amd64 服务端为例）
TAG=$(curl -fsSL "$RELEASE_API_URL" | jq -r .tag_name)
curl -I "$RELEASE_DOWNLOAD_BASE/$TAG/linux_amd64_server.tar.gz"

# 3）服务端已配全局参数后执行更新（会比较版本，已是最新则跳过）
nps update
```

## 注意

1. **API 与下载必须成对**：只改 API、下载仍指向 GitHub，升级仍会失败。  
2. **tag 目录名必须与 `tag_name` 完全一致**（含 `v` 前缀与否）。  
3. **npc 与 nps 分别配置**：给服务端配了全局参数，不等于所有客户端自动走镜像。  
4. **远程升级客户端**（`allow_remote_update`）依赖客户端侧已具备的更新逻辑与可达源；旧 npc 可能不识别远程升级指令。  
5. 完整性校验（SHA256 / 签名）尚未强制，请用 HTTPS，并只同步官方 Release 附件。

## 相关配置项

- 配置表：[服务端配置 · 审计与其它可选开关](../server/server_config.html#审计与其它可选开关)  
- 全局参数热更新：[增强功能 · 全局参数](../server/nps_extend.html#全局参数)  
