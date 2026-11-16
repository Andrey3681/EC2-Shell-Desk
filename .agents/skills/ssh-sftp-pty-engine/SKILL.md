---
name: ssh-sftp-pty-engine
description: Core Xshell + Xftp technical engine using x/crypto/ssh, TOFU known_hosts, PTY bridge to xterm.js, and pkg/sftp streaming transfers.
---

# SSH, SFTP & PTY Engine Skill

Use this skill when building `internal/adapters/sshclient`, `internal/adapters/sftpclient`, `internal/services/session_service.go`, `internal/services/transfer_service.go`, and `xterm.js` integration.

## Guidelines
1. **Key Parsing & Formats:**
   - Support RSA, Ed25519, and ECDSA private keys in PEM and OpenSSH formats via `ssh.ParsePrivateKey` and `ssh.ParsePrivateKeyWithPassphrase`.
   - If a user attempts to import a PuTTY `.ppk` key, detect the `PuTTY-User-Key-File` header and return a structured domain error or converter guidance instead of a generic parse failure.
2. **Host Key Verification (TOFU):**
   - Implement a custom `ssh.HostKeyCallback` backed by `known_hosts` repository scoped to `ownerUserID`.
   - First connection to an unseen `(ownerUserID, host, port)`: compute SHA-256 fingerprint (`ssh.FingerprintSHA256`), pause or request explicit user confirmation in the UI, and persist upon acceptance.
   - Mismatched host key: immediately abort the handshake with a critical security error (`ErrHostKeyMismatch`).
3. **Interactive PTY Bridge (`xterm.js`):**
   - Request PTY (`xterm-256color`, initial cols/rows from frontend) and start shell (`session.Shell()`).
   - Forward stdin bytes from Wails binding `WriteTerminal(sessionID, data)` to PTY stdin.
   - Stream PTY stdout/stderr in a dedicated goroutine emitting `terminal:data:<sessionID>` events.
   - Handle `ResizeTerminal(sessionID, cols, rows)` via `session.WindowChange(rows, cols)`.
   - Send periodic SSH keepalive requests (`keepalive@openssh.com`) respecting `ctx.Done()`.
4. **SFTP Dual-Pane & Cancelable Transfers:**
   - Open an SFTP subsystem (`sftp.NewClient(sshClient)`) sharing the authenticated SSH connection (or dedicated reconnectable handle).
   - Wrap `io.Reader` / `io.Writer` with a context-aware progress reader (`32KB`–`64KB` buffer) that checks `ctx.Err()` on every chunk and emits throttled progress updates.
   - Clean up partial local temp files on cancelled downloads unless resume is explicitly implemented.
