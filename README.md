<div align="center">
  <img src="assets/images/banner.png" alt="Horsync Banner" width="800">
</div>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
  <img src="https://img.shields.io/github/go-mod/go-version/Yavuzlar/horsync" alt="Go Version">
  <img src="https://img.shields.io/github/v/release/Yavuzlar/horsync" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/Yavuzlar/horsync/ci.yml?branch=main" alt="CI">

  <br>

  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#documentation">Docs</a> •
  <a href="#contributing">Contributing</a>
</p>

---

Horsync combines a centralized SaaS control plane (Hub) with direct peer-to-peer block exchange between client nodes, featuring client-side encryption, automated metadata stripping, and cross-platform background agents.

---

## Features

- **Hybrid Architecture** — Central Hub + direct P2P transfers between nodes
- **Zero-Knowledge Encryption** — AES-GCM 256-bit with PBKDF2 key derivation
- **Metadata Stripping** — EXIF removal from images, XMP stripping from PDFs/Office docs
- **P2P Mesh Discovery** — UDP multicast (port 21027) for automatic LAN peer discovery
- **Secure Transfers** — TLS 1.2+ encrypted TCP (port 22000) with strict approval mode
- **Background Agent** — Cross-platform sync agent (Windows, Linux, macOS)
- **Chunked Uploads** — Resumable, integrity-verified with SHA-256 validation
- **Bandwidth Governance** — Token-bucket rate limiting
- **Automation Rules** — Configurable policies for encryption, metadata wiping, archival

---

## Quick Start

### Prerequisites

Go 1.22+, Node.js 18+, Docker Desktop (for PostgreSQL).

### Windows

```powershell
run_mvp.bat
```

### Manual

```bash
docker compose up -d postgres
cp .env.example .env
cd frontend && npm install && cd ..
go build -o bin/horsync ./cmd/horsync
DATABASE_URL="postgres://horsync:horsync123@localhost:5433/horsync?sslmode=disable" bin/horsync
# Another terminal:
cd frontend && npm run dev
```

Then open **http://localhost:3000**.

| Default Login | |
|---|---|
| Email | `admin@horsync.local` |
| Password | `admin12345` |

---

## Tests

```bash
go test ./... -v -race -count=1
cd frontend && npx tsc --noEmit && npm run build
```

---

## Documentation

- [Contributing](CONTRIBUTING.md)
- [Frontend](frontend/README.md)

---

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

MIT License — see [LICENSE](LICENSE).

---

<div align="center">
  <sub>Built by <a href="https://github.com/Yavuzlar">Yavuzlar</a></sub>
  <br><br>
  <a href="https://yavuzlar.org">🌐 Website</a> •
  <a href="https://x.com/siberyavuzlar">🐦 X/Twitter</a> •
  <a href="https://linkedin.com/company/siberyavuzlar">💼 LinkedIn</a> •
  <a href="https://instagram.com/siberyavuzlar">📸 Instagram</a> •
  <a href="mailto:cyberyavuzlar@gmail.com">✉️ Email</a>
</div>
