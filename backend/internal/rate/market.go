package rate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
)

type MarketRate struct {
	Value decimal.Decimal
	Date  time.Time
}

func (m *MarketRate) Columns() (*decimal.Decimal, *time.Time) {
	if m == nil {
		return nil, nil
	}
	return &m.Value, &m.Date
}

func MarketRateFromColumns(value *decimal.Decimal, date *time.Time) *MarketRate {
	if value == nil || date == nil {
		return nil
	}
	return &MarketRate{Value: *value, Date: taipei.FromDBDate(*date)}
}

type Finder interface {
	On(ctx context.Context, base, quote string, date time.Time) (Snapshot, error)
}

func MarketRateInTWD(ctx context.Context, rates Finder, cur string, date time.Time) (*MarketRate, error) {
	if cur == currency.TWD {
		return nil, nil
	}
	snap, err := rates.On(ctx, cur, currency.TWD, date)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("look up %s market rate: %w", cur, err)
	}
	return &MarketRate{Value: snap.Value, Date: snap.Date}, nil
}
