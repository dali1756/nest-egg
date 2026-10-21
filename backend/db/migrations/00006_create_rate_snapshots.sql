-- +goose Up
-- 每日市場參考匯率，所有使用者共用。只存來源真正公布的日子，週末與假日沒有資料列；
-- 「某一天的匯率」由查詢往回找最近一筆（CLAUDE.md 設計規則 3）。
CREATE TABLE rate_snapshots (
    base       CHAR(3)       NOT NULL REFERENCES currencies(code),
    quote      CHAR(3)       NOT NULL REFERENCES currencies(code),
    rate_date  DATE          NOT NULL,  -- 匯率本身的台北日期，不是抓取日期
    rate       NUMERIC(20,8) NOT NULL CHECK (rate > 0),  -- 1 base = rate 個 quote
    source     TEXT          NOT NULL,
    fetched_at TIMESTAMPTZ   NOT NULL DEFAULT now(),  -- 最後一次新增或修正的時間
    PRIMARY KEY (base, quote, rate_date),
    CHECK (base <> quote)
);

-- +goose Down
DROP TABLE rate_snapshots;
