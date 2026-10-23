-- +goose Up
-- 收支紀錄。收入或支出由分類的 kind 決定，不另外存。
CREATE TABLE ledger_entries (
    id               BIGSERIAL     PRIMARY KEY,
    user_id          BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    occurred_on      DATE          NOT NULL,  -- 台北日期
    category_id      BIGINT        NOT NULL,
    currency         CHAR(3)       NOT NULL REFERENCES currencies(code),
    amount           NUMERIC(20,4) NOT NULL CHECK (amount > 0),
    -- 外幣才有：記錄當下查得到的市場參考匯率（1 外幣 = X 台幣），之後不自動更新（CLAUDE.md 設計規則 2、3）
    market_rate      NUMERIC(20,8),
    market_rate_date DATE,
    note             TEXT          NOT NULL DEFAULT '' CHECK (char_length(note) <= 200),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- 複合外鍵：資料庫層也保證只能用自己的分類
    FOREIGN KEY (category_id, user_id) REFERENCES categories (id, user_id),
    CHECK ((market_rate IS NULL) = (market_rate_date IS NULL))
);

CREATE INDEX ledger_entries_user_month ON ledger_entries (user_id, occurred_on);

-- +goose Down
DROP TABLE ledger_entries;
