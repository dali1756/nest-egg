package rate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
)

type lookup struct{ base, quote, date string }

type fakeFinder struct {
	snapshot Snapshot
	err      error
	lookups  []lookup
}

func (f *fakeFinder) On(_ context.Context, base, quote string, date time.Time) (Snapshot, error) {
	f.lookups = append(f.lookups, lookup{base, quote, date.Format(time.DateOnly)})
	return f.snapshot, f.err
}

func TestMarketRateInTWDLooksUpTheForeignCurrencyAgainstTWD(t *testing.T) {
	finder := &fakeFinder{snapshot: Snapshot{Rate: Rate{Base: "USD", Quote: "TWD", Date: day(2026, 10, 16), Value: decimal.RequireFromString("32.338")}}}

	got, err := MarketRateInTWD(context.Background(), finder, "USD", day(2026, 10, 17))

	if err != nil || got == nil || !got.Value.Equal(decimal.RequireFromString("32.338")) || !got.Date.Equal(day(2026, 10, 16)) {
		t.Fatalf("MarketRateInTWD = %+v, %v; want 32.338 on 2026-10-16", got, err)
	}
	if want := []lookup{{"USD", "TWD", "2026-10-17"}}; len(finder.lookups) != 1 || finder.lookups[0] != want[0] {
		t.Errorf("lookups = %v, want %v", finder.lookups, want)
	}
}

func TestMarketRateInTWDIsNoneForTWDItself(t *testing.T) {
	finder := &fakeFinder{}

	got, err := MarketRateInTWD(context.Background(), finder, "TWD", day(2026, 10, 17))

	if got != nil || err != nil || len(finder.lookups) != 0 {
		t.Errorf("MarketRateInTWD = %+v, %v after %d lookups; want nil without looking up", got, err, len(finder.lookups))
	}
}

func TestMarketRateInTWDIsNoneWhenNoRateWithinTheLookBack(t *testing.T) {
	finder := &fakeFinder{err: apperr.ErrNotFound}

	got, err := MarketRateInTWD(context.Background(), finder, "USD", day(2008, 1, 1))

	if got != nil || err != nil {
		t.Errorf("MarketRateInTWD = %+v, %v; want nil, nil", got, err)
	}
}

func TestMarketRateColumnsOfNoRateAreNull(t *testing.T) {
	var none *MarketRate
	if value, date := none.Columns(); value != nil || date != nil {
		t.Errorf("Columns = %v, %v; want nil, nil", value, date)
	}
}

func TestMarketRateColumnsRoundTrip(t *testing.T) {
	stored := &MarketRate{Value: decimal.RequireFromString("32.338"), Date: day(2026, 10, 16)}
	value, date := stored.Columns()
	if value == nil || date == nil || !value.Equal(stored.Value) || !date.Equal(stored.Date) {
		t.Fatalf("Columns = %v, %v; want 32.338 on 2026-10-16", value, date)
	}

	// 資料庫讀出的 DATE 是 UTC 午夜，要換回台北日期
	dbDate := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	got := MarketRateFromColumns(value, &dbDate)
	if got == nil || !got.Value.Equal(stored.Value) || !got.Date.Equal(stored.Date) {
		t.Errorf("MarketRateFromColumns = %+v, want 32.338 on 2026-10-16 in Asia/Taipei", got)
	}
}

func TestMarketRateFromNullColumnsIsNone(t *testing.T) {
	if got := MarketRateFromColumns(nil, nil); got != nil {
		t.Errorf("MarketRateFromColumns(nil, nil) = %+v, want nil", got)
	}
}

func TestMarketRateInTWDPassesOtherErrorsOn(t *testing.T) {
	broken := errors.New("connection refused")
	finder := &fakeFinder{err: broken}

	got, err := MarketRateInTWD(context.Background(), finder, "USD", day(2026, 10, 17))

	if got != nil || !errors.Is(err, broken) {
		t.Errorf("MarketRateInTWD = %+v, %v; want the lookup error", got, err)
	}
}
