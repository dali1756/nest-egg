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

func TestUsersLockedIsZeroOrOne(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	var locked, failed int
	err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('me@example.com', 'x') RETURNING locked, failed_login_count`).Scan(&locked, &failed)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if locked != 0 || failed != 0 {
		t.Errorf("defaults: locked = %d, failed_login_count = %d; want 0, 0", locked, failed)
	}

	_, err = pool.Exec(ctx, `UPDATE users SET locked = 2`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("locked = 2: error = %v, want check violation 23514", err)
	}
}

func TestSessionsAreDeletedWithUser(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('me@example.com', 'x') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, last_seen_at, expires_at) VALUES ('\x01', $1, now(), now())`, id); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("sessions after deleting the user = %d, want 0", n)
	}
}
