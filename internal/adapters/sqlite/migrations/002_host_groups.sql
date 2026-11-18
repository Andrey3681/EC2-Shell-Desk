-- 002_host_groups.sql: User-defined host groups for organizing EC2 instances and VMs.

CREATE TABLE IF NOT EXISTS host_groups (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    name          TEXT NOT NULL COLLATE NOCASE,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_host_groups_owner ON host_groups(owner_user_id);

ALTER TABLE host_instances ADD COLUMN group_id TEXT REFERENCES host_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_host_instances_group ON host_instances(owner_user_id, group_id);
