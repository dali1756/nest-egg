-- name: MonthlyCost :many
SELECT date_trunc('month', traded_at) AS month,
       SUM(pay_amount + fee) / SUM(buy_amount) AS avg_cost_rate
FROM fx_trades
GROUP BY month;

-- name: TradesOnDay :many
SELECT * FROM fx_trades WHERE traded_at::date = $1;

-- name: TodayRate :one
SELECT rate FROM rate_snapshots WHERE rate_date = CURRENT_DATE;

-- name: TaipeiDay :many
SELECT * FROM fx_trades WHERE (traded_at AT TIME ZONE 'Asia/Taipei')::date = $1;
