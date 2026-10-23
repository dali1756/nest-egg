package migrations_test

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nest-egg/backend/internal/platform/db/dbtest"
)

func insertExpenseCategory(t *testing.T, pool *pgxpool.Pool, userID int64) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO categories (user_id, kind, name, position) VALUES ($1, 'expense', '飲食', 1) RETURNING id`, userID).Scan(&id)
	if err != nil {
		t.Fatalf("insert category for user %d: %v", userID, err)
	}
	return id
}

const insertEntry = `INSERT INTO ledger_entries (user_id, occurred_on, category_id, currency, amount) VALUES ($1, '2026-10-23', $2, 'TWD', $3)`

func TestLedgerEntryCannotUseAnotherUsersCategory(t *testing.T) {
	pool := dbtest.Migrated(t)
	a, b := insertUser(t, pool, "a@example.com"), insertUser(t, pool, "b@example.com")
	aCategory := insertExpenseCategory(t, pool, a)

	_, err := pool.Exec(t.Context(), insertEntry, b, aCategory, "120")
	wantPgError(t, err, "23503", "B's entry with A's category")
}

func TestLedgerEntryAmountMustBePositive(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := insertUser(t, pool, "a@example.com")
	category := insertExpenseCategory(t, pool, a)

	for _, amount := range []string{"0", "-1"} {
		_, err := pool.Exec(t.Context(), insertEntry, a, category, amount)
		wantPgError(t, err, "23514", "amount "+amount)
	}
}

func TestLedgerEntryMarketRateAndDateComeTogether(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")
	category := insertExpenseCategory(t, pool, a)
	const insert = `INSERT INTO ledger_entries (user_id, occurred_on, category_id, currency, amount, market_rate, market_rate_date)
		VALUES ($1, '2026-10-23', $2, 'USD', 10, $3, $4)`

	for name, tc := range map[string]struct {
		rate, date any
		ok         bool
	}{
		"both":           {"32.508", "2026-10-22", true},
		"neither":        {nil, nil, true},
		"rate only":      {"32.508", nil, false},
		"rate date only": {nil, "2026-10-22", false},
	} {
		_, err := pool.Exec(ctx, insert, a, category, tc.rate, tc.date)
		if tc.ok {
			if err != nil {
				t.Errorf("%s: %v, want it to be allowed", name, err)
			}
			continue
		}
		wantPgError(t, err, "23514", name)
	}
}

func TestLedgerEntryNoteDefaultsToEmptyAndIsAtMost200Characters(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")
	category := insertExpenseCategory(t, pool, a)

	var note string
	if err := pool.QueryRow(ctx, insertEntry+` RETURNING note`, a, category, "120").Scan(&note); err != nil {
		t.Fatalf("insert without note: %v", err)
	}
	if note != "" {
		t.Errorf("default note = %q, want empty", note)
	}

	const withNote = `INSERT INTO ledger_entries (user_id, occurred_on, category_id, currency, amount, note) VALUES ($1, '2026-10-23', $2, 'TWD', 120, $3)`
	if _, err := pool.Exec(ctx, withNote, a, category, strings.Repeat("午", 200)); err != nil {
		t.Errorf("200-character note: %v, want it to be allowed", err)
	}
	_, err := pool.Exec(ctx, withNote, a, category, strings.Repeat("午", 201))
	wantPgError(t, err, "23514", "201-character note")
}

func TestLedgerDataIsDeletedWithUser(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")
	category := insertExpenseCategory(t, pool, a)
	if _, err := pool.Exec(ctx, insertEntry, a, category, "120"); err != nil {
		t.Fatalf("insert entry: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var categories, entries int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM categories), (SELECT count(*) FROM ledger_entries)`).Scan(&categories, &entries); err != nil {
		t.Fatal(err)
	}
	if categories != 0 || entries != 0 {
		t.Errorf("after deleting the user: categories = %d, ledger_entries = %d; want 0, 0", categories, entries)
	}
}
