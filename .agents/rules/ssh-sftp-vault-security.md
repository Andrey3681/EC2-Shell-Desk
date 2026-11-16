# Rule: SSH, SFTP & Vault Security

- Derive per-user master keys from local passwords using **Argon2id** with unique random salts stored per user.
- Encrypt all imported SSH private keys (`.pem`, `id_ed25519`, `id_rsa`) at rest with **AES-256-GCM** using unique nonces; never persist plaintext private keys on disk.
- Enforce strict `user_id` (`owner_user_id`) isolation on every query in `sqlite` and every service action.
- Zero/wipe sensitive `[]byte` slices in memory when locking the vault or closing key material where practical.
- Never log private keys, passphrases, user passwords, or decrypted secrets in `slog` logs or returned error strings.
- Never use `ssh.InsecureIgnoreHostKey()` in production code. Implement TOFU (*Trust On First Use*) verification and persist approved fingerprints per user in `known_hosts`.
- Always attach `context.Context` cancellation and timeouts to SSH dialing, PTY loops, and SFTP transfers; clean up goroutines on session close.
- Validate and normalize Windows local paths (`winfs`) and remote POSIX paths (`sftpclient`) to prevent path traversal.
