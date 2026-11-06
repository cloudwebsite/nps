#!/bin/bash
# Android APK（Fyne），在 fyneio/fyne-cross:android-latest 内运行。
# 依赖由宿主机预先 `go mod vendor`；容器内禁止再走 Git/VCS（镜像 Git 过旧）。
set -euo pipefail

cd /go
if grep -q "buster" /etc/apt/sources.list 2>/dev/null; then
  sed -i 's|http://deb.debian.org/debian|http://archive.debian.org/debian|g' /etc/apt/sources.list
  sed -i 's|http://security.debian.org/debian-security|http://archive.debian.org/debian-security|g' /etc/apt/sources.list
  echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99no-check-valid
fi
apt-get update
apt-get install -y libegl1-mesa-dev libgles2-mesa-dev libx11-dev xorg-dev ca-certificates curl

export GOPROXY=off
export GOFLAGS="-mod=vendor"
export GOTOOLCHAIN=local

GO_VERSION="${GO_VERSION:-1.26.0}"
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
export PATH="/usr/local/go/bin:$PATH"
go version

# fyne CLI 需联网装一次；与工程库版本解耦，装到 GOPATH/bin。
# v1.6.1 依赖的 x/tools 无法用 Go 1.26 编译，临时切 1.24 工具链只装 CLI。
export GOPROXY="${CLI_GOPROXY:-https://proxy.golang.org,direct}"
export GOFLAGS=""
export GOTOOLCHAIN=go1.24.6
GO111MODULE=on go install fyne.io/tools/cmd/fyne@v1.6.1
export PATH="$PATH:$(go env GOPATH)/bin"
echo "fyne install success"

# 恢复：工程用本机 Go 1.26 + vendor（容器 Git 过旧，禁止再拉模块）
export GOTOOLCHAIN=local
export GOPROXY=off
export GOFLAGS="-mod=vendor"

mkdir -p /go/src/ehang.io/nps
cp -a /app/. /go/src/ehang.io/nps/
cd /go/src/ehang.io/nps
rm -f cmd/npc/sdk.go

if [ ! -d vendor ]; then
  echo "vendor/ missing: host must run 'go mod vendor' before docker"
  exit 1
fi

# 铺开 vendor 到 GOPATH（兼容旧 fyne package 解析）
cp -a vendor/. /go/src/
# 保留 vendor 目录供 -mod=vendor
echo "vendor ready"

cd gui/npc
# fyne tools v1.6+ 使用 --app-id / --os（旧 -appID 已移除）
fyne package --app-id org.nps.client --os android --icon ../../docs/logo.png
apk="$(ls -1 *.apk 2>/dev/null | head -n 1)"
if [ -z "$apk" ]; then
  echo "No apk produced"
  ls -la
  exit 1
fi
mv "$apk" /app/android_client.apk
echo "android build success"
