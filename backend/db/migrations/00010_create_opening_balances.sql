-- +goose Up
-- 期初餘額：開始使用 nest-egg 前已經有的錢，每個使用者每個幣別最多一筆。
-- 外幣查不到 opened_on 的匯率時 twd_cost 必填；台幣不可以有 twd_cost、不存匯率。這些由 service 檢查，不寫成 CHECK（CLAUDE.md 設計規則 5：不在 SQL 寫死幣別）。
CREATE TABLE opening_balances (
    id               BIGSERIAL     PRIMARY KEY,
    user_id          BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency         CHAR(3)       NOT NULL REFERENCES currencies(code),
    -- 台北日期；amount 是這天結束時的餘額，該幣別這天（含）以前的收支與換匯不算進餘額與成本
    opened_on        DATE          NOT NULL,
    amount           NUMERIC(20,4) NOT NULL CHECK (amount > 0),
    -- 外幣才有，使用者填的台幣總額；NULL 表示用 amount × market_rate 換算
    twd_cost         NUMERIC(20,4) CHECK (twd_cost > 0),
    -- opened_on 的市場參考匯率（1 外幣 = X 台幣），只有 opened_on 改變才重新查（CLAUDE.md 設計規則 2、3）
    market_rate      NUMERIC(20,8),
    market_rate_date DATE,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (user_id, currency),
    CHECK ((market_rate IS NULL) = (market_rate_date IS NULL))
);

-- +goose Down
DROP TABLE opening_balances;
