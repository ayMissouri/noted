-- name: CreateVault :exec
INSERT INTO vaults (id, owner_id, name, created_at, updated_at, change_seq)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetVault :one
SELECT * FROM vaults
WHERE id = ? AND deleted_at IS NULL;

-- name: ListVaults :many
SELECT * FROM vaults
WHERE deleted_at IS NULL
ORDER BY name COLLATE NOCASE;

-- name: RenameVault :exec
UPDATE vaults SET name = ?, updated_at = ?, change_seq = ?
WHERE id = ? AND deleted_at IS NULL;

-- name: SoftDeleteVault :exec
UPDATE vaults SET deleted_at = ?, updated_at = ?, change_seq = ?
WHERE id = ? AND deleted_at IS NULL;
