-- name: SetOpeningBalance :one
-- 沒有就新增、有就取代。xmax = 0 表示這一列是剛插入的，不是 ON CONFLICT 更新的。
INSERT INTO opening_balances (user_id, currency, opened_on, amount, twd_cost, market_rate, market_rate_date)
VALUES (sqlc.arg(user_id), sqlc.arg(currency), sqlc.arg(opened_on), sqlc.arg(amount),
        sqlc.narg(twd_cost), sqlc.narg(market_rate), sqlc.narg(market_rate_date))
ON CONFLICT (user_id, currency) DO UPDATE
SET opened_on = EXCLUDED.opened_on, amount = EXCLUDED.amount, twd_cost = EXCLUDED.twd_cost,
    market_rate = EXCLUDED.market_rate, market_rate_date = EXCLUDED.market_rate_date
RETURNING (xmax = 0)::boolean AS created;

-- name: GetOpeningBalance :one
SELECT o.currency, cur.minor_unit, o.opened_on, o.amount, o.twd_cost, o.market_rate, o.market_rate_date
FROM opening_balances o
JOIN currencies cur ON cur.code = o.currency
WHERE o.user_id = sqlc.arg(user_id) AND o.currency = sqlc.arg(currency);

-- name: ListOpeningBalances :many
-- 欄位與 GetOpeningBalance 相同，依幣別代碼排序。
SELECT o.currency, cur.minor_unit, o.opened_on, o.amount, o.twd_cost, o.market_rate, o.market_rate_date
FROM opening_balances o
JOIN currencies cur ON cur.code = o.currency
WHERE o.user_id = sqlc.arg(user_id)
ORDER BY o.currency;

-- name: DeleteOpeningBalance :execrows
DELETE FROM opening_balances
WHERE user_id = sqlc.arg(user_id) AND currency = sqlc.arg(currency);

-- name: ListBalanceMovements :many
-- 自己的所有收支與換匯，算各幣別總額用。kind 是 income、expense（收支）或 buy、sell（換匯）。
-- amount 是收支的金額或換匯的外幣金額；twd_amount 只有換匯有，收支是 0；market_rate 只有外幣收支有。
-- 順序由 Go 決定（portfolio.Summarize），這裡不排序。
SELECT 'ledger'::text AS source, e.id, e.occurred_on, e.created_at, c.kind,
       e.currency, e.amount, 0::numeric AS twd_amount, e.market_rate
FROM ledger_entries e
JOIN categories c ON c.id = e.category_id AND c.user_id = e.user_id
WHERE e.user_id = sqlc.arg(user_id)
UNION ALL
SELECT 'fx'::text, t.id, t.traded_on, t.created_at, t.side,
       t.currency, t.foreign_amount, t.twd_amount, NULL::numeric
FROM fx_trades t
WHERE t.user_id = sqlc.arg(user_id);
