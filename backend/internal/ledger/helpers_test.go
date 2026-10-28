package ledger

import (
	"testing"
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

func usdSnapshot(date, value string) *rate.Snapshot {
	return &rate.Snapshot{
		Rate:   rate.Rate{Base: "USD", Quote: "TWD", Date: taipeiDate(date), Value: decimal.RequireFromString(value)},
		Source: "cbc",
	}
}

func wantMarketRate(t *testing.T, got *rate.MarketRate, value, date string) {
	t.Helper()
	want := &rate.MarketRate{Value: decimal.RequireFromString(value), Date: taipeiDate(date)}
	if !sameMarketRate(got, want) {
		t.Errorf("market rate = %+v, want %s on %s", got, value, date)
	}
}

func sameMarketRate(a, b *rate.MarketRate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Value.Equal(b.Value) && a.Date.Equal(b.Date)
}

func wantEntry(t *testing.T, got, want Entry) {
	t.Helper()
	if !got.OccurredOn.Equal(want.OccurredOn) || got.OccurredOn.Location() != taipei.Location {
		t.Errorf("occurred on %v, want %v in Asia/Taipei", got.OccurredOn, want.OccurredOn)
	}
	if got.Category != want.Category {
		t.Errorf("category = %+v, want %+v", got.Category, want.Category)
	}
	if got.Currency != want.Currency || got.MinorUnit != want.MinorUnit {
		t.Errorf("currency = %s with %d decimals, want %s with %d", got.Currency, got.MinorUnit, want.Currency, want.MinorUnit)
	}
	if !got.Amount.Equal(want.Amount) {
		t.Errorf("amount = %s, want %s", got.Amount, want.Amount)
	}
	if !sameMarketRate(got.MarketRate, want.MarketRate) {
		t.Errorf("market rate = %+v, want %+v", got.MarketRate, want.MarketRate)
	}
	if got.Note != want.Note {
		t.Errorf("note = %q, want %q", got.Note, want.Note)
	}
}
