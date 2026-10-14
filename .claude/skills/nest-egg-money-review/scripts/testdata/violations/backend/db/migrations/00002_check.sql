-- +goose Up
ALTER TABLE fx_trades ADD CONSTRAINT buy_currency_check CHECK (buy_currency IN ('TWD', 'USD'));
