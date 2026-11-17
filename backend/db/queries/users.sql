-- name: GetUserByEmail :one
SELECT id, email, password_hash
FROM users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id;

-- name: ReserveLoginAttempt :one
-- 比對密碼前先預留一次嘗試，同時送出的請求也只能用完 max_failures 次。一個陳述式完成，FOR UPDATE 讓同時的預留排隊：
--   鎖定中：什麼都不改，回報被拒絕與解鎖時間。
--   這次用完次數：在比對密碼前就上鎖（比對正確才由 ClearLoginFailures 解除），比對中斷也只鎖到這個時間。
--     登入成功後第一次鎖到 first_lock_until，之後鎖到 repeat_lock_until。
--   其他：失敗次數 +1，過期的鎖定時間清掉。
WITH current_user_lock AS (
    SELECT users.id,
        users.failed_login_count,
        users.lock_count,
        users.locked_until,
        COALESCE(users.locked_until > sqlc.arg(now)::timestamptz, false)    AS was_locked,
        users.failed_login_count + 1 >= sqlc.arg(max_failures)::smallint AS uses_last_attempt
    FROM users
    WHERE users.id = sqlc.arg(user_id)
    FOR UPDATE
)
UPDATE users u
SET failed_login_count = CASE
                            WHEN c.was_locked THEN c.failed_login_count
                            WHEN c.uses_last_attempt THEN 0
                            ELSE c.failed_login_count + 1
                        END,
    lock_count         = CASE
                            WHEN NOT c.was_locked AND c.uses_last_attempt THEN c.lock_count + 1
                            ELSE c.lock_count
                        END,
    locked_until       = CASE
                            WHEN c.was_locked THEN c.locked_until
                            WHEN c.uses_last_attempt AND c.lock_count = 0 THEN sqlc.arg(first_lock_until)::timestamptz
                            WHEN c.uses_last_attempt THEN sqlc.arg(repeat_lock_until)::timestamptz
                        END
FROM current_user_lock c
WHERE u.id = c.id
RETURNING c.was_locked::boolean AS was_locked, u.locked_until;

-- name: ClearLoginFailures :exec
-- 密碼比對正確：失敗次數歸零、解除鎖定，下一次鎖定回到 first_lock_until
UPDATE users
SET failed_login_count = 0,
    lock_count         = 0,
    locked_until       = NULL
WHERE id = $1;

-- name: UpdatePasswordHash :exec
-- 換密碼一併解除鎖定：重設密碼要解鎖；變更密碼與重算雜湊都已經比對成功，本來就不會在鎖定中
UPDATE users
SET password_hash      = $2,
    failed_login_count = 0,
    lock_count         = 0,
    locked_until       = NULL,
    updated_at         = now()
WHERE id = $1;
