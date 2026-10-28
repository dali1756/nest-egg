package fx

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/rate"
)

const (
	userA = int64(1)
	userB = int64(2)
)

var fixedNow = time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)

func newTestService(t *testing.T) (*Service, *fakeStore, *fakeRates) {
	t.Helper()
	store := newFakeStore()
	rates := &fakeRates{}
	svc := NewService(store, fakeCurrencies{}, rates)
	svc.now = func() time.Time { return fixedNow }
	return svc, store, rates
}

func usdSnapshot(date, value string) *rate.Snapshot {
	return &rate.Snapshot{
		Rate:   rate.Rate{Base: "USD", Quote: "TWD", Date: taipeiDate(date), Value: decimal.RequireFromString(value)},
		Source: "cbc",
	}
}

func usdMarketRate(date, value string) *rate.MarketRate {
	return &rate.MarketRate{Value: decimal.RequireFromString(value), Date: taipeiDate(date)}
}

func buyInput() TradeInput {
	return TradeInput{TradedOn: "2026-10-27", Side: "buy", Currency: "USD", ForeignAmount: "1000", TWDAmount: "32420", Note: "10 月定期換匯"}
}

func wantFields(t *testing.T, err error, want ...string) {
	t.Helper()
	var invalid *apperr.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want a validation error on %v", err, want)
	}
	got := slices.Sorted(maps.Keys(invalid.Fields))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("invalid fields = %v, want %v", got, want)
	}
}

func TestCreateTradeRejectsInvalidFields(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*TradeInput)
		want []string
	}{
		"missing date":                  {func(in *TradeInput) { in.TradedOn = "" }, []string{"traded_on"}},
		"date in the future":            {func(in *TradeInput) { in.TradedOn = "2026-10-29" }, []string{"traded_on"}},
		"unknown side":                  {func(in *TradeInput) { in.Side = "swap" }, []string{"side"}},
		"missing side":                  {func(in *TradeInput) { in.Side = "" }, []string{"side"}},
		"missing currency":              {func(in *TradeInput) { in.Currency = "" }, []string{"currency"}},
		"unknown currency":              {func(in *TradeInput) { in.Currency = "XYZ" }, []string{"currency"}},
		"TWD is not a foreign currency": {func(in *TradeInput) { in.Currency = "TWD" }, []string{"currency"}},
		"zero foreign amount":           {func(in *TradeInput) { in.ForeignAmount = "0" }, []string{"foreign_amount"}},
		"malformed foreign amount":      {func(in *TradeInput) { in.ForeignAmount = "1e3" }, []string{"foreign_amount"}},
		"too many USD decimals":         {func(in *TradeInput) { in.ForeignAmount = "1000.123" }, []string{"foreign_amount"}},
		"decimals in JPY":               {func(in *TradeInput) { in.Currency, in.ForeignAmount = "JPY", "23000.5" }, []string{"foreign_amount"}},
		"zero TWD amount":               {func(in *TradeInput) { in.TWDAmount = "0" }, []string{"twd_amount"}},
		"too many TWD decimals":         {func(in *TradeInput) { in.TWDAmount = "32420.123" }, []string{"twd_amount"}},
		"note too long":                 {func(in *TradeInput) { in.Note = strings.Repeat("換", 201) }, []string{"note"}},
		"several fields":                {func(in *TradeInput) { in.Side, in.TWDAmount = "swap", "0" }, []string{"side", "twd_amount"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, _ := newTestService(t)
			in := buyInput()
			tc.edit(&in)

			_, err := svc.CreateTrade(t.Context(), userA, in)

			wantFields(t, err, tc.want...)
			if store.writes != 0 {
				t.Errorf("store writes = %d, want none", store.writes)
			}
		})
	}
}

func TestCreateTradeStoresTheMarketRateOnOrBeforeItsDate(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-16", "32.338")
	in := buyInput()
	in.TradedOn = "2026-10-17"

	got, err := svc.CreateTrade(t.Context(), userA, in)
	if err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	if want := []rateLookup{{"USD", "TWD", "2026-10-17"}}; !slices.Equal(rates.lookups, want) {
		t.Errorf("rate lookups = %v, want %v", rates.lookups, want)
	}
	wantTrade(t, got, Trade{
		ID: got.ID, TradedOn: taipeiDate("2026-10-17"), Side: SideBuy, Currency: "USD", MinorUnit: 2,
		ForeignAmount: decimal.RequireFromString("1000"), TWDAmount: decimal.RequireFromString("32420"),
		MarketRate: usdMarketRate("2026-10-16", "32.338"), Note: "10 月定期換匯",
	})
	if got.TWDMinorUnit != 2 {
		t.Errorf("TWD minor unit = %d, want 2", got.TWDMinorUnit)
	}
	if stored, err := store.Trade(t.Context(), userA, got.ID); err != nil || !sameMarketRate(stored.MarketRate, got.MarketRate) {
		t.Errorf("stored for user A: %+v, %v", stored, err)
	}
}

func TestCreateTradeWithoutAMarketRateStillCreatesIt(t *testing.T) {
	svc, store, _ := newTestService(t)

	got, err := svc.CreateTrade(t.Context(), userA, buyInput())

	if err != nil || got.MarketRate != nil || store.writes != 1 {
		t.Errorf("CreateTrade = %+v, %v with %d writes; want a trade without a market rate", got.MarketRate, err, store.writes)
	}
}

func TestCreateTradeFailsWhenTheRateLookupFails(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.err = errors.New("connection refused")

	if _, err := svc.CreateTrade(t.Context(), userA, buyInput()); !errors.Is(err, rates.err) || store.writes != 0 {
		t.Errorf("CreateTrade error = %v with %d writes; want the lookup error and nothing stored", err, store.writes)
	}
}

func TestCreateTradeCanSellMoreThanWasBought(t *testing.T) {
	svc, _, _ := newTestService(t)
	in := buyInput()
	in.Side, in.ForeignAmount, in.TWDAmount = "sell", "500", "16050"

	got, err := svc.CreateTrade(t.Context(), userA, in)

	if err != nil || got.Side != SideSell {
		t.Errorf("CreateTrade = %+v, %v; want the sell to be recorded", got, err)
	}
}

func createdTrade(t *testing.T, svc *Service, in TradeInput) Trade {
	t.Helper()
	tr, err := svc.CreateTrade(t.Context(), userA, in)
	if err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}
	return tr
}

func TestUpdateTradeKeepsTheStoredMarketRateWhenDateAndCurrencyStay(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-26", "32.4")
	created := createdTrade(t, svc, buyInput())
	rates.snapshot, rates.lookups = usdSnapshot("2026-10-27", "32.3"), nil

	in := buyInput()
	in.Side, in.TWDAmount, in.Note = "sell", "32000", "改過"
	got, err := svc.UpdateTrade(t.Context(), userA, created.ID, in)
	if err != nil {
		t.Fatalf("UpdateTrade: %v", err)
	}

	if len(rates.lookups) != 0 {
		t.Errorf("rate lookups = %v, want none", rates.lookups)
	}
	wantTrade(t, got, Trade{
		ID: created.ID, TradedOn: taipeiDate("2026-10-27"), Side: SideSell, Currency: "USD", MinorUnit: 2,
		ForeignAmount: decimal.RequireFromString("1000"), TWDAmount: decimal.RequireFromString("32000"),
		MarketRate: usdMarketRate("2026-10-26", "32.4"), Note: "改過",
	})
}

func TestUpdateTradeLooksUpTheMarketRateAgainWhenDateOrCurrencyChange(t *testing.T) {
	for name, edit := range map[string]func(*TradeInput){
		"date":     func(in *TradeInput) { in.TradedOn = "2026-10-16" },
		"currency": func(in *TradeInput) { in.Currency, in.ForeignAmount = "JPY", "150000" },
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, rates := newTestService(t)
			rates.snapshot = usdSnapshot("2026-10-26", "32.4")
			created := createdTrade(t, svc, buyInput())
			rates.snapshot, rates.lookups = usdSnapshot("2026-10-16", "32.338"), nil
			in := buyInput()
			edit(&in)

			got, err := svc.UpdateTrade(t.Context(), userA, created.ID, in)
			if err != nil {
				t.Fatalf("UpdateTrade: %v", err)
			}

			if want := []rateLookup{{in.Currency, "TWD", in.TradedOn}}; !slices.Equal(rates.lookups, want) {
				t.Errorf("rate lookups = %v, want %v", rates.lookups, want)
			}
			if !sameMarketRate(got.MarketRate, usdMarketRate("2026-10-16", "32.338")) {
				t.Errorf("market rate = %+v, want the new lookup", got.MarketRate)
			}
		})
	}
}

func TestUpdateTradeOfAnotherUserIsNotFound(t *testing.T) {
	svc, store, _ := newTestService(t)
	created := createdTrade(t, svc, buyInput())
	writes := store.writes

	in := buyInput()
	in.TWDAmount = "1"
	if _, err := svc.UpdateTrade(t.Context(), userB, created.ID, in); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B updates A's trade: %v, want not found", err)
	}
	if store.writes != writes {
		t.Errorf("store writes = %d, want unchanged %d", store.writes, writes)
	}
}

func TestUpdateTradeRejectsInvalidFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := createdTrade(t, svc, buyInput())
	in := buyInput()
	in.ForeignAmount = "0"

	_, err := svc.UpdateTrade(t.Context(), userA, created.ID, in)

	wantFields(t, err, "foreign_amount")
}

func TestTradeReadsOwnTradeWithTheTWDMinorUnit(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := createdTrade(t, svc, buyInput())

	got, err := svc.Trade(t.Context(), userA, created.ID)
	if err != nil || got.ID != created.ID || got.TWDMinorUnit != 2 {
		t.Errorf("Trade = %+v, %v; want trade %d with TWD minor unit 2", got, err, created.ID)
	}
	if _, err := svc.Trade(t.Context(), userB, created.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B reads A's trade: %v, want not found", err)
	}
}

func TestDeleteTradeOfAnotherUserIsNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := createdTrade(t, svc, buyInput())

	if err := svc.DeleteTrade(t.Context(), userB, created.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B deletes A's trade: %v, want not found", err)
	}
	if err := svc.DeleteTrade(t.Context(), userA, created.ID); err != nil {
		t.Errorf("A deletes own trade: %v", err)
	}
}

func TestMonthDefaultsToTheCurrentTaipeiMonth(t *testing.T) {
	svc, store, _ := newTestService(t)
	createdTrade(t, svc, buyInput())

	got, err := svc.Month(t.Context(), userA, "")
	if err != nil {
		t.Fatalf("Month: %v", err)
	}

	if got.Month != "2026-10" {
		t.Errorf("month = %q, want 2026-10", got.Month)
	}
	if want := []period{{taipeiDate("2026-10-01"), taipeiDate("2026-11-01")}}; !slices.Equal(store.periods, want) {
		t.Errorf("store queried %v, want %v", store.periods, want)
	}
	if len(got.Trades) != 1 || got.Trades[0].TWDMinorUnit != 2 {
		t.Errorf("trades = %+v, want one trade with TWD minor unit 2", got.Trades)
	}
}

func TestMonthQueriesTheRequestedMonth(t *testing.T) {
	svc, store, _ := newTestService(t)

	if _, err := svc.Month(t.Context(), userA, "2026-09"); err != nil {
		t.Fatalf("Month: %v", err)
	}

	if want := []period{{taipeiDate("2026-09-01"), taipeiDate("2026-10-01")}}; !slices.Equal(store.periods, want) {
		t.Errorf("store queried %v, want %v", store.periods, want)
	}
}

func TestMonthRejectsAMalformedMonth(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.Month(t.Context(), userA, "2026-13")

	wantFields(t, err, "month")
}

func TestPreviewNewTradeLooksUpTheMarketRateWithoutStoring(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-27", "32.3")

	got, err := svc.PreviewNew(t.Context(), buyInput())
	if err != nil {
		t.Fatalf("PreviewNew: %v", err)
	}

	if store.writes != 0 {
		t.Errorf("store writes = %d, want none", store.writes)
	}
	if got.TradeRate().String() != "32.42" || got.Spread() == nil || got.Spread().StringFixed(2) != "120.00" {
		t.Errorf("preview rate %s spread %v, want 32.42 and 120.00", got.TradeRate(), got.Spread())
	}
	if got.TWDMinorUnit != 2 {
		t.Errorf("TWD minor unit = %d, want 2 so the spread is rounded to cents", got.TWDMinorUnit)
	}
}

func TestPreviewNewTradeRejectsInvalidFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	in := buyInput()
	in.TWDAmount = ""

	_, err := svc.PreviewNew(t.Context(), in)

	wantFields(t, err, "twd_amount")
}

func TestPreviewChangeUsesTheStoredMarketRateWhenDateAndCurrencyStay(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-26", "32.4")
	created := createdTrade(t, svc, buyInput())
	rates.snapshot, rates.lookups = usdSnapshot("2026-10-27", "32.3"), nil
	writes := store.writes

	got, err := svc.PreviewChange(t.Context(), userA, created.ID, buyInput())
	if err != nil {
		t.Fatalf("PreviewChange: %v", err)
	}

	if len(rates.lookups) != 0 || !sameMarketRate(got.MarketRate, usdMarketRate("2026-10-26", "32.4")) {
		t.Errorf("market rate = %+v after lookups %v, want the stored 32.4 without looking up", got.MarketRate, rates.lookups)
	}
	if store.writes != writes {
		t.Errorf("store writes = %d, want unchanged %d", store.writes, writes)
	}
}

func TestPreviewChangeLooksUpAgainWhenTheDateChanges(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-26", "32.4")
	created := createdTrade(t, svc, buyInput())
	rates.snapshot, rates.lookups = usdSnapshot("2026-10-16", "32.338"), nil
	in := buyInput()
	in.TradedOn = "2026-10-16"

	got, err := svc.PreviewChange(t.Context(), userA, created.ID, in)

	if err != nil || len(rates.lookups) != 1 || !sameMarketRate(got.MarketRate, usdMarketRate("2026-10-16", "32.338")) {
		t.Errorf("PreviewChange = %+v, %v after lookups %v; want the new date's rate", got.MarketRate, err, rates.lookups)
	}
}

func TestPreviewChangeOfAnotherUsersTradeIsNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := createdTrade(t, svc, buyInput())

	if _, err := svc.PreviewChange(t.Context(), userB, created.ID, buyInput()); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B previews A's trade: %v, want not found", err)
	}
}
