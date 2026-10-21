package rate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
)

type fakeProvider struct {
	rates []Rate
	err   error
	panic bool
	calls int
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) Fetch(context.Context) ([]Rate, error) {
	p.calls++
	if p.panic {
		panic("boom")
	}
	return p.rates, p.err
}

type fakeStore struct {
	upserted [][]Rate
	source   string
	result   UpsertResult
	err      error
	snapshot Snapshot
	onArgs   []string
}

func (s *fakeStore) Upsert(_ context.Context, source string, rates []Rate) (UpsertResult, error) {
	s.upserted = append(s.upserted, rates)
	s.source = source
	return s.result, s.err
}

func (s *fakeStore) On(_ context.Context, base, quote string, date time.Time) (Snapshot, error) {
	if date.Location() != taipei.Location || !date.Equal(taipei.Today(date)) {
		return Snapshot{}, fmt.Errorf("date %v is not a Taipei midnight", date)
	}
	s.onArgs = append(s.onArgs, base+"/"+quote+" "+date.Format(time.DateOnly))
	return s.snapshot, s.err
}

type fakeCurrencies struct {
	err error
}

func (f fakeCurrencies) Exists(_ context.Context, code string) (bool, error) {
	return code == "USD" || code == "TWD", f.err
}

func newTestService(store *fakeStore, provider *fakeProvider, now time.Time) *Service {
	svc := NewService(store, provider, fakeCurrencies{})
	svc.now = func() time.Time { return now }
	return svc
}

func TestValidateAcceptsAGoodBatch(t *testing.T) {
	batch := []Rate{usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "32.445")}
	if err := validate(batch, day(2026, 10, 19)); err != nil {
		t.Errorf("validate: %v", err)
	}
}

func TestValidateRejectsBadBatches(t *testing.T) {
	today := day(2026, 10, 19)
	tests := map[string][]Rate{
		"empty":          nil,
		"duplicate date": {usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 16), "32.34")},
		"future date":    {usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 20), "32.5")},
		"zero":           {usdTWD(day(2026, 10, 16), "0")},
		"negative":       {usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "-1")},
	}
	for name, batch := range tests {
		if err := validate(batch, today); err == nil {
			t.Errorf("%s: validate = nil, want error", name)
		}
	}
}

func TestValidateAllowsTheSameDateForDifferentPairs(t *testing.T) {
	jpy := usdTWD(day(2026, 10, 16), "0.21")
	jpy.Base = "JPY"
	if err := validate([]Rate{usdTWD(day(2026, 10, 16), "32.338"), jpy}, day(2026, 10, 19)); err != nil {
		t.Errorf("validate: %v", err)
	}
}

var evening = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

func TestUpdateStoresTheFetchedRates(t *testing.T) {
	rates := []Rate{usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "32.445")}
	store := &fakeStore{result: UpsertResult{Inserted: 1, Updated: 1}}
	svc := newTestService(store, &fakeProvider{rates: rates}, evening)

	got, err := svc.Update(t.Context())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := UpdateResult{Source: "fake", Fetched: 2, Inserted: 1, Updated: 1, Latest: day(2026, 10, 19)}
	if got != want {
		t.Errorf("Update = %+v, want %+v", got, want)
	}
	if len(store.upserted) != 1 || !slices.Equal(store.upserted[0], rates) || store.source != "fake" {
		t.Errorf("store got %v from %q, want the fetched rates from fake", store.upserted, store.source)
	}
}

func TestUpdateWritesNothingWhenTheBatchIsBad(t *testing.T) {
	tests := map[string]*fakeProvider{
		"fetch failed":   {err: errors.New("connection refused")},
		"empty":          {rates: nil},
		"invalid batch":  {rates: []Rate{usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "-1")}},
		"dated tomorrow": {rates: []Rate{usdTWD(day(2026, 10, 20), "32.5")}},
	}
	for name, provider := range tests {
		store := &fakeStore{}
		if _, err := newTestService(store, provider, evening).Update(t.Context()); err == nil {
			t.Errorf("%s: Update = nil error, want error", name)
		}
		if len(store.upserted) != 0 {
			t.Errorf("%s: store was called %d times, want 0", name, len(store.upserted))
		}
	}
}

func TestUpdateReturnsStoreErrors(t *testing.T) {
	store := &fakeStore{err: errors.New("deadlock")}
	svc := newTestService(store, &fakeProvider{rates: []Rate{usdTWD(day(2026, 10, 16), "32.338")}}, evening)
	if _, err := svc.Update(t.Context()); err == nil {
		t.Error("Update = nil error, want the store error")
	}
}

func TestOnLooksUpTheGivenDate(t *testing.T) {
	want := Snapshot{Rate: usdTWD(day(2026, 10, 16), "32.338"), Source: "cbc"}
	store := &fakeStore{snapshot: want}
	got, err := newTestService(store, nil, evening).On(t.Context(), "USD", "TWD", "2026-10-17")
	if err != nil || got != want {
		t.Errorf("On = %+v, %v; want %+v", got, err, want)
	}
	if !slices.Equal(store.onArgs, []string{"USD/TWD 2026-10-17"}) {
		t.Errorf("store.On calls = %v, want [USD/TWD 2026-10-17]", store.onArgs)
	}
}

func TestOnDefaultsToTodayInTaipei(t *testing.T) {
	store := &fakeStore{}
	if _, err := newTestService(store, nil, time.Date(2026, 10, 19, 16, 30, 0, 0, time.UTC)).On(t.Context(), "USD", "TWD", ""); err != nil {
		t.Fatalf("On: %v", err)
	}
	if !slices.Equal(store.onArgs, []string{"USD/TWD 2026-10-20"}) {
		t.Errorf("store.On calls = %v, want [USD/TWD 2026-10-20]", store.onArgs)
	}
}

func TestOnRejectsBadInput(t *testing.T) {
	tests := []struct {
		base, quote, date string
		fields            []string
	}{
		{"XYZ", "TWD", "", []string{"base"}},
		{"USD", "", "", []string{"quote"}},
		{"", "", "", []string{"base", "quote"}},
		{"USD", "TWD", "2026/10/17", []string{"date"}},
		{"USD", "TWD", "2026-10-20", []string{"date"}},
	}
	for _, tt := range tests {
		store := &fakeStore{}
		_, err := newTestService(store, nil, evening).On(t.Context(), tt.base, tt.quote, tt.date)
		var invalid *apperr.ValidationError
		if !errors.As(err, &invalid) {
			t.Errorf("On(%q, %q, %q) = %v, want ValidationError", tt.base, tt.quote, tt.date, err)
			continue
		}
		if got := slices.Sorted(maps.Keys(invalid.Fields)); !slices.Equal(got, tt.fields) {
			t.Errorf("On(%q, %q, %q) fields = %v, want %v", tt.base, tt.quote, tt.date, invalid.Fields, tt.fields)
		}
		if len(store.onArgs) != 0 {
			t.Errorf("On(%q, %q, %q) queried the store with invalid input", tt.base, tt.quote, tt.date)
		}
	}
}

func TestOnSaysWhichProblemTheFieldHas(t *testing.T) {
	_, err := newTestService(&fakeStore{}, nil, evening).On(t.Context(), "", "XYZ", "2026-10-20")
	var invalid *apperr.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("On = %v, want ValidationError", err)
	}
	want := map[string]string{"base": "required", "quote": "unknown currency", "date": "must not be in the future"}
	if !maps.Equal(invalid.Fields, want) {
		t.Errorf("fields = %v, want %v", invalid.Fields, want)
	}
}

func TestOnPassesThroughErrors(t *testing.T) {
	_, err := newTestService(&fakeStore{err: fmt.Errorf("rate: %w", apperr.ErrNotFound)}, nil, evening).On(t.Context(), "USD", "TWD", "")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("not found: err = %v, want ErrNotFound", err)
	}

	svc := newTestService(&fakeStore{}, nil, evening)
	svc.currencies = fakeCurrencies{err: errors.New("connection refused")}
	_, err = svc.On(t.Context(), "USD", "TWD", "")
	var invalid *apperr.ValidationError
	if err == nil || errors.As(err, &invalid) {
		t.Errorf("currency lookup failed: err = %v, want a non-validation error", err)
	}
}
