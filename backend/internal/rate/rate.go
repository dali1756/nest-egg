package rate

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// 1 Base = Value Quote，例如 1 USD = 32.617 TWD（CLAUDE.md 設計規則 6）
type Rate struct {
	Base, Quote string
	// 匯率本身的台北日期，不是抓取日期；週六抓到的是週五的匯率
	Date  time.Time
	Value decimal.Decimal
}

type Snapshot struct {
	Rate
	Source string
}

type RateProvider interface {
	Name() string
	Fetch(ctx context.Context) ([]Rate, error)
}
