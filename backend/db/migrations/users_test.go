package migrations_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"nest-egg/backend/internal/platform/db/dbtest"
)

func TestUsersEmailIsUnique(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	const insert = `INSERT INTO users (email, password_hash) VALUES ($1, 'x')`

	if _, err := pool.Exec(ctx, insert, "me@example.com"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := pool.Exec(ctx, insert, "me@example.com")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second insert error = %v, want unique violation 23505", err)
	}
}

func TestUsersEmailMustBeLowercase(t *testing.T) {
	pool := dbtest.Migrated(t)

	_, err := pool.Exec(t.Context(), `INSERT INTO users (email, password_hash) VALUES ('Me@Example.com', 'x')`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("insert error = %v, want check violation 23514", err)
	}
}
