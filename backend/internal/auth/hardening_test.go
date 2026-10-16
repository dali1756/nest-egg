package auth_test

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
	"nest-egg/backend/internal/platform/ratelimit"
)

var errorAfterValidation = errors.New("store must not be called")

func changeBody(current, password string) map[string]string {
	return map[string]string{"current_password": current, "password": password, "confirm_password": password}
}

func TestCredentialEndpointsUseTheRateLimit(t *testing.T) {
	e := newTestEnv(t, envOptions{rateLimit: ratelimit.New(1, time.Hour, nil).Middleware})
	c := e.client(t)
	expectStatus(t, "register uses the only token", e.register(t, c, "me@example.com"), http.StatusCreated)

	limited := map[string]response{
		"register": e.register(t, e.client(t), "other@example.com"),
		"login":    e.login(t, e.client(t), "me@example.com", goodPassword),
		"forgot":   e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("me@example.com")),
		"reset":    e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody("token", newPassword)),
		"change":   e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody(goodPassword, newPassword)),
	}
	for name, r := range limited {
		if r.status != http.StatusTooManyRequests {
			t.Errorf("%s = %d, want 429", name, r.status)
		}
	}
	expectStatus(t, "me is not limited", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusOK)
	expectStatus(t, "logout is not limited", e.do(t, c, http.MethodPost, "/api/auth/logout", nil), http.StatusNoContent)
}

func TestChangePasswordLogsOutOtherDevices(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	current, other := e.client(t), e.client(t)
	expectStatus(t, "register", e.register(t, current, "me@example.com"), http.StatusCreated)
	expectStatus(t, "login on another device", e.login(t, other, "me@example.com", goodPassword), http.StatusOK)

	r := e.do(t, current, http.MethodPost, "/api/auth/password/change", changeBody(goodPassword, newPassword))
	expectStatus(t, "change", r, http.StatusNoContent)
	sessionCookie(t, r)

	expectStatus(t, "current device keeps working", e.do(t, current, http.MethodGet, "/api/auth/me", nil), http.StatusOK)
	expectStatus(t, "other device is logged out", e.do(t, other, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
	expectStatus(t, "old password", e.login(t, e.client(t), "me@example.com", goodPassword), http.StatusUnauthorized)
	expectStatus(t, "new password", e.login(t, e.client(t), "me@example.com", newPassword), http.StatusOK)
	if n := e.store.SessionCount(); n != 2 {
		t.Errorf("sessions = %d, want 2 (rotated current one + new login)", n)
	}
}

func TestChangePasswordRotatesTheCurrentSession(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	before := sessionCookie(t, e.register(t, c, "me@example.com"))

	after := sessionCookie(t, e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody(goodPassword, newPassword)))
	if after.Value == before.Value {
		t.Error("session token unchanged after password change, want a new one")
	}
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/auth/me", nil)
	req.AddCookie(before)
	expectStatus(t, "old token after change", e.send(t, http.DefaultClient, req), http.StatusUnauthorized)
}

func TestChangePasswordWrongCurrentPasswordCountsTowardLock(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)

	r := e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody("Wr0ngpass!", newPassword))
	expectStatus(t, "wrong current password", r, http.StatusForbidden)
	expectBody(t, "wrong current password", r, `{"error":"current password is incorrect"}`)
	e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody("Wr0ngpass!", newPassword))
	r = e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody("Wr0ngpass!", newPassword))
	expectStatus(t, "third wrong current password", r, http.StatusLocked)
	if !e.store.Locked("me@example.com") {
		t.Error("account not locked after 3 wrong current passwords")
	}
	expectStatus(t, "login while locked", e.login(t, e.client(t), "me@example.com", goodPassword), http.StatusLocked)
}

func TestChangePasswordValidationDoesNotCountAsFailure(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)

	for range 3 {
		r := e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody("Wr0ngpass!", "weak"))
		expectStatus(t, "weak new password", r, http.StatusUnprocessableEntity)
	}
	if e.store.Locked("me@example.com") {
		t.Error("locked by requests that never checked the current password")
	}
}

func TestChangePasswordRequiresLogin(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/change", changeBody(goodPassword, newPassword))
	expectStatus(t, "change without session", r, http.StatusUnauthorized)
}

func TestChangePasswordInvalidatesResetTokens(t *testing.T) {
	e, _, token := newResetEnv(t)
	c := e.client(t)
	expectStatus(t, "login", e.login(t, c, "me@example.com", goodPassword), http.StatusOK)
	expectStatus(t, "change", e.do(t, c, http.MethodPost, "/api/auth/password/change", changeBody(goodPassword, newPassword)), http.StatusNoContent)

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, "An0ther-password"))
	expectStatus(t, "reset with a token issued before the change", r, http.StatusBadRequest)
}

func TestLoginUpgradesWeakerHashes(t *testing.T) {
	store := authtest.NewMemStore()
	old, err := bcrypt.GenerateFromPassword([]byte(goodPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(t.Context(), "me@example.com", string(old)); err != nil {
		t.Fatal(err)
	}
	svc, err := auth.NewService(store, auth.Config{BcryptCost: bcrypt.MinCost + 1}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.Login(t.Context(), "me@example.com", goodPassword); err != nil {
		t.Fatalf("Login: %v", err)
	}
	hash := store.PasswordHash("me@example.com")
	if cost, _ := bcrypt.Cost([]byte(hash)); cost != bcrypt.MinCost+1 {
		t.Errorf("cost after login = %d, want upgraded to %d", cost, bcrypt.MinCost+1)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(goodPassword)) != nil {
		t.Error("upgraded hash does not match the password")
	}
}

func TestDefaultBcryptCostIs12(t *testing.T) {
	store := authtest.NewMemStore()
	svc, err := auth.NewService(store, auth.Config{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Register(t.Context(), auth.RegisterInput{Email: "me@example.com", Password: goodPassword, ConfirmPassword: goodPassword}); err != nil {
		t.Fatal(err)
	}
	if cost, _ := bcrypt.Cost([]byte(store.PasswordHash("me@example.com"))); cost != 12 {
		t.Errorf("bcrypt cost = %d, want 12", cost)
	}
}

func TestOversizedInputsAreRejectedEarly(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)
	e.store.SetError(errorAfterValidation)

	long := strings.Repeat("a", 300) + "@example.com"
	expectStatus(t, "login with a 300-character email", e.login(t, e.client(t), long, goodPassword), http.StatusUnauthorized)

	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "nest_egg_session", Value: strings.Repeat("A", 100)})
	expectStatus(t, "100-character session token", e.send(t, http.DefaultClient, req), http.StatusUnauthorized)

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(strings.Repeat("A", 100), newPassword))
	expectStatus(t, "100-character reset token", r, http.StatusBadRequest)
}

func TestRegisterRejectsNonASCIIEmail(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	r := e.register(t, e.client(t), "ｍｅ@example.com")
	expectStatus(t, "full-width look-alike", r, http.StatusUnprocessableEntity)
}
