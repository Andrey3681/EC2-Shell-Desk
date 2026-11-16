# Engineering & AI Agent Rules — EC2 Shell Desk (Windows Go + Wails)

You are an expert in Go, Wails v2 desktop development for Windows, SSH/SFTP protocols, local cryptography, and Hexagonal Architecture.

## 1. Language Rules (STRICT)
- **Code Identifiers & Struct Attributes in English:** ALL Go packages, file names, interfaces, structs, struct fields/attributes, functions, methods, variables, constants, SQL columns, and TypeScript types/props MUST be written in **English** (e.g., `HostInstance`, `hostAddress`, `sshUsername`, `privateKeyPEM`, `ownerUserID`).
- **Code Comments & Docstrings in English:** ALL inline comments (`//`), TODOs, package docs, and exported Go doc comments (`// SymbolName ...`) MUST be written in **English**.
- **Sensitive Logging Forbidden:** NEVER log private keys, passphrases, user passwords, or decrypted secrets in logs, errors, or test snapshots.
- **UI Copy via i18n:** Keep user-facing UI strings isolated in `frontend/src/i18n/` (`es` / `en`), decoupled from Go domain logic.

## 2. Hexagonal Architecture & Separation of Concerns
- Follow strict Hexagonal Architecture: `internal/core/{domain,ports}`, `internal/adapters`, `internal/services`, `internal/app`, and `frontend/`.
- `internal/core/` MUST NOT import concrete adapters (`sqlite`, `sshclient`, `sftpclient`, `crypto`, `winfs`), Wails runtime, or UI frameworks.
- `internal/services/` MUST depend only on `internal/core/domain` and `internal/core/ports`.
- Dependency injection happens exclusively in `cmd/desktop/main.go`.
- Every database query for instances, SSH keys, or known hosts MUST enforce `user_id` ownership isolation.

## 3. Security & SSH/SFTP Standards
- Encrypt all stored SSH private keys at rest per user using Argon2id key derivation + AES-256-GCM (never store plaintext `.pem` files inside app data).
- Wipe sensitive byte slices from memory when sessions lock or keys are unloaded where practical.
- Never silently ignore host key verification (`ssh.InsecureIgnoreHostKey` is forbidden in production paths; implement TOFU prompt and persist verified host fingerprints per user).
- Always attach timeouts and `context.Context` cancellation to SSH dialing, PTY sessions, and SFTP file transfers so closing a tab never leaks goroutines.
- Sanitize and validate local Windows paths and remote POSIX paths to prevent directory traversal.

## 4. Documentation, Verification & Commits
- Every exported Go type, function, method, and constant MUST have a valid Go doc comment in English starting with its exact name (compatible with `gomarkdoc`).
- Keep `README.md`, `CHANGELOG.md`, and `docs/` synchronized whenever features, schema, or build steps change.
- Run `task check` (`gofumpt`, `golangci-lint`, `go test`) before considering any task complete.
- Follow **Conventional Commits 1.0.0** in English imperative mood (`feat(vault): encrypt imported PEM keys with AES-256-GCM`).
