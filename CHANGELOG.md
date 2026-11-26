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
- Reusable command snippets (`CommandSnippet`, `003_command_snippets.sql`, `004_snippet_host_scope.sql`, `SnippetRepository`, `SnippetService`, `SnippetHandler`, and `Snippets` sidebar tab with per-instance or group scoping, auto-execute or paste-only execution, and multi-session terminal broadcast).
- Private Observability and Services Portal (`PrivateService`, `005_private_services.sql`, `ServiceRepository`, `ObservabilityService`, `ObservabilityHandler`, loopback SSH forwarding with on-demand random local ports, HTTP/HTTPS background health checks, presets for Grafana, Prometheus, Loki, Mailpit, Portainer, Netdata, Uptime Kuma, and custom endpoints, 4th sidebar `Servicios` tab with 1-click external browser launch, and per-host service shortcuts).
- Modern Commander SFTP (`XftpDualPane`) redesign and recursive directory uploads (`SFTPClient.UploadDir`, `SFTPClient.MkdirAll`, `TransferService.StartUploadDirectory`, `SFTPHandler.PickLocalFolderToUpload` / `StartUploadDirectory`), adding clickable path breadcrumbs, file-type SVG icons, quick filter, multi-selection with clean contextual action bars, drag-and-drop between panes, resizable bottom splitter, and persistent display modes (`docked`, `floating`, `maximized`) with automatic active SSH session context synchronization.
- Reusable `ports.HealthProber` port and `internal/adapters/httpprobe` adapter with shared TLS 1.2+ HTTP clients and per-service opt-in `allow_insecure_tls` setting (`006_service_insecure_tls.sql` + modal checkbox).
- Reference-counted SSH connection multiplexing pool in `internal/adapters/sshclient` so concurrent tunnels and private services targeting the same host share a single SSH connection while still enforcing `VerifyHostKey`.

### Changed
- Bounded finished SFTP transfer history in memory (`TransferService` default 100 entries, `XftpDualPane` 50 entries) and deterministic `StartedAt` sorting in `ListTransfers()`.
- Upgraded `.golangci.yml`, `Taskfile.yml`, and `.github/workflows/ci.yml` to `golangci-lint v2` and `go-version-file: go.mod` for Go 1.27 compatibility.
- Synchronized `README.md`, `docs/arquitectura.md`, `docs/diseno-solucion.md`, `docs/seguridad-boveda.md`, `docs/ui-ux-spec.md`, `docs/TODO.md`, `docs/diagramas/arquitectura-hexagonal.puml`, and `docs/api/` with all current features.

### Fixed
- Guarded `HostService.DeleteHost` against deleting host instances still referenced by `PrivateService` records, returning `domain.ErrHostInUse` instead of a raw SQLite foreign-key error.
- Synchronized the top `Panel SFTP (Xftp)` toggle button state when closing the panel via its internal close button and disconnected `ResizeObserver` in `TerminalTabsManager.disposeAll()`.

### Security
- Enforced active session authentication and strict `owner_user_id` ownership verification in `ObservabilityService.ProbeServiceHealth`.
- Added progressive exponential backoff (`domain.ErrTooManyAttempts`) after 5 consecutive failed local login attempts in `AuthService.Login`.
