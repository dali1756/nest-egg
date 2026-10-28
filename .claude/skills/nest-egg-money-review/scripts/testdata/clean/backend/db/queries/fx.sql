-- name: MonthlyCost :many
SELECT date_trunc('month', traded_at AT TIME ZONE 'Asia/Taipei') AS month,
       SUM(twd_amount) / SUM(foreign_amount)                    AS avg_cost_rate,
       SUM(foreign_amount)                                      AS total_bought
FROM fx_trades
WHERE currency = $1 AND side = 'buy'
GROUP BY month
ORDER BY month;
