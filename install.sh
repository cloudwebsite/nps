#!/bin/sh
# install.sh — cloudwebsite/nps（融合版）安装脚本
#
# 与旧 fork（如 djylb）的差异：
# - 面板 Web 已嵌入 nps 二进制，不需要 /etc/nps/web/
# - Release 包仅含可执行文件（无 conf/、无 web/）；配置由 nps 首次启动自动生成
#
# 用法：
#   ./install.sh [mode] [version] [install_dir]
#   mode: npc | nps | all（默认 all）
#   version: 发行标签，如 v0.26.41.1（默认 latest）
#   install_dir: 可选；指定则只解压到该目录，不安装到系统路径
#
# 环境变量：NPS_INSTALL_MODE / NPS_INSTALL_VERSION / NPS_INSTALL_DIR
#           NPS_CONNECT_TIMEOUT / NPS_INSECURE=1 / NPS_IPV4=1

# Must run as root（仅系统安装；指定 install_dir 时仍建议 root，避免权限混乱）
if command -v id >/dev/null 2>&1; then
  if [ "$(id -u)" -ne 0 ]; then
    echo "Error: Please run as root or use sudo." >&2
    exit 1
  fi
fi

set -e

has() { command -v "$1" >/dev/null 2>&1; }

for cmd in uname tar; do
  if ! has "$cmd"; then
    echo "Error: missing required command: $cmd" >&2
    exit 1
  fi
done
if ! has curl && ! has wget && ! has uclient-fetch; then
  echo "Error: need curl or wget or uclient-fetch" >&2
  exit 1
fi

CONNECT_TIMEOUT="${NPS_CONNECT_TIMEOUT:-10}"
if [ "${NPS_INSECURE:-0}" = "1" ]; then
  CURL_INSECURE="--insecure"
  WGET_INSECURE="--no-check-certificate"
  UCLIENT_INSECURE="--no-check-certificate"
else
  CURL_INSECURE=""
  WGET_INSECURE=""
  UCLIENT_INSECURE=""
fi
if [ "${NPS_IPV4:-0}" = "1" ]; then
  CURL_IP="-4"
  WGET_IP="-4"
else
  CURL_IP=""
  WGET_IP=""
fi

if has curl; then
  FETCH_TOOL="curl"
elif has wget; then
  FETCH_TOOL="wget"
else
  FETCH_TOOL="uclient-fetch"
fi

TMP_DIRS=""
cleanup() {
  for d in $TMP_DIRS; do
    case "$d" in
      /tmp/*)
        [ -n "$d" ] && [ "$d" != "/" ] && [ -d "$d" ] && rm -rf "$d"
        ;;
    esac
  done
}
trap cleanup 0 INT TERM

INSTALL_MODE="${1:-${NPS_INSTALL_MODE:-all}}"
INSTALL_VERSION="${2:-${NPS_INSTALL_VERSION:-latest}}"
INSTALL_DIR="${3:-${NPS_INSTALL_DIR:-}}"

case "$INSTALL_MODE" in
  npc|nps|all) ;;
  *)
    echo "Error: unsupported mode: $INSTALL_MODE（允许 npc|nps|all）" >&2
    exit 1
    ;;
esac

echo "Mode: $INSTALL_MODE"

USE_CF_LATEST=0

if [ "$INSTALL_VERSION" = "latest" ]; then
  echo "Get latest version..."
  if has grep && has sed; then
    API_URL="https://api.github.com/repos/cloudwebsite/nps/releases/latest"
    if has curl; then
      RAW_JSON=$(curl -sSLf "$API_URL" || true)
    else
      RAW_JSON=$(wget -qO- "$API_URL" || true)
    fi

    INSTALL_VERSION=$(printf '%s' "$RAW_JSON" \
      | grep -m1 '"tag_name"' \
      | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')

    if [ -z "$INSTALL_VERSION" ]; then
      echo "Warn: failed to detect version from GitHub API, will use CDN @latest." >&2
      USE_CF_LATEST=1
      INSTALL_VERSION=latest
    fi
  else
    echo "No grep/sed; use CDN @latest." >&2
    USE_CF_LATEST=1
    INSTALL_VERSION=latest
  fi
fi

if [ "$USE_CF_LATEST" -eq 1 ]; then
  echo "Version: latest (CDN @latest fallback)"
else
  echo "Version: $INSTALL_VERSION"
fi

OS="$(uname -s)"
case "$OS" in
  Linux)   OS=linux;;
  Darwin)  OS=darwin;;
  FreeBSD) OS=freebsd;;
  *)
    echo "Error: OS not supported: $OS" >&2
    exit 1
    ;;
esac

RAW_ARCH="$(uname -m)"
case "$RAW_ARCH" in
  x86_64|amd64)      ARCH=amd64;;
  i?86)              ARCH=386;;
  aarch64|arm64)     ARCH=arm64;;
  armv7*|armv7l)     ARCH=arm_v7;;
  armv6*|armv6l)     ARCH=arm_v6;;
  armv5*|armv5l)     ARCH=arm_v5;;
  arm)               ARCH=arm;;
  mips64le)          ARCH=mips64le;;
  mips64)            ARCH=mips64;;
  mipsle)            ARCH=mipsle;;
  mips)              ARCH=mips;;
  loongarch64)       ARCH=loong64;;
  riscv64)           ARCH=riscv64;;
  *)
    echo "Error: ARCH not supported: $RAW_ARCH" >&2
    exit 1
    ;;
esac

echo "Detected OS/ARCH: $OS/$ARCH"

if [ "$ARCH" = "mips64" ] || [ "$ARCH" = "mips64le" ]; then
  if [ -r /proc/cpuinfo ] && ! grep -qi 'fpu' /proc/cpuinfo; then
    ARCH="${ARCH}_softfloat"
    echo "No FPU in /proc/cpuinfo; use soft-float, new ARCH: $ARCH"
  else
    echo "Use hard-float for $ARCH"
  fi
elif [ "$ARCH" = "mips" ] || [ "$ARCH" = "mipsle" ]; then
  if has file; then
    out="$(file /bin/sh 2>/dev/null || true)"
    case "$out" in
      *hard[-\ ]float*)
        echo "Use hard-float for $ARCH"
        ;;
      *)
        ARCH="${ARCH}_softfloat"
        echo "Use soft-float, new ARCH: $ARCH"
        ;;
    esac
  else
    ARCH="${ARCH}_softfloat"
    echo "No 'file' tool; default to soft-float: $ARCH"
  fi
fi

tar_extract() {
  f="$1"

  if tar -xzf "$f" 2>/dev/null; then
    return 0
  fi

  if tar xf "$f" 2>/dev/null; then
    return 0
  fi

  if has gzip; then
    if gzip -dc "$f" | tar xf -; then
      return 0
    fi
  fi

  if has gunzip; then
    if gunzip -c "$f" | tar xf -; then
      return 0
    fi
  fi

  return 1
}

# 在解压目录中定位二进制（兼容顶层或子目录）
find_bin() {
  name="$1"
  root="$2"
  if [ -x "$root/$name" ]; then
    printf '%s\n' "$root/$name"
    return 0
  fi
  found=$(find "$root" -maxdepth 3 -type f -name "$name" 2>/dev/null | head -n 1)
  if [ -n "$found" ] && [ -f "$found" ]; then
    printf '%s\n' "$found"
    return 0
  fi
  return 1
}

install_bin() {
  src="$1"
  name="$2"
  if cp -f "$src" "/usr/bin/$name" 2>/dev/null; then
    chmod 755 "/usr/bin/$name"
    echo "Installed /$name -> /usr/bin/$name"
    return 0
  fi
  mkdir -p /usr/local/bin
  cp -f "$src" "/usr/local/bin/$name"
  chmod 755 "/usr/local/bin/$name"
  echo "Installed /$name -> /usr/local/bin/$name"
}

download() {
  NAME=$1
  FILE="${OS}_${ARCH}_${NAME}.tar.gz"

  # GitHub Releases + jsDelivr 镜像仓（cloudwebsite/nps-mirror）
  if [ "$USE_CF_LATEST" -eq 1 ]; then
    URLS="
      https://cdn.jsdelivr.net/gh/cloudwebsite/nps-mirror@latest/${FILE}
      https://fastly.jsdelivr.net/gh/cloudwebsite/nps-mirror@latest/${FILE}
      https://github.com/cloudwebsite/nps/releases/latest/download/${FILE}
    "
  else
    URLS="
      https://github.com/cloudwebsite/nps/releases/download/${INSTALL_VERSION}/${FILE}
      https://cdn.jsdelivr.net/gh/cloudwebsite/nps-mirror@${INSTALL_VERSION}/${FILE}
      https://fastly.jsdelivr.net/gh/cloudwebsite/nps-mirror@${INSTALL_VERSION}/${FILE}
    "
  fi

  if [ -n "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
    WORKDIR="$INSTALL_DIR"
  else
    if has mktemp; then
      TMPD=$(mktemp -d 2>/dev/null || mktemp -d -t nps-install.XXXXXX)
    else
      TS="$(date +%s 2>/dev/null || echo 0)"
      TMPD="/tmp/nps-$$.$TS"
      mkdir -p "$TMPD"
    fi
    TMP_DIRS="$TMP_DIRS $TMPD"
    WORKDIR="$TMPD"
  fi

  cd "$WORKDIR"

  success=0
  for u in $URLS; do
    echo "Trying $u" >&2

    rm -f -- "./$FILE"

    case "$FETCH_TOOL" in
      curl)
        if curl $CURL_IP -sfSL $CURL_INSECURE --connect-timeout "$CONNECT_TIMEOUT" -o "$FILE" "$u"; then
          success=1
        fi
        ;;
      wget)
        if wget $WGET_IP -q $WGET_INSECURE -T "$CONNECT_TIMEOUT" -O "$FILE" "$u"; then
          success=1
        fi
        ;;
      uclient-fetch)
        if uclient-fetch -q $UCLIENT_INSECURE -T "$CONNECT_TIMEOUT" -O "$FILE" "$u"; then
          success=1
        fi
        ;;
    esac

    if [ "$success" -eq 1 ] && [ -s "$FILE" ]; then
      break
    fi
  done

  if [ "$success" -ne 1 ] || [ ! -f "$FILE" ]; then
    echo "Error: Download failed for all URLs:" >&2
    for u in $URLS; do
      echo "  - $u" >&2
    done
    exit 1
  fi

  if ! tar_extract "$FILE"; then
    echo "Error: failed to extract $FILE" >&2
    exit 1
  fi

  rm -f -- "./$FILE"
  printf '%s\n' "$WORKDIR"
}

# 客户端：仅安装 npc 二进制（发行包不含 conf）
install_npc() {
  SRC=$(download client)
  echo "Install npc..."

  BIN=$(find_bin npc "$SRC") || {
    echo "Error: 'npc' binary not found in package ($SRC)" >&2
    exit 1
  }

  if [ -n "$INSTALL_DIR" ]; then
    echo "npc extracted to $INSTALL_DIR（二进制: $BIN）"
    return
  fi

  install_bin "$BIN" npc
  mkdir -p /etc/nps/conf
  echo "npc done"
  echo "提示: 用 npc -server=... -vkey=... 启动，或自备 conf/npc.conf"
}

# 服务端：仅安装 nps 二进制；创建配置目录；不安装 web/（已嵌入）
install_nps() {
  SRC=$(download server)
  echo "Install nps..."

  BIN=$(find_bin nps "$SRC") || {
    echo "Error: 'nps' binary not found in package ($SRC)" >&2
    exit 1
  }

  if [ -n "$INSTALL_DIR" ]; then
    echo "nps extracted to $INSTALL_DIR（二进制: $BIN）"
    return
  fi

  install_bin "$BIN" nps

  # GetInstallPath=/etc/nps；存在后服务/CLI 会从此读 conf
  mkdir -p /etc/nps/conf
  # 故意不创建 web/：融合版面板静态资源已 embed 进 nps
  if [ -d /etc/nps/web ]; then
    echo "Note: /etc/nps/web 可忽略（本版 Web 已嵌入二进制）；如需清理可手动 rm -rf /etc/nps/web"
  fi

  echo "nps done"
  echo "配置目录: /etc/nps/conf （首次 nps 启动或 nps install 时自动生成 nps.conf）"
  echo "下一步: nps install && nps start   # 或以管理员/root 执行"
}

case "$INSTALL_MODE" in
  npc) install_npc ;;
  nps) install_nps ;;
  all) install_npc; install_nps ;;
esac

echo "All done"
