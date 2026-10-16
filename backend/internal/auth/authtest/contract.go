package authtest

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"nest-egg/backend/internal/auth"
)

var base = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func RunStoreContract(t *testing.T, newStore func(t *testing.T) auth.Store) {
	t.Run("users", func(t *testing.T) { testUsers(t, newStore(t)) })
	t.Run("login attempts", func(t *testing.T) { testLoginAttempts(t, newStore(t)) })
	t.Run("sessions", func(t *testing.T) { testSessions(t, newStore(t)) })
	t.Run("stale sessions", func(t *testing.T) { testStaleSessions(t, newStore(t)) })
	t.Run("password reset", func(t *testing.T) { testPasswordReset(t, newStore(t)) })
	t.Run("password reset keeps lock", func(t *testing.T) { testPasswordResetKeepsLock(t, newStore(t)) })
	t.Run("change password", func(t *testing.T) { testChangePassword(t, newStore(t)) })
	t.Run("update password hash", func(t *testing.T) { testUpdatePasswordHash(t, newStore(t)) })
}

func mustCreateUser(t *testing.T, s auth.Store, email string) int64 {
	t.Helper()
	id, err := s.CreateUser(t.Context(), email, "hash-of-"+email)
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	return id
}

func testUsers(t *testing.T, s auth.Store) {
	ctx := t.Context()
	if _, err := s.UserByEmail(ctx, "me@example.com"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("UserByEmail before create: err = %v, want ErrNotFound", err)
	}

	id := mustCreateUser(t, s, "me@example.com")
	got, err := s.UserByEmail(ctx, "me@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	want := auth.UserRecord{ID: id, Email: "me@example.com", PasswordHash: "hash-of-me@example.com"}
	if id <= 0 || got != want {
		t.Errorf("UserByEmail = %+v, want %+v with ID > 0", got, want)
	}

	if _, err := s.CreateUser(ctx, "me@example.com", "other"); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("CreateUser duplicate: err = %v, want ErrEmailTaken", err)
	}
}

func reserve(t *testing.T, s auth.Store, id int64) bool {
	t.Helper()
	ok, err := s.ReserveLoginAttempt(t.Context(), id, 3)
	if err != nil {
		t.Fatalf("ReserveLoginAttempt: %v", err)
	}
	return ok
}

func lockIfExhausted(t *testing.T, s auth.Store, id int64) bool {
	t.Helper()
	locked, err := s.LockIfFailuresExhausted(t.Context(), id, 3)
	if err != nil {
		t.Fatalf("LockIfFailuresExhausted: %v", err)
	}
	return locked
}

func testLoginAttempts(t *testing.T, s auth.Store) {
	a := mustCreateUser(t, s, "a@example.com")
	for i := 1; i <= 3; i++ {
		if !reserve(t, s, a) {
			t.Fatalf("attempt %d: reserve = false, want true", i)
		}
	}
	if reserve(t, s, a) {
		t.Error("4th attempt: reserve = true, want false once 3 attempts are used")
	}
	if !lockIfExhausted(t, s, a) {
		t.Error("lock after 3 failures = false, want true")
	}
	if lockIfExhausted(t, s, a) {
		t.Error("second lock = true, want false (count was reset when locking)")
	}
	if err := s.ClearLoginFailures(t.Context(), a); err != nil {
		t.Fatalf("ClearLoginFailures: %v", err)
	}
	if reserve(t, s, a) {
		t.Error("reserve on locked account after ClearLoginFailures = true, want false")
	}

	b := mustCreateUser(t, s, "b@example.com")
	reserve(t, s, b)
	reserve(t, s, b)
	if lockIfExhausted(t, s, b) {
		t.Error("lock after 2 failures = true, want false")
	}
	if err := s.ClearLoginFailures(t.Context(), b); err != nil {
		t.Fatalf("ClearLoginFailures: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if !reserve(t, s, b) {
			t.Fatalf("attempt %d after clear: reserve = false, want true", i)
		}
	}
}

func testSessions(t *testing.T, s auth.Store) {
	ctx := t.Context()
	uid := mustCreateUser(t, s, "me@example.com")
	hash := bytes.Repeat([]byte{0xab}, 32)
	if _, err := s.SessionByTokenHash(ctx, hash); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("SessionByTokenHash before create: err = %v, want ErrNotFound", err)
	}

	err := s.CreateSession(ctx, auth.NewSession{TokenHash: hash, UserID: uid, LastSeenAt: base, ExpiresAt: base.Add(7 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.SessionByTokenHash(ctx, hash)
	if err != nil {
		t.Fatalf("SessionByTokenHash: %v", err)
	}
	if got.ID <= 0 || got.UserID != uid || got.Email != "me@example.com" ||
		!got.LastSeenAt.Equal(base) || !got.ExpiresAt.Equal(base.Add(7*24*time.Hour)) {
		t.Errorf("SessionByTokenHash = %+v, want user %d me@example.com, last seen %v, expires %v", got, uid, base, base.Add(7*24*time.Hour))
	}

	if err := s.TouchSession(ctx, got.ID, base.Add(time.Hour)); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}
	if got, _ := s.SessionByTokenHash(ctx, hash); !got.LastSeenAt.Equal(base.Add(time.Hour)) {
		t.Errorf("LastSeenAt after touch = %v, want %v", got.LastSeenAt, base.Add(time.Hour))
	}

	deletedFor, err := s.DeleteSession(ctx, hash)
	if err != nil || deletedFor != uid {
		t.Fatalf("DeleteSession = %d, %v; want %d, nil", deletedFor, err, uid)
	}
	if _, err := s.DeleteSession(ctx, hash); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("second DeleteSession: err = %v, want ErrNotFound", err)
	}
	if _, err := s.SessionByTokenHash(ctx, hash); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("SessionByTokenHash after delete: err = %v, want ErrNotFound", err)
	}
}

func testStaleSessions(t *testing.T, s auth.Store) {
	ctx := t.Context()
	now := base
	idleBefore := now.Add(-24 * time.Hour)
	a := mustCreateUser(t, s, "a@example.com")
	b := mustCreateUser(t, s, "b@example.com")

	sessions := []struct {
		name      string
		user      int64
		lastSeen  time.Time
		expires   time.Time
		wantAlive bool
	}{
		{"fresh", a, now.Add(-time.Hour), now.Add(time.Hour), true},
		{"idle", a, idleBefore, now.Add(time.Hour), false},
		{"expired", a, now.Add(-time.Hour), now, false},
		{"other user idle", b, idleBefore, now.Add(time.Hour), true},
	}
	for i, sess := range sessions {
		err := s.CreateSession(ctx, auth.NewSession{TokenHash: []byte{byte(i)}, UserID: sess.user, LastSeenAt: sess.lastSeen, ExpiresAt: sess.expires})
		if err != nil {
			t.Fatalf("CreateSession %s: %v", sess.name, err)
		}
	}

	if err := s.DeleteStaleSessions(ctx, a, now, idleBefore); err != nil {
		t.Fatalf("DeleteStaleSessions: %v", err)
	}
	for i, sess := range sessions {
		_, err := s.SessionByTokenHash(ctx, []byte{byte(i)})
		if alive := err == nil; alive != sess.wantAlive {
			t.Errorf("%s: alive = %v (err %v), want %v", sess.name, alive, err, sess.wantAlive)
		}
	}
}

func createReset(t *testing.T, s auth.Store, userID int64, hash []byte, expiresAt time.Time) {
	t.Helper()
	if err := s.CreatePasswordReset(t.Context(), userID, hash, expiresAt); err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}
}

func testPasswordReset(t *testing.T, s auth.Store) {
	ctx := t.Context()
	a := mustCreateUser(t, s, "a@example.com")
	b := mustCreateUser(t, s, "b@example.com")
	for i, uid := range []int64{a, a, b} {
		err := s.CreateSession(ctx, auth.NewSession{TokenHash: []byte{byte(i)}, UserID: uid, LastSeenAt: base, ExpiresAt: base.Add(time.Hour)})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	reserve(t, s, a)
	reserve(t, s, a)

	older, newer := []byte("older"), []byte("newer")
	createReset(t, s, a, older, base.Add(30*time.Minute))
	createReset(t, s, a, newer, base.Add(30*time.Minute))
	if _, err := s.ResetPassword(ctx, older, "new-hash", base); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("reset with superseded token: err = %v, want ErrNotFound", err)
	}

	uid, err := s.ResetPassword(ctx, newer, "new-hash", base)
	if err != nil || uid != a {
		t.Fatalf("ResetPassword = %d, %v; want %d, nil", uid, err, a)
	}
	if u, _ := s.UserByEmail(ctx, "a@example.com"); u.PasswordHash != "new-hash" {
		t.Errorf("password hash = %q, want new-hash", u.PasswordHash)
	}
	for i, wantAlive := range []bool{false, false, true} {
		if _, err := s.SessionByTokenHash(ctx, []byte{byte(i)}); (err == nil) != wantAlive {
			t.Errorf("session %d: alive = %v, want %v", i, err == nil, wantAlive)
		}
	}
	for i := 1; i <= 3; i++ {
		if !reserve(t, s, a) {
			t.Fatalf("attempt %d after reset: reserve = false, want true (failures cleared)", i)
		}
	}
	if _, err := s.ResetPassword(ctx, newer, "again", base); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("reusing a token: err = %v, want ErrNotFound", err)
	}

	expired := []byte("expired")
	createReset(t, s, b, expired, base)
	if _, err := s.ResetPassword(ctx, expired, "new-hash", base); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("expired token: err = %v, want ErrNotFound", err)
	}
	if u, _ := s.UserByEmail(ctx, "b@example.com"); u.PasswordHash != "hash-of-b@example.com" {
		t.Errorf("password changed by expired token: %q", u.PasswordHash)
	}
}

func testPasswordResetKeepsLock(t *testing.T, s auth.Store) {
	a := mustCreateUser(t, s, "a@example.com")
	for range 3 {
		reserve(t, s, a)
	}
	lockIfExhausted(t, s, a)
	createReset(t, s, a, []byte("token"), base.Add(time.Hour))
	if _, err := s.ResetPassword(t.Context(), []byte("token"), "new-hash", base); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if reserve(t, s, a) {
		t.Error("reserve after reset on a locked account = true, want false (only a manual unlock clears locked)")
	}
}

func testChangePassword(t *testing.T, s auth.Store) {
	ctx := t.Context()
	a := mustCreateUser(t, s, "a@example.com")
	b := mustCreateUser(t, s, "b@example.com")
	for i, uid := range []int64{a, b} {
		err := s.CreateSession(ctx, auth.NewSession{TokenHash: []byte{byte(i)}, UserID: uid, LastSeenAt: base, ExpiresAt: base.Add(time.Hour)})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	createReset(t, s, a, []byte("token"), base.Add(time.Hour))
	reserve(t, s, a)
	reserve(t, s, a)

	if err := s.ChangePassword(ctx, a, "new-hash"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if u, _ := s.UserByEmail(ctx, "a@example.com"); u.PasswordHash != "new-hash" {
		t.Errorf("password hash = %q, want new-hash", u.PasswordHash)
	}
	for i, wantAlive := range []bool{false, true} {
		if _, err := s.SessionByTokenHash(ctx, []byte{byte(i)}); (err == nil) != wantAlive {
			t.Errorf("session %d: alive = %v, want %v", i, err == nil, wantAlive)
		}
	}
	if _, err := s.ResetPassword(ctx, []byte("token"), "other", base); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("reset token after change: err = %v, want ErrNotFound", err)
	}
	for i := 1; i <= 3; i++ {
		if !reserve(t, s, a) {
			t.Fatalf("attempt %d after change: reserve = false, want true (failures cleared)", i)
		}
	}
}

func testUpdatePasswordHash(t *testing.T, s auth.Store) {
	ctx := t.Context()
	a := mustCreateUser(t, s, "a@example.com")
	err := s.CreateSession(ctx, auth.NewSession{TokenHash: []byte("s"), UserID: a, LastSeenAt: base, ExpiresAt: base.Add(time.Hour)})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.UpdatePasswordHash(ctx, a, "stronger-hash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	if u, _ := s.UserByEmail(ctx, "a@example.com"); u.PasswordHash != "stronger-hash" {
		t.Errorf("password hash = %q, want stronger-hash", u.PasswordHash)
	}
	if _, err := s.SessionByTokenHash(ctx, []byte("s")); err != nil {
		t.Errorf("session after UpdatePasswordHash: %v, want kept", err)
	}
}
