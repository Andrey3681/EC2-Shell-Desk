# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial repository structure following Hexagonal Architecture (`cmd/desktop`, `internal/core`, `internal/services`, `internal/adapters`, `internal/app`) and Wails v2 frontend skeleton.
- AI agent governance (`AGENTS.md`), modular rules (`.agents/rules/`), and domain skills (`.agents/skills/`).
- Tooling and quality configuration (`Taskfile.yml`, `.golangci.yml`, `.gomarkdoc.yml`, `lefthook.yml`, `wails.json`, Windows scripts, and GitHub Actions CI workflow).
- Architecture, solution design, vault security, UI/UX, verification documentation, and PlantUML diagrams under `docs/`.
- Phase 2 core domain entities and sentinel errors (`internal/core/domain`), hexagonal port interfaces (`internal/core/ports`), Argon2id + AES-256-GCM vault cryptography adapter (`internal/adapters/crypto`), pure-Go SQLite persistence with embedded migrations and strict `owner_user_id` isolation (`internal/adapters/sqlite`), and application services (`AuthService`, `VaultService`, `HostService`) with unit and end-to-end multi-user isolation tests.
- Phase 3 SSH, SFTP, and local Windows filesystem engines: `internal/adapters/sshclient` (PEM/OpenSSH auth, `.ppk` rejection, TOFU host key verification callback, interactive PTY, resize, keepalive), `internal/adapters/sftpclient` (`pkg/sftp` directory ops and cancelable chunked uploads/downloads), `internal/adapters/winfs` (Windows drive roots and local directory browsing), `SessionService`, `TransferService`, and embedded Go SSH/SFTP server integration tests.
- Phase 4 Wails v2 desktop integration and UI: `internal/app/*_handler.go` bindings and `EventBridge`, `cmd/desktop/bootstrap` DI wiring, Vite + TypeScript + `xterm.js` desktop UI (`AuthView`, `InstanceSidebar`, `HostModal`, `KeyVaultModal`, `TofuModal`, `TerminalTabsManager`, `XftpDualPane`), bilingual `es`/`en` i18n, and Windows `.exe` packaging via `task build`.
- Source-Available Non-Commercial `LICENSE` reserving all ownership, commercial, and redistribution rights to `Andrey3681`, plus GitHub PR/Issue templates and repository metadata.
- SSH local port-forwarding tunnels (`internal/core/domain/ssh_tunnel.go`, `002_ssh_tunnels.sql`, `TunnelRepository`, `sshclient.StartLocalForward`, `TunnelService`, `TunnelHandler`, and sidebar `DB Tunnels` UI with PostgreSQL/MySQL/MongoDB/Redis/SQL Server presets for AWS RDS, DocumentDB, and local DB clients).
- User-defined host groups (`HostGroup`, `002_host_groups.sql`, `GroupRepository`, `HostService` group CRUD, Wails bindings, and collapsible sidebar group sections preserving `Prod`/`Staging`/`Dev` filtering inside each group).
