package fx

import (
	"testing"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/rate"
)

func trade(side Side, foreign, twd string, market string) Trade {
	t := Trade{
		Side: side, Currency: "USD", MinorUnit: 2, TWDMinorUnit: 2,
		ForeignAmount: decimal.RequireFromString(foreign), TWDAmount: decimal.RequireFromString(twd),
	}
	if market != "" {
		t.MarketRate = &rate.MarketRate{Value: decimal.RequireFromString(market), Date: taipeiDate("2026-10-27")}
	}
	return t
}

func TestTradeRateIsTWDPerForeignUnit(t *testing.T) {
	for name, tc := range map[string]struct {
		trade Trade
		want  string
	}{
		"exact":               {trade(SideBuy, "1000", "32420", ""), "32.42"},
		"sell":                {trade(SideSell, "500", "16050", ""), "32.1"},
		"rounded to 8 places": {trade(SideBuy, "309.35", "10000", ""), "32.32584451"},
	} {
		if got := tc.trade.TradeRate().String(); got != tc.want {
			t.Errorf("%s: trade rate = %s, want %s", name, got, tc.want)
		}
	}
}

func TestSpreadComparesWithTheMarketInTWD(t *testing.T) {
	for name, tc := range map[string]struct {
		trade Trade
		want  string
	}{
		"buy pays more than the market":      {trade(SideBuy, "1000", "32420", "32.3"), "120"},
		"buy pays less than the market":      {trade(SideBuy, "1000", "32250", "32.3"), "-50"},
		"sell receives less than the market": {trade(SideSell, "500", "16050", "32.25"), "75"},
		"sell receives more than the market": {trade(SideSell, "500", "16200", "32.25"), "-75"},
		"rounded half away from zero":        {trade(SideBuy, "123.45", "4012", "32.5"), "-0.13"},
	} {
		got := tc.trade.Spread()
		if got == nil || !got.Equal(decimal.RequireFromString(tc.want)) {
			t.Errorf("%s: spread = %v, want %s", name, got, tc.want)
		}
	}
}

func TestSpreadRoundsToTheTWDMinorUnit(t *testing.T) {
	tr := trade(SideBuy, "309.35", "10000", "32.338")
	tr.TWDMinorUnit = 0

	if got := tr.Spread(); got == nil || !got.Equal(decimal.RequireFromString("-4")) {
		t.Errorf("spread with 0 TWD decimals = %v, want -4", got)
	}
}

func TestSpreadWithoutAMarketRateIsNone(t *testing.T) {
	if got := trade(SideBuy, "1000", "32420", "").Spread(); got != nil {
		t.Errorf("spread = %v, want nil", got)
	}
}
