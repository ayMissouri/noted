CREATE TABLE notes_new (
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
    change_seq       INTEGER NOT NULL
);

INSERT INTO notes_new
SELECT id, vault_id, folder_id, name, body, version, created_at, updated_at,
       updated_by_kind, updated_by_user, updated_by_token, trashed_at, deleted_at, change_seq
FROM notes;

DROP TABLE notes;

ALTER TABLE notes_new RENAME TO notes;

CREATE UNIQUE INDEX notes_live_name ON notes(vault_id, folder_id, name)
    WHERE trashed_at IS NULL AND deleted_at IS NULL;
CREATE INDEX notes_by_vault_name ON notes(vault_id, name COLLATE NOCASE);
CREATE INDEX notes_by_change ON notes(vault_id, change_seq);
