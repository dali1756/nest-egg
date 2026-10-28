package fx

import (
	"testing"
	"time"

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

func sameMarketRate(a, b *rate.MarketRate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Value.Equal(b.Value) && a.Date.Equal(b.Date)
}

func wantTrade(t *testing.T, got, want Trade) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("id = %d, want %d", got.ID, want.ID)
	}
	if !got.TradedOn.Equal(want.TradedOn) || got.TradedOn.Location() != taipei.Location {
		t.Errorf("traded on %v, want %v in Asia/Taipei", got.TradedOn, want.TradedOn)
	}
	if got.Side != want.Side || got.Currency != want.Currency || got.MinorUnit != want.MinorUnit {
		t.Errorf("side, currency, minor unit = %s %s %d; want %s %s %d", got.Side, got.Currency, got.MinorUnit, want.Side, want.Currency, want.MinorUnit)
	}
	if !got.ForeignAmount.Equal(want.ForeignAmount) || !got.TWDAmount.Equal(want.TWDAmount) {
		t.Errorf("amounts = %s / %s, want %s / %s", got.ForeignAmount, got.TWDAmount, want.ForeignAmount, want.TWDAmount)
	}
	if !sameMarketRate(got.MarketRate, want.MarketRate) {
		t.Errorf("market rate = %+v, want %+v", got.MarketRate, want.MarketRate)
	}
	if got.Note != want.Note {
		t.Errorf("note = %q, want %q", got.Note, want.Note)
	}
}
