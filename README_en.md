# 🔀 NPS Fusion Workspace

> Display `0.26.41.1` · Core handshake **`0.26.0` (do not bump)**

[中文](./README.md) | English

A lightweight, high-performance **intranet penetration** proxy (tcp / udp / http(s) / socks5 / p2p / Shadowsocks, …) with a Web admin UI.  
This tree is the **fusion workspace**: capabilities from sibling forks are merged under the compatibility contract—not a raw upstream mirror.

---

## ✅ Status (summary)

| Scope | Status |
|------|--------|
| Upgrade base / Bridge / ACL / user & API slices | ✅ Landed (enforcement flags default **off**) |
| Modern P2P | ✅ `legacy` default; `modern` = STUN + hole-punch then QUIC; optional holepunch |
| Register email OTP | ✅ SMTP; default off |
| Web terminal | ✅ Quick commands + Windows ConPTY; off by default with a guidance page |
| Shadowsocks | ✅ One-click / auto PSK |
| GUI **dual ship** | ✅ Fyne `gui/npc` + Wails `cmd/npc/npc-gui` |
| Admin panel UI | ✅ BS5 + graphite-teal theme + `npsTable` + env labels |
| Admin TOTP | ✅ `totp_secret` / `nps -gen2fa`; default off |
| Error page & notify | ✅ Custom error page + WeCom/TG/Webhook; default off |

---

## 🚀 Quick start

### 1. Build (CLI)

Need **Go 1.24+** on `PATH` (**1.26+** recommended for holepunch / some GUI deps):

```powershell
cd "f:\go_lang\nps"
go mod tidy
go test ./client/ ./lib/crypt/ ./lib/mail/ ./lib/notify/ ./server/proxy/ ./cmd/npc/ ./lib/install/ -count=1
go build -o nps.exe ./cmd/nps
go build -o npc.exe ./cmd/npc
```

Optional drill: `.\scripts\upgrade-drill.ps1`

### 2. Server

```text
# Linux / macOS
sudo ./nps install && sudo nps start

# Windows (Administrator)
nps.exe install
nps.exe start
```

Default ports (see `conf`):

| Port | Role |
|------|------|
| 80 / 443 | Host / domain proxy |
| 8080 | Web admin |
| 8024 | Bridge (client ↔ server) |

Config path after install: often `C:\Program Files\nps` on Windows; `/etc/nps` on Linux / Darwin.  
Change the default admin password immediately.

### 3. Client

- **CLI**: copy the command from the panel, or  
  `npc -server=host:8024 -vkey=KEY`
- **GUI (dual ship)**  
  - 🪶 **Fyne** (lightweight / Android): `gui/npc`  
  - 🖥️ **Wails** (tray / multi-client): `cmd/npc/npc-gui` (Win / macOS / Linux in Release)

---

## 🛡️ Upgrade mantra (lossless first)

```text
Backup conf → upgrade nps first (old npc still connects) → observe → roll npc in batches → enable hard policies last
```

- Keep binary names **`nps` / `npc`**
- Keep core handshake at **`0.26.0`** until legacy clients are gone
- New gates (challenge handshake, fingerprint enforce, email verify, …) stay **off** by default

---

## 🧩 Feature snapshot

- 📡 Protocols: tcp / udp / http(s) / socks5 / p2p / Shadowsocks …
- 🖥️ Platforms: Linux / Windows / macOS; optional system service
- 🔐 TLS, optional cert fingerprint, compression, rate limits, multi-user, admin TOTP …
- 🌐 Hosts: custom headers, Host rewrite, site protect, routing, wildcards, custom error page …
- 🎛️ Panel: users / audit / terminal / SS / conf preview & whitelist write-back; env labels & dark/light theme
- 📣 Ops: optional WeCom / Telegram / Webhook notify (default off)

---

## 📖 Docs

VuePress sources live in [`docs/`](./docs/). Handy pages:

- [Server config](./docs/server/server_config.md)
- [Extended features](./docs/server/nps_extend.md)
- [Changelog](./docs/changelog/index.md)

---

## 🔧 Dev tips

```powershell
# Wails desktop GUI (Node / Yarn / wails3)
cd cmd\npc\npc-gui
wails3 dev

# Fyne simple GUI
go build -o npc-fyne.exe ./gui/npc
```

Release: Fyne Android APK + Wails desktop (Win / macOS / Linux) dual ship.

---

## 🤝 Contributing

- Preserve lossless upgrade behavior for production fleets
- Handshake / protocol changes must keep old `npc` connectable by default
