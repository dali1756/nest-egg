-- +goose Up
-- 收支分類，每個使用者各自一套；預設分類在使用者第一次取得分類時才建立。
CREATE TABLE categories (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL CHECK (kind IN ('income', 'expense')),
    name       TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 20),
    position   SMALLINT    NOT NULL,  -- 顯示順序
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, kind, name),
    UNIQUE (id, user_id)              -- 讓 ledger_entries 用複合外鍵確保只能用自己的分類
);

-- +goose Down
DROP TABLE categories;
