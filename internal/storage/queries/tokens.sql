-- name: CreateToken :exec
INSERT INTO tokens (id, user_id, kind, name, token_hash, scopes, created_at, last_seen_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetTokenByHash :one
SELECT * FROM tokens
WHERE token_hash = ? AND revoked_at IS NULL;

-- name: TouchToken :exec
UPDATE tokens SET last_seen_at = ? WHERE id = ?;

-- name: ListUserTokens :many
SELECT * FROM tokens
WHERE user_id = ? AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeToken :execrows
UPDATE tokens SET revoked_at = ?
WHERE id = ? AND user_id = ? AND revoked_at IS NULL;
