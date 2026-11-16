---
name: security-auth
description: Local multi-user authentication, Argon2id key derivation, AES-256-GCM vault encryption, memory wiping, and user_id isolation.
---

# Security & Authentication Skill

Use this skill when designing or implementing `internal/adapters/crypto`, `internal/services/auth_service.go`, `internal/services/vault_service.go`, and user isolation checks.

## Guidelines
1. **Password Hashing & Key Derivation (Argon2id):**
   - Store a unique random 16-byte salt per user (`crypto/rand`).
   - Use `golang.org/x/crypto/argon2.IDKey` with strong parameters (e.g., `time=3`, `memory=64*1024` KiB, `threads=4`, `keyLen=32`) to derive a 256-bit key.
   - Use separate domain-separated salts or HKDF-derived subkeys for authentication verification vs. vault master encryption key (or store an Argon2id verifier hash separate from the master key derivation salt).
2. **AES-256-GCM Encryption at Rest:**
   - Encrypt each SSH private key (and optional passphrase) with AES-256-GCM using a fresh 12-byte random nonce per encryption operation (`cipher.NewGCM`).
   - Bind Additional Authenticated Data (AAD) such as `userID + ":" + keyID` so ciphertext cannot be transplanted across users or records.
3. **Initial User Bootstrap:**
   - On first run (0 users in SQLite), support initial local administrator/default user setup cleanly without hardcoded weak default passwords in production binaries.
4. **Memory Hygiene & Zero Logging:**
   - Provide a helper `ZeroBytes(b []byte)` and invoke it via `defer` after parsing private keys or locking the user session.
   - Never include secret values in errors or `log/slog` attributes.
