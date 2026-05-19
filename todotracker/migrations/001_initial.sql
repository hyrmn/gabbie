-- 001_initial.sql: Initial schema for todotracker
-- Tables: users, lists, list_collaborators, items, api_keys

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS lists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_id TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS list_collaborators (
    list_id TEXT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner', 'member')),
    invited_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (list_id, user_id)
);

CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    list_id TEXT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo' CHECK(status IN ('todo', 'in_progress', 'done')),
    assignee_id TEXT REFERENCES users(id),
    priority INTEGER NOT NULL DEFAULT 2 CHECK(priority BETWEEN 0 AND 3),
    due_date TEXT,
    tags TEXT NOT NULL DEFAULT '[]',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_used_at TEXT,
    revoked_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_items_list_id ON items(list_id);
CREATE INDEX IF NOT EXISTS idx_items_status ON items(status);
CREATE INDEX IF NOT EXISTS idx_items_assignee ON items(assignee_id);
CREATE INDEX IF NOT EXISTS idx_lists_owner ON lists(owner_id);
CREATE INDEX IF NOT EXISTS idx_collab_user ON list_collaborators(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
