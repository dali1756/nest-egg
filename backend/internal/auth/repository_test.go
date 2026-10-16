package auth_test

import (
	"testing"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
	"nest-egg/backend/internal/platform/db/dbtest"
)

func TestRepositoryContract(t *testing.T) {
	authtest.RunStoreContract(t, func(t *testing.T) auth.Store {
		return auth.NewRepository(dbtest.Migrated(t))
	})
}

func TestRepositoryUnlockBySettingLockedToZero(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := auth.NewRepository(pool)
	ctx := t.Context()
	id, err := repo.CreateUser(ctx, "me@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	for range 3 {
		if _, err := repo.ReserveLoginAttempt(ctx, id, 3); err != nil {
			t.Fatalf("ReserveLoginAttempt: %v", err)
		}
	}
	if locked, err := repo.LockIfFailuresExhausted(ctx, id, 3); err != nil || !locked {
		t.Fatalf("LockIfFailuresExhausted = %v, %v; want true, nil", locked, err)
	}

	var locked, failed int
	if err := pool.QueryRow(ctx, `SELECT locked, failed_login_count FROM users WHERE id = $1`, id).Scan(&locked, &failed); err != nil {
		t.Fatal(err)
	}
	if locked != 1 || failed != 0 {
		t.Fatalf("after lock: locked = %d, failed_login_count = %d; want 1, 0", locked, failed)
	}

	if _, err := pool.Exec(ctx, `UPDATE users SET locked = 0 WHERE email = 'me@example.com'`); err != nil {
		t.Fatalf("manual unlock: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if ok, err := repo.ReserveLoginAttempt(ctx, id, 3); err != nil || !ok {
			t.Fatalf("attempt %d after manual unlock = %v, %v; want true, nil", i, ok, err)
		}
	}
}
