package portfolio

import (
	"context"
	"fmt"
	"time"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/rate"
)

type openingKey struct {
	userID   int64
	currency string
}

type fakeStore struct {
	openings  map[openingKey]Opening
	movements map[int64][]Movement
	writes    int
	readErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{openings: map[openingKey]Opening{}, movements: map[int64][]Movement{}}
}

func (f *fakeStore) put(userID int64, o Opening) {
	f.openings[openingKey{userID, o.Currency}] = o
}

func (f *fakeStore) SetOpening(_ context.Context, userID int64, cur string, v OpeningValues) (bool, error) {
	f.writes++
	key := openingKey{userID, cur}
	_, existed := f.openings[key]
	f.openings[key] = Opening{
		Currency: cur, MinorUnit: fakeCurrencyList[cur].MinorUnit, OpenedOn: v.OpenedOn,
		Amount: v.Amount, TWDCost: v.TWDCost, MarketRate: v.MarketRate,
	}
	return !existed, nil
}

func (f *fakeStore) Opening(_ context.Context, userID int64, cur string) (Opening, error) {
	if f.readErr != nil {
		return Opening{}, f.readErr
	}
	o, ok := f.openings[openingKey{userID, cur}]
	if !ok {
		return Opening{}, fmt.Errorf("%s opening balance: %w", cur, apperr.ErrNotFound)
	}
	return o, nil
}

func (f *fakeStore) Openings(_ context.Context, userID int64) ([]Opening, error) {
	var out []Opening
	for _, code := range []string{"JPY", "TWD", "USD"} {
		if o, ok := f.openings[openingKey{userID, code}]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteOpening(ctx context.Context, userID int64, cur string) error {
	f.writes++
	if _, err := f.Opening(ctx, userID, cur); err != nil {
		return err
	}
	delete(f.openings, openingKey{userID, cur})
	return nil
}

func (f *fakeStore) Movements(_ context.Context, userID int64) ([]Movement, error) {
	return f.movements[userID], nil
}

var fakeCurrencyList = map[string]currency.Currency{"JPY": jpy, "TWD": twd, "USD": usd}

type fakeCurrencies struct{}

func (fakeCurrencies) Get(_ context.Context, code string) (currency.Currency, error) {
	c, ok := fakeCurrencyList[code]
	if !ok {
		return currency.Currency{}, fmt.Errorf("currency %q: %w", code, apperr.ErrNotFound)
	}
	return c, nil
}

func (fakeCurrencies) List(context.Context) ([]currency.Currency, error) {
	return allCurrencies, nil
}

type rateLookup struct {
	base, quote, date string
}

type fakeRates struct {
	snapshots map[string]rate.Snapshot
	err       error
	lookups   []rateLookup
}

func (f *fakeRates) On(_ context.Context, base, quote string, date time.Time) (rate.Snapshot, error) {
	day := date.Format(time.DateOnly)
	f.lookups = append(f.lookups, rateLookup{base, quote, day})
	if f.err != nil {
		return rate.Snapshot{}, f.err
	}
	snap, ok := f.snapshots[base+" "+day]
	if !ok {
		return rate.Snapshot{}, fmt.Errorf("rate %s/%s: %w", base, quote, apperr.ErrNotFound)
	}
	return snap, nil
}
