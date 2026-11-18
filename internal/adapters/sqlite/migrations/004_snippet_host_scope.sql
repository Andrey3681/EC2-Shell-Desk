-- 004_snippet_host_scope.sql: Associate command snippets optionally with a specific host instance
-- and scope snippet name uniqueness by (owner_user_id, host_id, group_id, name).

CREATE TABLE IF NOT EXISTS command_snippets_new (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    host_id       TEXT REFERENCES host_instances(id) ON DELETE SET NULL,
    group_id      TEXT REFERENCES host_groups(id) ON DELETE SET NULL,
    name          TEXT NOT NULL COLLATE NOCASE,
    command       TEXT NOT NULL,
    auto_execute  INTEGER NOT NULL DEFAULT 1,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO command_snippets_new (
    id, owner_user_id, host_id, group_id, name, command, auto_execute, notes, created_at, updated_at
)
SELECT
    id, owner_user_id, NULL, group_id, name, command, auto_execute, notes, created_at, updated_at
FROM command_snippets;

DROP TABLE command_snippets;

ALTER TABLE command_snippets_new RENAME TO command_snippets;

CREATE INDEX IF NOT EXISTS idx_command_snippets_owner ON command_snippets(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_command_snippets_host ON command_snippets(owner_user_id, host_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_command_snippets_scope_name
    ON command_snippets(owner_user_id, COALESCE(host_id, ''), COALESCE(group_id, ''), name COLLATE NOCASE);
