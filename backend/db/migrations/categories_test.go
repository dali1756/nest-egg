package migrations_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nest-egg/backend/internal/platform/db/dbtest"
)

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	return id
}

func wantPgError(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: error = %v, want SQLSTATE %s", what, err, code)
	}
}

const insertCategory = `INSERT INTO categories (user_id, kind, name, position) VALUES ($1, $2, $3, 1)`

func TestCategoryNameIsUniquePerUserAndKind(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a, b := insertUser(t, pool, "a@example.com"), insertUser(t, pool, "b@example.com")

	if _, err := pool.Exec(ctx, insertCategory, a, "expense", "飲食"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := pool.Exec(ctx, insertCategory, a, "expense", "飲食")
	wantPgError(t, err, "23505", "same user, kind and name")

	for _, tc := range []struct {
		user int64
		kind string
	}{{a, "income"}, {b, "expense"}} {
		if _, err := pool.Exec(ctx, insertCategory, tc.user, tc.kind, "飲食"); err != nil {
			t.Errorf("user %d kind %s: %v, want the same name to be allowed", tc.user, tc.kind, err)
		}
	}
}

func TestCategoryKindIsIncomeOrExpense(t *testing.T) {
	pool := dbtest.Migrated(t)
	a := insertUser(t, pool, "a@example.com")

	_, err := pool.Exec(t.Context(), insertCategory, a, "transfer", "轉帳")
	wantPgError(t, err, "23514", "kind transfer")
}

func TestCategoryNameIsOneToTwentyCharacters(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")

	if _, err := pool.Exec(ctx, insertCategory, a, "expense", strings.Repeat("飲", 20)); err != nil {
		t.Errorf("20-character name: %v, want it to be allowed", err)
	}
	for _, name := range []string{"", strings.Repeat("飲", 21)} {
		_, err := pool.Exec(ctx, insertCategory, a, "income", name)
		wantPgError(t, err, "23514", fmt.Sprintf("name with %d characters", utf8.RuneCountInString(name)))
	}
}
