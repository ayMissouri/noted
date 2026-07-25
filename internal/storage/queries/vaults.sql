-- name: CreateVault :exec
INSERT INTO vaults (id, owner_id, name, created_at, updated_at, change_seq)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListVaults :many
SELECT * FROM vaults
WHERE deleted_at IS NULL
ORDER BY name COLLATE NOCASE;
