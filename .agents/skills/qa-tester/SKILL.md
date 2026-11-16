---
name: qa-tester
description: QA test planning, edge-case verification for Windows paths, SSH network failures, TOFU prompts, and multi-user isolation.
---

# QA Tester Skill

Use this skill when creating or executing verification matrices (`docs/verificacion.md`) and end-to-end tests.

## Guidelines
1. **Core Test Scenarios:**
   - First-launch initial user creation and subsequent password login (valid/invalid passwords).
   - Multi-user isolation: User A imports `.pem` and creates EC2 host; User B logs in and verifies zero visibility of User A's keys, hosts, or `known_hosts`.
   - Key vault formats: RSA PEM, Ed25519 OpenSSH, ECDSA, encrypted private keys with passphrase, invalid/corrupted files, and `.ppk` guidance message.
   - SSH PTY interaction: resize window (`WindowChange`), ANSI colors, interactive tools (`top`/`less`), keepalive, and clean disconnect on tab close.
   - TOFU host verification: prompt on first connection, reject/warn loudly on host key mismatch (MITM check).
   - SFTP dual-pane: upload/download large files (>10MB) with progress events, mid-transfer cancellation without partial leaks or panics, Windows drive switching (`C:\`, `D:\`), filename conflicts, and permission denied handling.
2. **Test Environment:** Use local Docker OpenSSH server or embedded Go SSH test server for automated checks, plus manual checklist against a live AWS EC2 instance.
