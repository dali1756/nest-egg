package fx

import (
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
)

// today 回傳台北時區的今天
func today(loc *time.Location) string {
	return time.Now().In(loc).Format("2006-01-02")
}

func parseDay(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, loc)
}

// avgCost = SUM(付出 + 手續費) ÷ SUM(取得)
func avgCost(pay, fee, buy decimal.Decimal) decimal.Decimal {
	return pay.Add(fee).Div(buy)
}

func logConnected(host, user string) {
	slog.Info("database connected", "host", host, "user", user)
}

var fetchTimeout = 30 * time.Second

// 圖表座標比例，不是金額
var chartRatio float64 // money-rules:allow float-go: 圖表座標比例，非金額
