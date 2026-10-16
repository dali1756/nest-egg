-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, last_seen_at, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetSessionByTokenHash :one
SELECT s.id, s.user_id, u.email, s.last_seen_at, s.expires_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = $2
WHERE id = $1;

-- name: DeleteSession :one
DELETE FROM sessions
WHERE token_hash = $1
RETURNING user_id;

-- name: DeleteStaleSessions :exec
DELETE FROM sessions
WHERE user_id = $1
  AND (expires_at <= sqlc.arg(now) OR last_seen_at <= sqlc.arg(idle_before));

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE user_id = $1;
