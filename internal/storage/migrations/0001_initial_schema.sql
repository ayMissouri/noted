CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    email         TEXT UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    deleted_at    TEXT
);

-- One row per authentication (browser, CLI login, MCP connection).
CREATE TABLE tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    kind         TEXT NOT NULL CHECK (kind IN ('web','cli','pat','mcp')),
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    scopes       TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    last_seen_at TEXT,
    expires_at   TEXT,
    revoked_at   TEXT
);

CREATE TABLE vaults (
    id         TEXT PRIMARY KEY,
    owner_id   TEXT REFERENCES users(id),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    change_seq INTEGER NOT NULL
);

CREATE TABLE folders (
    id         TEXT PRIMARY KEY,
    vault_id   TEXT NOT NULL REFERENCES vaults(id),
    parent_id  TEXT REFERENCES folders(id),
    name       TEXT NOT NULL,
    path       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    change_seq INTEGER NOT NULL,
    UNIQUE (vault_id, path)
);

CREATE TABLE notes (
    id               TEXT PRIMARY KEY,
    vault_id         TEXT NOT NULL REFERENCES vaults(id),
    folder_id        TEXT NOT NULL REFERENCES folders(id),
    name             TEXT NOT NULL,
    body             TEXT NOT NULL,
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    updated_by_kind  TEXT NOT NULL CHECK (updated_by_kind IN ('user','agent','system','import')),
    updated_by_user  TEXT REFERENCES users(id),
    updated_by_token TEXT REFERENCES tokens(id),
    trashed_at       TEXT,
    deleted_at       TEXT,
    change_seq       INTEGER NOT NULL,
    UNIQUE (vault_id, folder_id, name)
);
CREATE INDEX notes_by_vault_name ON notes(vault_id, name COLLATE NOCASE);
CREATE INDEX notes_by_change ON notes(vault_id, change_seq);

CREATE TABLE note_versions (
    note_id     TEXT NOT NULL REFERENCES notes(id),
    version     INTEGER NOT NULL,
    body        TEXT NOT NULL,
    name        TEXT NOT NULL,
    folder_path TEXT NOT NULL,
    saved_at    TEXT NOT NULL,
    actor_kind  TEXT NOT NULL CHECK (actor_kind IN ('user','agent','system','import')),
    actor_user  TEXT REFERENCES users(id),
    actor_token TEXT REFERENCES tokens(id),
    PRIMARY KEY (note_id, version)
);
CREATE INDEX note_versions_by_token ON note_versions(actor_token);

CREATE TABLE change_counter (
    id  INTEGER PRIMARY KEY CHECK (id = 1),
    seq INTEGER NOT NULL
);
INSERT INTO change_counter (id, seq) VALUES (1, 0);

CREATE TABLE server_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
