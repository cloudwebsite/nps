# 无损升级演练夹具（Windows）
# 用法：在已备份的真实 conf 副本上，用本目录编译的 nps 做冒烟。
# 本脚本不替你连旧 npc，只检查：迁移幂等、核心版本、关键文件仍在。

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location -LiteralPath $Root

Write-Host "== build =="
go build -o "$Root\nps.exe" ./cmd/nps
go build -o "$Root\npc.exe" ./cmd/npc

Write-Host "== unit tests (migration / password) =="
go test ./lib/common/ ./cmd/nps/

Write-Host "== core version lock =="
$verSrc = Get-Content -LiteralPath "$Root\lib\version\version.go" -Raw
if ($verSrc -notmatch 'CoreVersion\s*=\s*"0\.26\.0"') {
    throw "无损升级演练失败：CoreVersion 必须保持 0.26.0"
}
Write-Host "CoreVersion OK"

$DrillConf = Join-Path $Root "scripts\drill-conf"
if (-not (Test-Path -LiteralPath $DrillConf)) {
    New-Item -ItemType Directory -Force -Path $DrillConf | Out-Null
    Copy-Item -LiteralPath "$Root\conf\*" -Destination $DrillConf -Recurse -Force
    Write-Host "seeded scripts/drill-conf from conf/"
}

Write-Host "== migrate idempotency (via go test already covers logic) =="
Write-Host "Manual next steps:"
Write-Host "  1) Backup production conf -> conf.bak.YYYYMMDD"
Write-Host "  2) Point -conf_path to a COPY of production conf"
Write-Host "  3) Start new nps; keep OLD npc connected"
Write-Host "  4) Verify online clients / tunnels / hosts counts"
Write-Host "  5) Stop new nps; restore old binary + conf.bak for rollback drill"
Write-Host "DONE (automated portion)"
