-- name: CreateFxTrade :one
INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount, market_rate, market_rate_date, note)
VALUES (sqlc.arg(user_id), sqlc.arg(traded_on), sqlc.arg(side), sqlc.arg(currency), sqlc.arg(foreign_amount), sqlc.arg(twd_amount),
        sqlc.narg(market_rate), sqlc.narg(market_rate_date), sqlc.arg(note))
RETURNING id;

-- name: GetFxTrade :one
SELECT t.id, t.traded_on, t.side, t.currency, cur.minor_unit, t.foreign_amount, t.twd_amount,
       t.market_rate, t.market_rate_date, t.note
FROM fx_trades t
JOIN currencies cur ON cur.code = t.currency
WHERE t.id = sqlc.arg(id) AND t.user_id = sqlc.arg(user_id);

-- name: UpdateFxTrade :execrows
UPDATE fx_trades
SET traded_on = sqlc.arg(traded_on), side = sqlc.arg(side), currency = sqlc.arg(currency),
    foreign_amount = sqlc.arg(foreign_amount), twd_amount = sqlc.arg(twd_amount),
    market_rate = sqlc.narg(market_rate), market_rate_date = sqlc.narg(market_rate_date), note = sqlc.arg(note)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteFxTrade :execrows
DELETE FROM fx_trades
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListFxTrades :many
-- 欄位與 GetFxTrade 相同；期間是 [from_date, to_date)。日期由新到舊，同一天後建立的在前。
SELECT t.id, t.traded_on, t.side, t.currency, cur.minor_unit, t.foreign_amount, t.twd_amount,
       t.market_rate, t.market_rate_date, t.note
FROM fx_trades t
JOIN currencies cur ON cur.code = t.currency
WHERE t.user_id = sqlc.arg(user_id)
  AND t.traded_on >= sqlc.arg(from_date)
  AND t.traded_on < sqlc.arg(to_date)
ORDER BY t.traded_on DESC, t.id DESC;
