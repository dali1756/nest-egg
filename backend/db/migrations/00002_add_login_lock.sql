-- +goose Up
ALTER TABLE users
    ADD COLUMN failed_login_count SMALLINT NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
    ADD COLUMN locked             SMALLINT NOT NULL DEFAULT 0 CHECK (locked IN (0, 1));

COMMENT ON COLUMN users.locked IS '0 未封鎖、1 封鎖。連續登入失敗 3 次會設成 1，只能手動改回 0 解鎖';

-- +goose Down
ALTER TABLE users
    DROP COLUMN locked,
    DROP COLUMN failed_login_count;
