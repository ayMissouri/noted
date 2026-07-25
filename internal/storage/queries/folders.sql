-- name: CreateFolder :exec
INSERT INTO folders (id, vault_id, parent_id, name, path, created_at, updated_at, change_seq)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFolderByPath :one
SELECT * FROM folders
WHERE vault_id = ? AND path = ? AND deleted_at IS NULL;
