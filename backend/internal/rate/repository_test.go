package rate

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbgen"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/taipei"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, taipei.Location)
}

func usdTWD(date time.Time, value string) Rate {
	return Rate{Base: "USD", Quote: "TWD", Date: date, Value: decimal.RequireFromString(value)}
}

func TestRepositoryRoundTripKeepsValueAndDate(t *testing.T) {
	repo := NewRepository(dbtest.Migrated(t))
	ctx := t.Context()

	if _, err := repo.Upsert(ctx, "cbc", []Rate{usdTWD(day(2026, 10, 16), "32.443")}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := repo.On(ctx, "USD", "TWD", day(2026, 10, 16))
	if err != nil {
		t.Fatalf("On: %v", err)
	}
	if got.Value.String() != "32.443" {
		t.Errorf("value = %s, want 32.443", got.Value)
	}
	if !got.Date.Equal(day(2026, 10, 16)) || got.Date.Location() != taipei.Location {
		t.Errorf("date = %v, want 2026-10-16 in Asia/Taipei", got.Date)
	}
	if got.Base != "USD" || got.Quote != "TWD" || got.Source != "cbc" {
		t.Errorf("snapshot = %+v, want USD/TWD from cbc", got)
	}
}

func fetchedAt(t *testing.T, db dbgen.DBTX, date time.Time) time.Time {
	t.Helper()
	var at time.Time
	err := db.QueryRow(t.Context(), `SELECT fetched_at FROM rate_snapshots WHERE base = 'USD' AND quote = 'TWD' AND rate_date = $1`, date).Scan(&at)
	if err != nil {
		t.Fatalf("select fetched_at: %v", err)
	}
	return at
}

func TestRepositoryUpsert(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	first := []Rate{usdTWD(day(2026, 10, 15), "32.265"), usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "32.445")}

	got, err := repo.Upsert(ctx, "cbc", first)
	if err != nil || got != (UpsertResult{Inserted: 3}) {
		t.Fatalf("first Upsert = %+v, %v; want 3 inserted", got, err)
	}
	unchangedAt := fetchedAt(t, pool, day(2026, 10, 15))

	got, err = repo.Upsert(ctx, "cbc", first)
	if err != nil || got != (UpsertResult{}) {
		t.Errorf("same data again = %+v, %v; want nothing inserted or updated", got, err)
	}
	if at := fetchedAt(t, pool, day(2026, 10, 15)); !at.Equal(unchangedAt) {
		t.Errorf("fetched_at changed from %v to %v although the rate did not", unchangedAt, at)
	}

	revised := []Rate{usdTWD(day(2026, 10, 15), "32.265"), usdTWD(day(2026, 10, 16), "32.340"), usdTWD(day(2026, 10, 19), "32.445"), usdTWD(day(2026, 10, 20), "32.5")}
	got, err = repo.Upsert(ctx, "cbc", revised)
	if err != nil || got != (UpsertResult{Inserted: 1, Updated: 1}) {
		t.Errorf("revised Upsert = %+v, %v; want 1 inserted, 1 updated", got, err)
	}
	snap, err := repo.On(ctx, "USD", "TWD", day(2026, 10, 16))
	if err != nil || snap.Value.String() != "32.34" {
		t.Errorf("2026-10-16 after revision = %v, %v; want 32.34", snap.Value, err)
	}
}

func TestRepositoryOnLooksBackAtMostTenDays(t *testing.T) {
	repo := NewRepository(dbtest.Migrated(t))
	ctx := t.Context()
	if _, err := repo.Upsert(ctx, "cbc", []Rate{usdTWD(day(2026, 10, 1), "31.5"), usdTWD(day(2026, 10, 16), "32.338"), usdTWD(day(2026, 10, 19), "32.445")}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	tests := []struct {
		name string
		on   time.Time
		want time.Time
	}{
		{"business day", day(2026, 10, 16), day(2026, 10, 16)},
		{"saturday uses friday", day(2026, 10, 17), day(2026, 10, 16)},
		{"sunday never uses the following monday", day(2026, 10, 18), day(2026, 10, 16)},
		{"exactly ten days back", day(2026, 10, 11), day(2026, 10, 1)},
	}
	for _, tt := range tests {
		got, err := repo.On(ctx, "USD", "TWD", tt.on)
		if err != nil || !got.Date.Equal(tt.want) {
			t.Errorf("%s: On(%s) = %v, %v; want rate dated %s", tt.name, tt.on.Format(time.DateOnly), got.Date, err, tt.want.Format(time.DateOnly))
		}
	}

	for _, on := range []time.Time{day(2026, 10, 12), day(2026, 9, 30)} {
		if _, err := repo.On(ctx, "USD", "TWD", on); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("On(%s): err = %v, want ErrNotFound", on.Format(time.DateOnly), err)
		}
	}
}
