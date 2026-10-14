-- name: MonthlyCost :many
SELECT date_trunc('month', traded_at AT TIME ZONE 'Asia/Taipei') AS month,
       SUM(pay_amount + fee) / SUM(buy_amount)                  AS avg_cost_rate,
       SUM(buy_amount)                                          AS total_bought
FROM fx_trades
WHERE buy_currency = $1 AND pay_currency = $2
GROUP BY month
ORDER BY month;
