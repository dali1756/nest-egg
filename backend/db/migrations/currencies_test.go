package migrations_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"nest-egg/backend/internal/platform/db/dbtest"
)

func TestCurrenciesHaveInitialData(t *testing.T) {
	pool := dbtest.Migrated(t)
	rows, err := pool.Query(t.Context(), `SELECT code, name, minor_unit FROM currencies ORDER BY code`)
	if err != nil {
		t.Fatalf("select currencies: %v", err)
	}
	defer rows.Close()

	type currency struct {
		code, name string
		minorUnit  int
	}
	var got []currency
	for rows.Next() {
		var c currency
		if err := rows.Scan(&c.code, &c.name, &c.minorUnit); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []currency{{"TWD", "新臺幣", 2}, {"USD", "美元", 2}}
	if len(got) != len(want) {
		t.Fatalf("currencies = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("currencies[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCurrencyCodeMustBeThreeUppercaseLetters(t *testing.T) {
	pool := dbtest.Migrated(t)
	for _, code := range []string{"usd", "US1"} {
		_, err := pool.Exec(t.Context(), `INSERT INTO currencies (code, name, minor_unit) VALUES ($1, 'x', 2)`, code)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("insert code %q: error = %v, want check violation 23514", code, err)
		}
	}
}
