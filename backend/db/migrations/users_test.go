package migrations_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"nest-egg/backend/db/migrations"
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

func TestUsersStartUnlocked(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	var lockedUntil *time.Time
	var lockCount, failed int
	err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('me@example.com', 'x') RETURNING locked_until, lock_count, failed_login_count`).
		Scan(&lockedUntil, &lockCount, &failed)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if lockedUntil != nil || lockCount != 0 || failed != 0 {
		t.Errorf("defaults: locked_until = %v, lock_count = %d, failed_login_count = %d; want nil, 0, 0", lockedUntil, lockCount, failed)
	}

	_, err = pool.Exec(ctx, `UPDATE users SET lock_count = -1`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("lock_count = -1: error = %v, want check violation 23514", err)
	}
}

const (
	permanentLockVersion = 10
	temporaryLockVersion = 11
)

func newProvider(t *testing.T, pool *pgxpool.Pool) *goose.Provider {
	t.Helper()
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return provider
}

func hasColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(t.Context(), `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2)`, table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	return exists
}

func TestUpgradeUnlocksPermanentlyLockedUsers(t *testing.T) {
	pool := dbtest.EmptySchema(t)
	ctx := t.Context()
	provider := newProvider(t, pool)
	if _, err := provider.UpTo(ctx, permanentLockVersion); err != nil {
		t.Fatalf("up to %d: %v", permanentLockVersion, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (email, password_hash, locked) VALUES ('me@example.com', 'x', 1)`); err != nil {
		t.Fatalf("insert locked user: %v", err)
	}

	if _, err := provider.UpTo(ctx, temporaryLockVersion); err != nil {
		t.Fatalf("up to %d: %v", temporaryLockVersion, err)
	}
	if hasColumn(t, pool, "users", "locked") {
		t.Error("users.locked still exists after the upgrade")
	}
	var lockedUntil *time.Time
	var lockCount int
	if err := pool.QueryRow(ctx, `SELECT locked_until, lock_count FROM users`).Scan(&lockedUntil, &lockCount); err != nil {
		t.Fatalf("select: %v", err)
	}
	if lockedUntil != nil || lockCount != 0 {
		t.Errorf("after upgrade: locked_until = %v, lock_count = %d; want nil, 0", lockedUntil, lockCount)
	}
}

func TestRollbackDoesNotLockAnyonePermanently(t *testing.T) {
	pool := dbtest.EmptySchema(t)
	ctx := t.Context()
	provider := newProvider(t, pool)
	if _, err := provider.UpTo(ctx, temporaryLockVersion); err != nil {
		t.Fatalf("up to %d: %v", temporaryLockVersion, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO users (email, password_hash, locked_until, lock_count, failed_login_count) VALUES
		('locked@example.com', 'x', now() + interval '10 minutes', 1, 0),
		('failing@example.com', 'x', NULL, 0, 2)`)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	if _, err := provider.DownTo(ctx, permanentLockVersion); err != nil {
		t.Fatalf("down to %d: %v", permanentLockVersion, err)
	}
	rows, err := pool.Query(ctx, `SELECT email, locked, failed_login_count FROM users ORDER BY email`)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	type user struct {
		email          string
		locked, failed int
	}
	var got []user
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.email, &u.locked, &u.failed); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, u)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	want := []user{{"failing@example.com", 0, 2}, {"locked@example.com", 0, 0}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("after rollback = %+v, want %+v (nobody locked, failures kept)", got, want)
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
