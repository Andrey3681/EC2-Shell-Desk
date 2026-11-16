-- 001_initial_schema.sql: Initial multi-user schema for EC2 Shell Desk.

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash BLOB NOT NULL,
    auth_salt     BLOB NOT NULL,
    vault_salt    BLOB NOT NULL,
    is_default    INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ssh_keys (
    id                   TEXT PRIMARY KEY,
    owner_user_id        TEXT NOT NULL,
    name                 TEXT NOT NULL,
    key_type             TEXT NOT NULL,
    fingerprint          TEXT NOT NULL,
    encrypted_pem        BLOB NOT NULL,
    encrypted_passphrase BLOB,
    has_passphrase       INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_ssh_keys_owner ON ssh_keys(owner_user_id);

CREATE TABLE IF NOT EXISTS host_instances (
    id                TEXT PRIMARY KEY,
    owner_user_id     TEXT NOT NULL,
    alias             TEXT NOT NULL,
    host_address      TEXT NOT NULL,
    port              INTEGER NOT NULL DEFAULT 22,
    ssh_username      TEXT NOT NULL,
    ssh_key_id        TEXT,
    environment       TEXT NOT NULL DEFAULT 'Prod',
    notes             TEXT NOT NULL DEFAULT '',
    last_connected_at TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (ssh_key_id) REFERENCES ssh_keys(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_host_instances_owner ON host_instances(owner_user_id);

CREATE TABLE IF NOT EXISTS known_hosts (
    id              TEXT PRIMARY KEY,
    owner_user_id   TEXT NOT NULL,
    host_address    TEXT NOT NULL,
    port            INTEGER NOT NULL,
    key_type        TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,
    public_key_blob BLOB NOT NULL,
    trusted_at      TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (owner_user_id, host_address, port)
);

CREATE INDEX IF NOT EXISTS idx_known_hosts_owner_host ON known_hosts(owner_user_id, host_address, port);
