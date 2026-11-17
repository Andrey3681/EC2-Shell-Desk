-- 002_ssh_tunnels.sql: Per-user saved SSH local port-forwarding tunnels.

CREATE TABLE IF NOT EXISTS ssh_tunnels (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    host_id       TEXT NOT NULL,
    name          TEXT NOT NULL,
    preset        TEXT NOT NULL DEFAULT 'custom',
    local_host    TEXT NOT NULL DEFAULT '127.0.0.1',
    local_port    INTEGER NOT NULL,
    remote_host   TEXT NOT NULL,
    remote_port   INTEGER NOT NULL,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (host_id) REFERENCES host_instances(id) ON DELETE RESTRICT,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_ssh_tunnels_owner ON ssh_tunnels(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_ssh_tunnels_host ON ssh_tunnels(owner_user_id, host_id);
