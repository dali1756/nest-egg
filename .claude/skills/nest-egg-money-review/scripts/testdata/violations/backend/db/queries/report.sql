-- name: AvgRate :one
SELECT AVG(trade_rate) FROM fx_trades;

-- name: AvgMarket :one
SELECT avg( market_rate ) AS m FROM fx_trades;

-- name: AvgAmount :one
SELECT AVG(buy_amount) FROM fx_trades;
