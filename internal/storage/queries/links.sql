-- name: DeleteNoteLinks :exec
DELETE FROM links WHERE source_note_id = ?;

-- name: InsertLink :exec
INSERT INTO links (source_note_id, ord, kind, target_raw, heading)
VALUES (?, ?, ?, ?, ?);

-- name: ListNoteLinks :many
SELECT * FROM links
WHERE source_note_id = ?
ORDER BY ord;

-- name: ResolveNoteName :one
SELECT id FROM notes
WHERE vault_id = sqlc.arg(vault_id) AND name COLLATE NOCASE = sqlc.arg(name)
  AND trashed_at IS NULL AND deleted_at IS NULL
LIMIT 1;
