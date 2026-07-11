# Contributing to Horsync

First off, thank you for considering contributing to Horsync! We welcome contributions from everyone, whether it's a bug report, feature suggestion, documentation improvement, or code change.

<p align="center">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg" alt="PRs Welcome">
  <img src="https://img.shields.io/github/issues/Yavuzlar/horsync/good%20first%20issue" alt="Good First Issues">
  <img src="https://img.shields.io/github/contributors/Yavuzlar/horsync" alt="Contributors">
</p>

---

## Code of Conduct

This project is committed to providing a welcoming and inclusive experience for everyone. Be respectful, constructive, and considerate in all interactions.

---

## How to Contribute

### Reporting Bugs

- Check if the bug has already been reported in [Issues](https://github.com/Yavuzlar/horsync/issues).
- If not, open a new issue with:
  - A clear title and description
  - Steps to reproduce
  - Expected vs actual behavior
  - Environment details (OS, Go version, database)
  - Logs or screenshots if applicable

### Suggesting Features

- Search existing issues to see if the feature has been discussed.
- Open a feature request issue with:
  - The problem you're trying to solve
  - Your proposed solution
  - Alternative approaches considered
  - Mockups or diagrams if relevant

### Code Contributions

#### Getting Started

```bash
# Fork the repo and clone your fork
git clone https://github.com/Yavuzlar/horsync.git
cd horsync

# Ensure you have the prerequisites:
# - Go 1.22+
# - Node.js 18+
# - Docker Desktop (for PostgreSQL)

# Copy environment config
cp .env.example .env

# Run the development setup
./run_mvp.bat   # Windows
# OR manually:
docker compose up -d postgres
cd frontend && npm install && cd ..
go build -o bin/horsync ./cmd/horsync
```

#### Development Workflow

```bash
# Create a branch
git checkout -b feat/my-feature

# Make your changes and run tests
go test ./... -v -race -count=1
go vet ./...
cd frontend && npx tsc --noEmit

# Commit with a descriptive message
git commit -m "feat: add node grouping by location"
```

#### Branch Naming

- `feat/` — new features
- `fix/` — bug fixes
- `docs/` — documentation only
- `refactor/` — code restructuring
- `test/` — test additions or improvements
- `chore/` — build process, CI, dependencies

#### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `style`, `perf`

Examples:
- `feat: add bandwidth governor to P2P transfers`
- `fix: prevent nil pointer in vault lock when no key loaded`
- `docs: update README with new API endpoints`

### Pull Request Process

1. **Keep PRs focused** — one feature/fix per PR.
2. **Write tests** for new functionality.
3. **Ensure CI passes** — all checks must be green.
4. **Update documentation** if you change behavior.
5. **Request review** from maintainers.
6. **Address feedback** promptly.

#### PR Checklist

- [ ] Code compiles (`go build ./...`)
- [ ] Tests pass (`go test ./...`)
- [ ] Linter passes (`go vet ./...`) and TypeScript clean
- [ ] Added tests for new code
- [ ] Updated documentation
- [ ] Commit messages follow conventions
- [ ] PR description explains the change and motivation

---

## Project Structure

```
horsync/
├── cmd/horsync/             # Main binary entrypoint
├── internal/
│   ├── api/                 # HTTP layer (handlers, middleware, routes)
│   ├── config/              # Configuration, database migrations
│   ├── core/
│   │   ├── engine/          # Automation rules engine
│   │   ├── p2p/             # P2P networking (TCP, TLS, UDP discovery)
│   │   ├── sysmonitor/      # System monitoring (CPU, RAM, disk)
│   │   ├── topology/        # Device mesh topology management
│   │   ├── transfer/        # File upload & replication
│   │   └── vault/           # Encryption vault (AES-GCM)
│   └── models/              # Data transfer objects
├── pkg/
│   ├── logger/              # Structured logging
│   └── utils/               # Shared utilities
├── frontend/                # React + TypeScript SPA
│   └── src/
│       ├── components/      # UI components
│       ├── lib/             # Types, i18n, helpers
│       └── services/        # API client
└── data/                    # Runtime data (gitignored)
```

---

## Coding Standards

### Go

- Follow [Go style guide](https://go.dev/doc/effective_go).
- Use `gofmt` / `go vet` before committing.
- Prefer `slog` for logging (`logger.L.Info`, `logger.L.Error`).
- Errors should be wrapped with context: `fmt.Errorf("do thing: %w", err)`.
- Use `sync.RWMutex` for thread-safe singletons.
- Keep packages focused; avoid circular imports.

### TypeScript / React

- Use TypeScript strict mode.
- Components go in `frontend/src/components/`.
- Services/interfaces in `frontend/src/lib/` and `frontend/src/services/`.
- Use `tailwind-merge` + `clsx` utility for className composition.
- Prefer functional components with hooks.

---

## Need Help?

- Open a [Discussion](https://github.com/Yavuzlar/horsync/discussions).
- Tag maintainers in issues.
- Check the [README](./README.md) for architecture docs.

---

**Thank you for helping make Horsync better!**
