---
name: database-manager
description: Pure-Go SQLite schema design (modernc.org/sqlite), embedded versioned migrations, indexes, and strict user_id filtering.
---

# Database Manager Skill

Use this skill when working on `internal/adapters/sqlite` and schema migrations.

## Guidelines
1. **Pure Go Driver:** Use `modernc.org/sqlite` (no CGO required on Windows). Enable `PRAGMA foreign_keys = ON;`, `PRAGMA journal_mode = WAL;`, and `PRAGMA busy_timeout = 5000;` on connection open.
2. **Versioned Embedded Migrations:**
   - Store SQL migrations under `internal/adapters/sqlite/migrations/*.sql` using `//go:embed`.
   - Track applied versions in a `schema_migrations` table inside a transaction.
3. **Core Tables & Columns (English):**
   - `users` (`id`, `username`, `password_hash`, `auth_salt`, `vault_salt`, `is_default`, `created_at`, `updated_at`)
   - `ssh_keys` (`id`, `owner_user_id`, `name`, `key_type`, `fingerprint`, `encrypted_pem`, `encrypted_passphrase`, `created_at`)
   - `host_instances` (`id`, `owner_user_id`, `alias`, `host_address`, `port`, `ssh_username`, `ssh_key_id`, `environment`, `notes`, `last_connected_at`, `created_at`, `updated_at`)
   - `known_hosts` (`id`, `owner_user_id`, `host_address`, `port`, `key_type`, `fingerprint`, `public_key_blob`, `trusted_at`)
4. **Strict Ownership Isolation:**
   - Every `SELECT`, `UPDATE`, and `DELETE` on `ssh_keys`, `host_instances`, and `known_hosts` MUST include `WHERE owner_user_id = ?` in addition to `id = ?`.
5. **AppData Storage Path:** On Windows, store the SQLite database in `%APPDATA%\EC2ShellDesk\vault.db` (configurable for tests via `t.TempDir()` or `:memory:`).
