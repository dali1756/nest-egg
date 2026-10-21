-- name: UpsertRates :one
-- 單一 statement：整批成功或整批不寫。只有數值或來源變了才更新（xmax = 0 代表這列是新增的）。
WITH upserted AS (
    INSERT INTO rate_snapshots (base, quote, rate_date, rate, source)
    -- 同長度的陣列在 SELECT 裡各自 unnest，會逐列配對
    SELECT unnest(sqlc.arg(bases)::text[]),
           unnest(sqlc.arg(quotes)::text[]),
           unnest(sqlc.arg(dates)::date[]),  -- money-rules:allow utc-day-sql: 只是指定參數的型別，傳入的已經是台北日期
           unnest(sqlc.arg(rates)::numeric[]),
           sqlc.arg(source)::text
    ON CONFLICT (base, quote, rate_date) DO UPDATE
        SET rate = EXCLUDED.rate, source = EXCLUDED.source, fetched_at = now()
        WHERE rate_snapshots.rate IS DISTINCT FROM EXCLUDED.rate
           OR rate_snapshots.source IS DISTINCT FROM EXCLUDED.source
    RETURNING xmax = 0 AS inserted
)
SELECT count(*) FILTER (WHERE inserted)     AS inserted,
       count(*) FILTER (WHERE NOT inserted) AS updated
FROM upserted;

-- name: RateOnOrBefore :one
-- 某一天的市場參考匯率：當天或以前最近一筆，最多往回 10 天，不用之後的匯率（CLAUDE.md 設計規則 3）。
SELECT rate_date, rate, source
FROM rate_snapshots
WHERE base = sqlc.arg(base)
  AND quote = sqlc.arg(quote)
  AND rate_date <= sqlc.arg(on_date)
  AND rate_date >= sqlc.arg(on_date) - 10
ORDER BY rate_date DESC
LIMIT 1;
