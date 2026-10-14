-- +goose Up
-- float 寫在註解裡不算
CREATE TABLE fx_trades (
    id          BIGSERIAL PRIMARY KEY,
    buy_amount  REAL NOT NULL,
    pay_amount  double precision NOT NULL,
    fee         FLOAT8 NOT NULL DEFAULT 0,
    trade_rate  float NOT NULL,
    market_rate FLOAT4,
    realized    BOOLEAN NOT NULL DEFAULT false
);
