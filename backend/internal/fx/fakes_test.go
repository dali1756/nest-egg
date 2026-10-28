package fx

import (
	"context"
	"fmt"
	"time"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/rate"
)

type storedTrade struct {
	userID int64
	Trade
}

type period struct{ from, to time.Time }

type fakeStore struct {
	trades  map[int64]storedTrade
	nextID  int64
	writes  int
	periods []period
}

func newFakeStore() *fakeStore {
	return &fakeStore{trades: map[int64]storedTrade{}}
}

func (f *fakeStore) put(userID, id int64, v TradeValues) {
	f.trades[id] = storedTrade{userID, Trade{
		ID: id, TradedOn: v.TradedOn, Side: v.Side, Currency: v.Currency, MinorUnit: minorUnits[v.Currency],
		ForeignAmount: v.ForeignAmount, TWDAmount: v.TWDAmount, MarketRate: v.MarketRate, Note: v.Note,
	}}
}

func (f *fakeStore) CreateTrade(_ context.Context, userID int64, v TradeValues) (int64, error) {
	f.writes++
	f.nextID++
	f.put(userID, f.nextID, v)
	return f.nextID, nil
}

func (f *fakeStore) Trade(_ context.Context, userID, id int64) (Trade, error) {
	t, ok := f.trades[id]
	if !ok || t.userID != userID {
		return Trade{}, fmt.Errorf("fx trade %d: %w", id, apperr.ErrNotFound)
	}
	return t.Trade, nil
}

func (f *fakeStore) UpdateTrade(ctx context.Context, userID, id int64, v TradeValues) error {
	f.writes++
	if _, err := f.Trade(ctx, userID, id); err != nil {
		return err
	}
	f.put(userID, id, v)
	return nil
}

func (f *fakeStore) DeleteTrade(ctx context.Context, userID, id int64) error {
	f.writes++
	if _, err := f.Trade(ctx, userID, id); err != nil {
		return err
	}
	delete(f.trades, id)
	return nil
}

func (f *fakeStore) Trades(_ context.Context, userID int64, from, to time.Time) ([]Trade, error) {
	f.periods = append(f.periods, period{from, to})
	var out []Trade
	for _, t := range f.trades {
		if t.userID == userID && !t.TradedOn.Before(from) && t.TradedOn.Before(to) {
			out = append(out, t.Trade)
		}
	}
	return out, nil
}

var minorUnits = map[string]int{"TWD": 2, "USD": 2, "JPY": 0}

type fakeCurrencies struct{}

func (fakeCurrencies) Get(_ context.Context, code string) (currency.Currency, error) {
	places, ok := minorUnits[code]
	if !ok {
		return currency.Currency{}, fmt.Errorf("currency %q: %w", code, apperr.ErrNotFound)
	}
	return currency.Currency{Code: code, MinorUnit: places}, nil
}

type rateLookup struct {
	base, quote, date string
}

type fakeRates struct {
	snapshot *rate.Snapshot
	err      error
	lookups  []rateLookup
}

func (f *fakeRates) On(_ context.Context, base, quote string, date time.Time) (rate.Snapshot, error) {
	f.lookups = append(f.lookups, rateLookup{base, quote, date.Format(time.DateOnly)})
	switch {
	case f.err != nil:
		return rate.Snapshot{}, f.err
	case f.snapshot == nil:
		return rate.Snapshot{}, fmt.Errorf("rate %s/%s: %w", base, quote, apperr.ErrNotFound)
	}
	return *f.snapshot, nil
}
