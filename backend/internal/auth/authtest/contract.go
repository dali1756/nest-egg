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
	t.Run("third attempt in flight", func(t *testing.T) { testThirdAttemptInFlight(t, newStore(t)) })
	t.Run("sessions", func(t *testing.T) { testSessions(t, newStore(t)) })
	t.Run("stale sessions", func(t *testing.T) { testStaleSessions(t, newStore(t)) })
	t.Run("password reset", func(t *testing.T) { testPasswordReset(t, newStore(t)) })
	t.Run("password reset unlocks", func(t *testing.T) { testPasswordResetUnlocks(t, newStore(t)) })
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

const (
	maxFailures = 3
	firstLock   = 10 * time.Minute
	repeatLock  = 30 * time.Minute
)

func reserveAt(t *testing.T, s auth.Store, id int64, now time.Time) auth.LoginAttempt {
	t.Helper()
	got, err := s.ReserveLoginAttempt(t.Context(), id, auth.AttemptLimit{
		MaxFailures:     maxFailures,
		Now:             now,
		FirstLockUntil:  now.Add(firstLock),
		RepeatLockUntil: now.Add(repeatLock),
	})
	if err != nil {
		t.Fatalf("ReserveLoginAttempt: %v", err)
	}
	return got
}

// 在 now 連續預留到上鎖為止，回傳鎖到什麼時候，前幾次不能上鎖，代表一開始的失敗次數是 0
func lockOut(t *testing.T, s auth.Store, id int64, now time.Time) time.Time {
	t.Helper()
	for i := 1; i < maxFailures; i++ {
		if got := reserveAt(t, s, id, now); !got.Reserved || !got.LockedUntil.IsZero() {
			t.Fatalf("attempt %d at %v = %+v, want reserved without a lock", i, now, got)
		}
	}
	got := reserveAt(t, s, id, now)
	if !got.Reserved || got.LockedUntil.IsZero() {
		t.Fatalf("attempt %d at %v = %+v, want reserved and locked", maxFailures, now, got)
	}
	return got.LockedUntil
}

func expectLockedUntil(t *testing.T, what string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s: locked until %v, want %v", what, got, want)
	}
}

func testLoginAttempts(t *testing.T, s auth.Store) {
	a := mustCreateUser(t, s, "a@example.com")
	b := mustCreateUser(t, s, "b@example.com")
	firstUntil := lockOut(t, s, a, base)
	expectLockedUntil(t, "first lock", firstUntil, base.Add(firstLock))

	during := base.Add(4 * time.Minute)
	if got := reserveAt(t, s, a, during); got.Reserved || !got.LockedUntil.Equal(firstUntil) {
		t.Errorf("reserve while locked = %+v, want rejected until %v", got, firstUntil)
	}
	if got := reserveAt(t, s, b, during); !got.Reserved || !got.LockedUntil.IsZero() {
		t.Errorf("another user while a is locked = %+v, want reserved without a lock", got)
	}

	// 第 3 次之後沒有呼叫 ClearLoginFailures（比對中斷）也一樣：時間到就重新計算，被拒絕的預留不算次數
	second := firstUntil
	secondUntil := lockOut(t, s, a, second)
	expectLockedUntil(t, "lock after the first one expired", secondUntil, second.Add(repeatLock))
	third := secondUntil
	expectLockedUntil(t, "every later lock", lockOut(t, s, a, third), third.Add(repeatLock))

	if err := s.ClearLoginFailures(t.Context(), a); err != nil {
		t.Fatalf("ClearLoginFailures: %v", err)
	}
	afterSuccess := third.Add(repeatLock)
	expectLockedUntil(t, "lock after a successful login", lockOut(t, s, a, afterSuccess), afterSuccess.Add(firstLock))
}

func testThirdAttemptInFlight(t *testing.T, s auth.Store) {
	a := mustCreateUser(t, s, "a@example.com")
	until := lockOut(t, s, a, base)
	if got := reserveAt(t, s, a, base); got.Reserved || !got.LockedUntil.Equal(until) {
		t.Errorf("reserve while the 3rd attempt is being checked = %+v, want rejected until %v", got, until)
	}
	if err := s.ClearLoginFailures(t.Context(), a); err != nil {
		t.Fatalf("ClearLoginFailures: %v", err)
	}
	expectLockedUntil(t, "lock after the 3rd attempt turned out right", lockOut(t, s, a, base), base.Add(firstLock))
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
	reserveAt(t, s, a, base)
	reserveAt(t, s, a, base)

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
	lockOut(t, s, a, base)
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

func testPasswordResetUnlocks(t *testing.T, s auth.Store) {
	a := mustCreateUser(t, s, "a@example.com")
	lockOut(t, s, a, base)
	createReset(t, s, a, []byte("token"), base.Add(time.Hour))
	if _, err := s.ResetPassword(t.Context(), []byte("token"), "new-hash", base); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	expectLockedUntil(t, "lock after a reset", lockOut(t, s, a, base), base.Add(firstLock))
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
	reserveAt(t, s, a, base)
	reserveAt(t, s, a, base)

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
	lockOut(t, s, a, base)
}

func testUpdatePasswordHash(t *testing.T, s auth.Store) {
	ctx := t.Context()
	a := mustCreateUser(t, s, "a@example.com")
	err := s.CreateSession(ctx, auth.NewSession{TokenHash: []byte("s"), UserID: a, LastSeenAt: base, ExpiresAt: base.Add(time.Hour)})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	lockOut(t, s, a, base)
	if err := s.UpdatePasswordHash(ctx, a, "stronger-hash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	if u, _ := s.UserByEmail(ctx, "a@example.com"); u.PasswordHash != "stronger-hash" {
		t.Errorf("password hash = %q, want stronger-hash", u.PasswordHash)
	}
	if _, err := s.SessionByTokenHash(ctx, []byte("s")); err != nil {
		t.Errorf("session after UpdatePasswordHash: %v, want kept", err)
	}
	expectLockedUntil(t, "lock after UpdatePasswordHash", lockOut(t, s, a, base), base.Add(firstLock))
}
