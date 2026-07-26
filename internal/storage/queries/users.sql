-- name: CreateUser :exec
INSERT INTO users (id, username, email, password_hash, is_admin, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetUser :one
SELECT * FROM users
WHERE id = ? AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ? AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
ORDER BY username COLLATE NOCASE;

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE deleted_at IS NULL;
