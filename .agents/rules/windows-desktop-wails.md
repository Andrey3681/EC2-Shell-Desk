# Rule: Windows Desktop & Wails v2 Conventions

- Target **Wails v2** (`github.com/wailsapp/wails/v2`) and Microsoft Edge **WebView2** on Windows (`windows/amd64`). Never mix Wails v3 APIs.
- Keep Wails runtime calls (`runtime.EventsEmit`, `runtime.OpenFileDialog`, lifecycle hooks) inside `cmd/desktop` or `internal/app`, never inside `internal/core` or `internal/services`.
- Stream terminal output (`xterm.js`) and SFTP transfer progress asynchronously using Wails events scoped by `sessionID` or `transferID`.
- Keep all frontend Wails wrappers typed in `frontend/src/services/` over generated bindings (`frontend/wailsjs/`).
- Isolate all user-facing UI strings in `frontend/src/i18n/` (`es` and `en`) rather than hardcoding copy inside components or Go services.
