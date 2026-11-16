# Rule: Go Hexagonal Architecture

- Keep `internal/core/domain` and `internal/core/ports` strictly isolated from infrastructure, drivers, and UI frameworks.
- `internal/core/` MUST NOT import `internal/adapters/`, `internal/app/`, `modernc.org/sqlite`, `golang.org/x/crypto`, `github.com/pkg/sftp`, or `github.com/wailsapp/wails/v2`.
- `internal/services/` depends only on `internal/core/domain` and `internal/core/ports`, orchestrating use cases and enforcing `OwnerUserID` authorization invariants.
- Concrete adapters live exclusively in `internal/adapters/{sqlite,crypto,sshclient,sftpclient,winfs}` and implement port interfaces defined in `internal/core/ports`.
- Wails bindings live in `internal/app/` and translate DTOs/errors between the frontend and `internal/services/`.
- Wire all dependencies exclusively in `cmd/desktop/main.go`.
