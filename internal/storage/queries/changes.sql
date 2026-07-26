-- name: NextChangeSeq :one
UPDATE change_counter SET seq = seq + 1 WHERE id = 1
RETURNING seq;

-- name: GetChangeSeq :one
SELECT seq FROM change_counter WHERE id = 1;
