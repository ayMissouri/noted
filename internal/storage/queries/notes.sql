-- name: CreateNote :exec
INSERT INTO notes (id, vault_id, folder_id, name, body, version, created_at, updated_at, updated_by_kind, updated_by_user, updated_by_token, change_seq)
VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?);

-- name: GetNote :one
SELECT * FROM notes
WHERE id = ? AND deleted_at IS NULL;

-- name: ListNotes :many
SELECT id, vault_id, folder_id, name, version, created_at, updated_at, change_seq
FROM notes
WHERE vault_id = ? AND trashed_at IS NULL AND deleted_at IS NULL
ORDER BY name COLLATE NOCASE;

-- name: ListNotesSince :many
SELECT id, vault_id, folder_id, name, version, created_at, updated_at, change_seq, trashed_at, deleted_at
FROM notes
WHERE vault_id = ? AND change_seq > ?
ORDER BY change_seq;

-- name: TrashNote :execrows
UPDATE notes
SET trashed_at = ?, updated_at = ?, updated_by_kind = ?, updated_by_user = ?, updated_by_token = ?, change_seq = ?
WHERE id = ? AND trashed_at IS NULL AND deleted_at IS NULL;

-- name: RestoreNote :execrows
UPDATE notes
SET trashed_at = NULL, updated_at = ?, updated_by_kind = ?, updated_by_user = ?, updated_by_token = ?, change_seq = ?
WHERE id = ? AND trashed_at IS NOT NULL AND deleted_at IS NULL;

-- name: UpdateNoteBody :execrows
UPDATE notes
SET body = ?, version = version + 1, updated_at = ?, updated_by_kind = ?, updated_by_user = ?, updated_by_token = ?, change_seq = ?
WHERE id = ? AND version = ? AND deleted_at IS NULL;

-- name: SnapshotNoteVersion :exec
INSERT INTO note_versions (note_id, version, body, name, folder_path, saved_at, actor_kind, actor_user, actor_token)
SELECT n.id, n.version, n.body, n.name, f.path, ?, ?, ?, ?
FROM notes n JOIN folders f ON f.id = n.folder_id
WHERE n.id = ?;
