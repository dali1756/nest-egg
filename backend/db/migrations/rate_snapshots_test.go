package migrations_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"nest-egg/backend/internal/platform/db/dbtest"
)

const insertRate = `INSERT INTO rate_snapshots (base, quote, rate_date, rate, source) VALUES ($1, $2, '2026-10-16', $3, 'cbc')`

func assertPgError(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: error = %v, want SQLSTATE %s", what, err, code)
	}
}

func TestRateSnapshotsConstraints(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()

	_, err := pool.Exec(ctx, insertRate, "USD", "TWD", "0")
	assertPgError(t, err, "23514", "rate = 0")

	_, err = pool.Exec(ctx, insertRate, "USD", "USD", "1")
	assertPgError(t, err, "23514", "base = quote")

	_, err = pool.Exec(ctx, insertRate, "XYZ", "TWD", "1")
	assertPgError(t, err, "23503", "unknown base currency")

	if _, err := pool.Exec(ctx, insertRate, "USD", "TWD", "32.338"); err != nil {
		t.Fatalf("valid insert: %v", err)
	}
	_, err = pool.Exec(ctx, insertRate, "USD", "TWD", "32.4")
	assertPgError(t, err, "23505", "same base, quote and rate_date")
}

func TestRateSnapshotsFetchedAtDefaultsToNow(t *testing.T) {
	pool := dbtest.Migrated(t)
	var recent bool
	err := pool.QueryRow(t.Context(), insertRate+` RETURNING fetched_at > now() - interval '1 minute'`, "USD", "TWD", "32.338").Scan(&recent)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !recent {
		t.Error("fetched_at is not set to now()")
	}
}
