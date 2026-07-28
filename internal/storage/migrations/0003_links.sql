CREATE TABLE links (
    source_note_id TEXT NOT NULL REFERENCES notes(id),
    ord            INTEGER NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('wikilink','embed','markdown')),
    target_raw     TEXT NOT NULL,
    heading        TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source_note_id, ord)
);

CREATE INDEX links_by_target ON links(target_raw COLLATE NOCASE);
