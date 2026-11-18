-- 005_private_services.sql: Internal private observability and web services running on EC2/VMs.

CREATE TABLE IF NOT EXISTS private_services (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    host_id       TEXT NOT NULL,
    name          TEXT NOT NULL COLLATE NOCASE,
    preset        TEXT NOT NULL DEFAULT 'custom',
    protocol      TEXT NOT NULL DEFAULT 'http',
    remote_host   TEXT NOT NULL DEFAULT '127.0.0.1',
    remote_port   INTEGER NOT NULL,
    base_path     TEXT NOT NULL DEFAULT '',
    local_port    INTEGER NOT NULL DEFAULT 0,
    health_path   TEXT NOT NULL DEFAULT '',
    open_mode     TEXT NOT NULL DEFAULT 'browser',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (host_id) REFERENCES host_instances(id) ON DELETE RESTRICT,
    UNIQUE (owner_user_id, host_id, name)
);

CREATE INDEX IF NOT EXISTS idx_private_services_owner ON private_services(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_private_services_host ON private_services(owner_user_id, host_id);
