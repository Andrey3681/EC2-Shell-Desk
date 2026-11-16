---
name: wails-desktop
description: Wails v2 desktop patterns for Windows, Go-to-TypeScript bindings, runtime events, native file dialogs, and executable/NSIS packaging.
---

# Wails v2 Desktop Skill

Use this skill when building or modifying `cmd/desktop/main.go`, `internal/app/`, `wails.json`, `build/windows/`, or `frontend/src/services/`.

## Guidelines
1. **Wails v2 API Only:** Use `github.com/wailsapp/wails/v2` (`wails.Run`, `options.App`, `runtime.EventsEmit`, `runtime.OpenFileDialog`). Never mix Wails v3 APIs.
2. **Thin App Handlers:** `internal/app/*_handler.go` structs are bound via `Bind: []interface{}{...}` in `cmd/desktop/main.go`. Handlers must validate input DTOs, delegate to `internal/services/`, and map domain errors into clean user-safe error messages.
3. **Streaming Events:**
   - Terminal output: emit `terminal:data:<sessionID>` and `terminal:closed:<sessionID>` via `runtime.EventsEmit`.
   - SFTP transfer progress: emit `sftp:progress` with `{ transferId, bytesTransferred, totalBytes, state, error }` throttled (e.g. every ~50ms or 64KB) so the WebView2 UI remains responsive.
   - TOFU host verification: emit or return structured host fingerprint prompts before continuing an unverified SSH handshake.
4. **Native Windows File Dialogs:** Use `runtime.OpenFileDialog` for importing `.pem` / OpenSSH keys and selecting local files/directories safely without exposing raw filesystem APIs to untrusted scripts.
5. **Lifecycle Cleanup:** On Wails `OnShutdown(ctx)`, gracefully close all active SSH/SFTP sessions, cancel ongoing transfers, and wipe unlocked master keys from memory.
