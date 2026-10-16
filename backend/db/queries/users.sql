-- name: GetUserByEmail :one
SELECT id, email, password_hash
FROM users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id;

-- name: ReserveLoginAttempt :execrows
UPDATE users
SET failed_login_count = failed_login_count + 1
WHERE id = $1
  AND locked = 0
  AND failed_login_count < sqlc.arg(max_failures);

-- name: LockIfFailuresExhausted :execrows
UPDATE users
SET locked             = 1,
    failed_login_count = 0,
    updated_at         = now()
WHERE id = $1
  AND failed_login_count >= sqlc.arg(max_failures);

-- name: ClearLoginFailures :exec
UPDATE users
SET failed_login_count = 0
WHERE id = $1;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password_hash      = $2,
    failed_login_count = 0,
    updated_at         = now()
WHERE id = $1;
