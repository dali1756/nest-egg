package migrations_test

import (
	"strings"
	"testing"

	"nest-egg/backend/internal/platform/db/dbtest"
)

const insertTrade = `INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount) VALUES ($1, '2026-10-27', $2, 'USD', $3, $4)`

func TestFxTradeAcceptsBuyAndSell(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	for _, side := range []string{"buy", "sell"} {
		if _, err := pool.Exec(t.Context(), insertTrade, a, side, "1000", "32420"); err != nil {
			t.Errorf("%s: %v, want it to be allowed", side, err)
		}
	}
}

func TestFxTradeSideMustBeBuyOrSell(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	_, err := pool.Exec(t.Context(), insertTrade, a, "swap", "1000", "32420")
	wantPgError(t, err, "23514", "side swap")
}

func TestFxTradeAmountsMustBePositive(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	for name, amounts := range map[string][2]string{
		"zero foreign amount":     {"0", "32420"},
		"negative foreign amount": {"-1", "32420"},
		"zero TWD amount":         {"1000", "0"},
		"negative TWD amount":     {"1000", "-1"},
	} {
		_, err := pool.Exec(t.Context(), insertTrade, a, "buy", amounts[0], amounts[1])
		wantPgError(t, err, "23514", name)
	}
}

func TestFxTradeCurrencyMustExist(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")

	_, err := pool.Exec(t.Context(), `INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount) VALUES ($1, '2026-10-27', 'buy', 'XYZ', 1000, 32420)`, a)
	wantPgError(t, err, "23503", "currency XYZ")
}

func TestFxTradeMarketRateAndDateComeTogether(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := dbtest.InsertUser(t, pool, "a@example.com")
	const insert = `INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount, market_rate, market_rate_date)
		VALUES ($1, '2026-10-27', 'buy', 'USD', 1000, 32420, $2, $3)`

	for name, tc := range map[string]struct {
		rate, date any
		ok         bool
	}{
		"both":           {"32.3", "2026-10-27", true},
		"neither":        {nil, nil, true},
		"rate only":      {"32.3", nil, false},
		"rate date only": {nil, "2026-10-27", false},
	} {
		_, err := pool.Exec(t.Context(), insert, a, tc.rate, tc.date)
		if tc.ok {
			if err != nil {
				t.Errorf("%s: %v, want it to be allowed", name, err)
			}
			continue
		}
		wantPgError(t, err, "23514", name)
	}
}

func TestFxTradeNoteDefaultsToEmptyAndIsAtMost200Characters(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")

	var note string
	if err := pool.QueryRow(ctx, insertTrade+` RETURNING note`, a, "buy", "1000", "32420").Scan(&note); err != nil {
		t.Fatalf("insert without note: %v", err)
	}
	if note != "" {
		t.Errorf("default note = %q, want empty", note)
	}

	const withNote = `INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount, note) VALUES ($1, '2026-10-27', 'buy', 'USD', 1000, 32420, $2)`
	if _, err := pool.Exec(ctx, withNote, a, strings.Repeat("換", 200)); err != nil {
		t.Errorf("200-character note: %v, want it to be allowed", err)
	}
	_, err := pool.Exec(ctx, withNote, a, strings.Repeat("換", 201))
	wantPgError(t, err, "23514", "201-character note")
}

func TestFxTradesAreDeletedWithUser(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	if _, err := pool.Exec(ctx, insertTrade, a, "buy", "1000", "32420"); err != nil {
		t.Fatalf("insert trade: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var trades int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM fx_trades`).Scan(&trades); err != nil {
		t.Fatal(err)
	}
	if trades != 0 {
		t.Errorf("after deleting the user: fx_trades = %d, want 0", trades)
	}
}
