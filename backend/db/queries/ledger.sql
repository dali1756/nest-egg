-- name: SeedCategories :exec
-- 使用者還沒有任何分類時才建立；同時多個請求時，後到的被唯一鍵擋下並略過，所以每個使用者只會有一套。
INSERT INTO categories (user_id, kind, name, position)
SELECT sqlc.arg(user_id)::bigint,
       unnest(sqlc.arg(kinds)::text[]),
       unnest(sqlc.arg(names)::text[]),
       unnest(sqlc.arg(positions)::smallint[])
WHERE NOT EXISTS (SELECT 1 FROM categories WHERE user_id = sqlc.arg(user_id)::bigint)
ON CONFLICT (user_id, kind, name) DO NOTHING;

-- name: ListCategories :many
SELECT id, kind, name
FROM categories
WHERE user_id = $1
ORDER BY position, id;

-- name: GetCategory :one
SELECT id, kind, name
FROM categories
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: CreateEntry :one
INSERT INTO ledger_entries (user_id, occurred_on, category_id, currency, amount, market_rate, market_rate_date, note)
VALUES (sqlc.arg(user_id), sqlc.arg(occurred_on), sqlc.arg(category_id), sqlc.arg(currency), sqlc.arg(amount),
        sqlc.narg(market_rate), sqlc.narg(market_rate_date), sqlc.arg(note))
RETURNING id;

-- name: GetEntry :one
SELECT e.id, e.occurred_on, e.category_id, c.kind AS category_kind, c.name AS category_name,
       e.currency, cur.minor_unit, e.amount, e.market_rate, e.market_rate_date, e.note
FROM ledger_entries e
JOIN categories c ON c.id = e.category_id AND c.user_id = e.user_id
JOIN currencies cur ON cur.code = e.currency
WHERE e.id = sqlc.arg(id) AND e.user_id = sqlc.arg(user_id);

-- name: UpdateEntry :execrows
UPDATE ledger_entries
SET occurred_on = sqlc.arg(occurred_on), category_id = sqlc.arg(category_id), currency = sqlc.arg(currency),
    amount = sqlc.arg(amount), market_rate = sqlc.narg(market_rate), market_rate_date = sqlc.narg(market_rate_date),
    note = sqlc.arg(note)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteEntry :execrows
DELETE FROM ledger_entries
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListEntries :many
-- 欄位與 GetEntry 相同；期間是 [from_date, to_date)。日期由新到舊，同一天後建立的在前。
SELECT e.id, e.occurred_on, e.category_id, c.kind AS category_kind, c.name AS category_name,
       e.currency, cur.minor_unit, e.amount, e.market_rate, e.market_rate_date, e.note
FROM ledger_entries e
JOIN categories c ON c.id = e.category_id AND c.user_id = e.user_id
JOIN currencies cur ON cur.code = e.currency
WHERE e.user_id = sqlc.arg(user_id)
  AND e.occurred_on >= sqlc.arg(from_date)
  AND e.occurred_on < sqlc.arg(to_date)
ORDER BY e.occurred_on DESC, e.id DESC;

-- name: SumEntries :many
-- 期間 [from_date, to_date) 內各幣別的收入、支出合計；沒有某一類時是 0。
SELECT e.currency, cur.minor_unit,
       COALESCE(SUM(e.amount) FILTER (WHERE c.kind = 'income'), 0)::numeric  AS income,
       COALESCE(SUM(e.amount) FILTER (WHERE c.kind = 'expense'), 0)::numeric AS expense
FROM ledger_entries e
JOIN categories c ON c.id = e.category_id AND c.user_id = e.user_id
JOIN currencies cur ON cur.code = e.currency
WHERE e.user_id = sqlc.arg(user_id)
  AND e.occurred_on >= sqlc.arg(from_date)
  AND e.occurred_on < sqlc.arg(to_date)
GROUP BY e.currency, cur.minor_unit
ORDER BY e.currency;
