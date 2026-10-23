package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/rate"
)

type fakeStore struct {
	categories  map[int64][]Category
	entries     map[int64]fakeEntry
	nextID      int64
	nextEntryID int64
	err         error
	totals      []Total
	monthCalls  []monthCall
}

type monthCall struct {
	userID   int64
	from, to time.Time
}

type fakeEntry struct {
	userID int64
	Entry
}

func newFakeStore() *fakeStore {
	return &fakeStore{categories: map[int64][]Category{}, entries: map[int64]fakeEntry{}}
}

func (f *fakeStore) SeedCategories(_ context.Context, userID int64, categories []Category) error {
	if f.err != nil {
		return f.err
	}
	if len(f.categories[userID]) > 0 {
		return nil
	}
	for _, c := range categories {
		f.nextID++
		f.categories[userID] = append(f.categories[userID], Category{ID: f.nextID, Kind: c.Kind, Name: c.Name})
	}
	return nil
}

func (f *fakeStore) Categories(_ context.Context, userID int64) ([]Category, error) {
	return f.categories[userID], f.err
}

var errStore = errors.New("store failed")

func (f *fakeStore) Category(_ context.Context, userID, id int64) (Category, error) {
	for _, c := range f.categories[userID] {
		if c.ID == id {
			return c, nil
		}
	}
	return Category{}, fmt.Errorf("category %d: %w", id, apperr.ErrNotFound)
}

func (f *fakeStore) CreateEntry(ctx context.Context, userID int64, v EntryValues) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	category, err := f.Category(ctx, userID, v.CategoryID)
	if err != nil {
		return 0, err
	}
	f.nextEntryID++
	f.entries[f.nextEntryID] = fakeEntry{userID: userID, Entry: Entry{
		ID: f.nextEntryID, OccurredOn: v.OccurredOn, Category: category, Currency: v.Currency, MinorUnit: 2,
		Amount: v.Amount, MarketRate: v.MarketRate, Note: v.Note,
	}}
	return f.nextEntryID, nil
}

func (f *fakeStore) Entry(_ context.Context, userID, id int64) (Entry, error) {
	e, ok := f.entries[id]
	if !ok || e.userID != userID {
		return Entry{}, fmt.Errorf("entry %d: %w", id, apperr.ErrNotFound)
	}
	return e.Entry, nil
}

type fakeCurrencies struct{}

func (fakeCurrencies) Get(_ context.Context, code string) (currency.Currency, error) {
	switch code {
	case "TWD":
		return currency.Currency{Code: "TWD", Name: "新臺幣", MinorUnit: 2}, nil
	case "USD":
		return currency.Currency{Code: "USD", Name: "美元", MinorUnit: 2}, nil
	}
	return currency.Currency{}, fmt.Errorf("currency %q: %w", code, apperr.ErrNotFound)
}

type rateCall struct {
	base, quote string
	date        time.Time
}

type fakeRates struct {
	snapshot *rate.Snapshot
	err      error
	calls    []rateCall
}

func (f *fakeRates) On(_ context.Context, base, quote string, date time.Time) (rate.Snapshot, error) {
	f.calls = append(f.calls, rateCall{base, quote, date})
	switch {
	case f.err != nil:
		return rate.Snapshot{}, f.err
	case f.snapshot == nil:
		return rate.Snapshot{}, fmt.Errorf("rate %s/%s: %w", base, quote, apperr.ErrNotFound)
	}
	return *f.snapshot, nil
}

func (f *fakeStore) UpdateEntry(ctx context.Context, userID, id int64, v EntryValues) error {
	e, err := f.Entry(ctx, userID, id)
	if err != nil {
		return err
	}
	category, err := f.Category(ctx, userID, v.CategoryID)
	if err != nil {
		return err
	}
	e.OccurredOn, e.Category, e.Currency, e.Amount, e.MarketRate, e.Note = v.OccurredOn, category, v.Currency, v.Amount, v.MarketRate, v.Note
	f.entries[id] = fakeEntry{userID: userID, Entry: e}
	return nil
}

func (f *fakeStore) Entries(_ context.Context, userID int64, from, to time.Time) ([]Entry, error) {
	f.monthCalls = append(f.monthCalls, monthCall{userID, from, to})
	var out []Entry
	for _, e := range f.entries {
		if e.userID == userID && !e.OccurredOn.Before(from) && e.OccurredOn.Before(to) {
			out = append(out, e.Entry)
		}
	}
	return out, f.err
}

func (f *fakeStore) Totals(_ context.Context, userID int64, from, to time.Time) ([]Total, error) {
	f.monthCalls = append(f.monthCalls, monthCall{userID, from, to})
	return f.totals, f.err
}

func (f *fakeStore) DeleteEntry(ctx context.Context, userID, id int64) error {
	if _, err := f.Entry(ctx, userID, id); err != nil {
		return err
	}
	delete(f.entries, id)
	return nil
}
