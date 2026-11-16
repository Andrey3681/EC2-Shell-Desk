---
name: golang-engineering
description: Idiomatic Go engineering, strict Hexagonal Architecture, error wrapping, concurrency safety, and lifecycle management.
---

# Go Engineering Skill

Use this skill for all Go code in `cmd/` and `internal/`.

## Guidelines
1. **Hexagonal Dependency Rule:**
   - `internal/core/domain` has zero external dependencies (stdlib only).
   - `internal/core/ports` imports only `context`, `io`, and `internal/core/domain`.
   - `internal/services` imports only `internal/core/domain` and `internal/core/ports`.
   - `internal/adapters/*` implement `internal/core/ports` interfaces.
2. **Explicit Error Handling:**
   - Wrap errors with context using `fmt.Errorf("action failed: %w", err)`.
   - Match domain sentinel errors with `errors.Is` / `errors.As`.
   - Never swallow errors silently (`_ = ...` only for explicitly documented cleanup like `defer file.Close()` when read-only).
3. **Concurrency & Goroutine Hygiene:**
   - Protect shared session maps and transfer queues with `sync.RWMutex` or `sync.Mutex`.
   - Every goroutine (SSH stdout/stderr reader, keepalive ticker, SFTP transfer worker) MUST honor `ctx.Done()` and exit cleanly when a tab or app closes.
   - Verify all packages under `go test ./... -race`.
4. **Naming & Formatting:**
   - All identifiers, struct fields, errors, and comments in **English**.
   - Format with `gofumpt -l -w .` and pass `golangci-lint run ./...`.
