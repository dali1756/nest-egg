-- +goose Up
-- 帳號鎖定從永久改成暫時：知道別人 email 的人故意輸錯，最多只能讓對方等 30 分鐘。
-- 原本被永久鎖住的帳號直接解鎖（locked 欄位整個拿掉）。
ALTER TABLE users
    ADD COLUMN locked_until TIMESTAMPTZ,
    ADD COLUMN lock_count   INTEGER NOT NULL DEFAULT 0 CHECK (lock_count >= 0),
    DROP COLUMN locked;

COMMENT ON COLUMN users.locked_until IS '鎖到什麼時候，NULL 或已過去表示沒有鎖定。連續 3 次密碼錯誤時寫入：登入成功後第一次鎖 10 分鐘，之後每次 30 分鐘。緊急時手動解鎖：UPDATE users SET locked_until = NULL, failed_login_count = 0, lock_count = 0 WHERE email = ''...''';
COMMENT ON COLUMN users.lock_count IS '上次登入成功（或重設密碼）之後鎖過幾次，0 表示下一次鎖 10 分鐘，其他鎖 30 分鐘';

-- +goose Down
-- 舊程式的 locked = 1 是永久鎖定，暫時鎖定不轉過去，回滾時一律解鎖。failed_login_count 只會是 0～2，舊程式可以沿用。
ALTER TABLE users
    ADD COLUMN locked SMALLINT NOT NULL DEFAULT 0 CHECK (locked IN (0, 1)),
    DROP COLUMN locked_until,
    DROP COLUMN lock_count;

COMMENT ON COLUMN users.locked IS '0 未封鎖、1 封鎖。連續登入失敗 3 次會設成 1，只能手動改回 0 解鎖';
