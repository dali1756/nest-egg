package migrations_test

import (
	"testing"

	"nest-egg/backend/internal/platform/db/dbtest"
)

const insertOpening = `INSERT INTO opening_balances (user_id, currency, opened_on, amount, twd_cost) VALUES ($1, $2, '2026-10-01', $3, $4)`

func TestOpeningBalanceAmountMustBePositive(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	for name, amount := range map[string]string{"zero amount": "0", "negative amount": "-1"} {
		_, err := pool.Exec(t.Context(), insertOpening, a, "USD", amount, nil)
		wantPgError(t, err, "23514", name)
	}
}

func TestOpeningBalanceTwdCostIsOptionalButPositive(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	if _, err := pool.Exec(t.Context(), insertOpening, a, "USD", "10000", nil); err != nil {
		t.Errorf("NULL twd_cost: %v, want it to be allowed", err)
	}
	for name, cost := range map[string]string{"zero twd_cost": "0", "negative twd_cost": "-1"} {
		_, err := pool.Exec(t.Context(), insertOpening, a, "TWD", "10000", cost)
		wantPgError(t, err, "23514", name)
	}
}

func TestOpeningBalanceMarketRateAndDateComeTogether(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")
	const insert = `INSERT INTO opening_balances (user_id, currency, opened_on, amount, market_rate, market_rate_date)
		VALUES ($1, 'USD', '2026-10-01', 10000, $2, $3)`

	for name, tc := range map[string]struct{ rate, date any }{
		"rate only":      {"32", nil},
		"rate date only": {nil, "2026-10-01"},
	} {
		_, err := pool.Exec(t.Context(), insert, a, tc.rate, tc.date)
		wantPgError(t, err, "23514", name)
	}
	if _, err := pool.Exec(t.Context(), insert, a, "32", "2026-10-01"); err != nil {
		t.Errorf("rate with date: %v, want it to be allowed", err)
	}
}

func TestOpeningBalanceIsOnePerUserAndCurrency(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	b := dbtest.InsertUser(t, pool, "b@example.com")
	if _, err := pool.Exec(ctx, insertOpening, a, "USD", "10000", nil); err != nil {
		t.Fatalf("first USD: %v", err)
	}

	_, err := pool.Exec(ctx, insertOpening, a, "USD", "5000", nil)
	wantPgError(t, err, "23505", "second USD for the same user")

	if _, err := pool.Exec(ctx, insertOpening, b, "USD", "5000", nil); err != nil {
		t.Errorf("USD for another user: %v, want it to be allowed", err)
	}
}

func TestOpeningBalancesAreDeletedWithUser(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	if _, err := pool.Exec(ctx, insertOpening, a, "USD", "10000", nil); err != nil {
		t.Fatalf("insert opening balance: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var openings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM opening_balances`).Scan(&openings); err != nil {
		t.Fatal(err)
	}
	if openings != 0 {
		t.Errorf("after deleting the user: opening_balances = %d, want 0", openings)
	}
}
