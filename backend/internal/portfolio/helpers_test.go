package portfolio

import (
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/rate"
)

func taipeiDate(s string) time.Time {
	d, err := taipei.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

func decPtr(s string) *decimal.Decimal {
	d := dec(s)
	return &d
}

func marketRate(value, date string) *rate.MarketRate {
	return &rate.MarketRate{Value: dec(value), Date: taipeiDate(date)}
}

func sameDecimal(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func sameMarketRate(a, b *rate.MarketRate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Value.Equal(b.Value) && a.Date.Equal(b.Date)
}
