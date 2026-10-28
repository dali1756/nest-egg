-- +goose Up
-- 換匯交易，一邊固定是台幣。成交匯率 = twd_amount ÷ foreign_amount，由系統算出、不存；銀行的成本已含在牌告匯率裡，沒有手續費欄位。
-- 外幣不能是 TWD 由 service 檢查，不寫成 CHECK（CLAUDE.md 設計規則 5：不在 SQL 寫死幣別）。
CREATE TABLE fx_trades (
    id               BIGSERIAL     PRIMARY KEY,
    user_id          BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    traded_on        DATE          NOT NULL,  -- 台北日期
    side             TEXT          NOT NULL CHECK (side IN ('buy', 'sell')),  -- buy：台幣換外幣；sell：外幣換台幣。money-rules:allow currency-list: buy、sell 是換匯方向，不是幣別
    currency         CHAR(3)       NOT NULL REFERENCES currencies(code),      -- 外幣
    foreign_amount   NUMERIC(20,4) NOT NULL CHECK (foreign_amount > 0),
    twd_amount       NUMERIC(20,4) NOT NULL CHECK (twd_amount > 0),
    -- 記錄當下查得到的市場參考匯率（1 外幣 = X 台幣），之後不自動更新（CLAUDE.md 設計規則 2、3）
    market_rate      NUMERIC(20,8),
    market_rate_date DATE,
    note             TEXT          NOT NULL DEFAULT '' CHECK (char_length(note) <= 200),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CHECK ((market_rate IS NULL) = (market_rate_date IS NULL))
);

CREATE INDEX fx_trades_user_date ON fx_trades (user_id, traded_on);

-- +goose Down
DROP TABLE fx_trades;
