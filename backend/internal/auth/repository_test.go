package auth_test

import (
	"testing"
	"time"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
	"nest-egg/backend/internal/platform/db/dbtest"
)

func TestRepositoryContract(t *testing.T) {
	authtest.RunStoreContract(t, func(t *testing.T) auth.Store {
		return auth.NewRepository(dbtest.Migrated(t))
	})
}

func TestRepositoryManualUnlock(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := auth.NewRepository(pool)
	ctx := t.Context()
	id, err := repo.CreateUser(ctx, "me@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	limit := auth.AttemptLimit{MaxFailures: 3, Now: now, FirstLockUntil: now.Add(10 * time.Minute), RepeatLockUntil: now.Add(30 * time.Minute)}
	reserve := func() auth.LoginAttempt {
		t.Helper()
		got, err := repo.ReserveLoginAttempt(ctx, id, limit)
		if err != nil {
			t.Fatalf("ReserveLoginAttempt: %v", err)
		}
		return got
	}
	for range 3 {
		reserve()
	}
	if got := reserve(); got.Reserved {
		t.Fatalf("4th attempt = %+v, want rejected while locked", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE users SET locked_until = NULL, failed_login_count = 0, lock_count = 0 WHERE email = 'me@example.com'`); err != nil {
		t.Fatalf("manual unlock: %v", err)
	}
	if got := reserve(); !got.Reserved || !got.LockedUntil.IsZero() {
		t.Errorf("attempt after manual unlock = %+v, want reserved without a lock", got)
	}
}
