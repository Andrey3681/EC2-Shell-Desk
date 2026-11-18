-- 003_command_snippets.sql: Per-user reusable SSH command snippets and deployment macros.

CREATE TABLE IF NOT EXISTS command_snippets (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    group_id      TEXT REFERENCES host_groups(id) ON DELETE SET NULL,
    name          TEXT NOT NULL COLLATE NOCASE,
    command       TEXT NOT NULL,
    auto_execute  INTEGER NOT NULL DEFAULT 1,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_command_snippets_owner ON command_snippets(owner_user_id);
