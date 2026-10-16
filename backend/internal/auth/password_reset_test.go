package auth_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type sentMail struct {
	to, token string
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
	err  error
}

func (m *fakeMailer) SendPasswordReset(_ context.Context, to, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, sentMail{to: to, token: token})
	return nil
}

func (m *fakeMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func (m *fakeMailer) last(t *testing.T) sentMail {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no mail sent")
	}
	return m.sent[len(m.sent)-1]
}

const newPassword = "N3w-password"

func forgotBody(email string) map[string]string {
	return map[string]string{"email": email}
}

func resetBody(token, password string) map[string]string {
	return map[string]string{"token": token, "password": password, "confirm_password": password}
}

func newResetEnv(t *testing.T) (*testEnv, *fakeMailer, string) {
	t.Helper()
	mailer := &fakeMailer{}
	e := newTestEnv(t, envOptions{mailer: mailer})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)
	expectStatus(t, "forgot", e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody(" ME@example.com")), http.StatusAccepted)
	return e, mailer, mailer.last(t).token
}

func TestForgotPasswordUnavailableWithoutMailer(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("me@example.com"))
	expectStatus(t, "forgot", r, http.StatusServiceUnavailable)
	expectBody(t, "forgot", r, `{"error":"password reset unavailable"}`)
}

func TestForgotPasswordLooksTheSameForUnknownEmail(t *testing.T) {
	mailer := &fakeMailer{}
	e := newTestEnv(t, envOptions{mailer: mailer})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	known := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("me@example.com"))
	unknown := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("nobody@example.com"))
	expectStatus(t, "known email", known, http.StatusAccepted)
	expectStatus(t, "unknown email", unknown, http.StatusAccepted)
	if known.body != unknown.body {
		t.Errorf("bodies differ: known %q, unknown %q", known.body, unknown.body)
	}
	if n := mailer.count(); n != 1 {
		t.Fatalf("sent %d mails, want 1 (only to the registered email)", n)
	}
	if to := mailer.last(t).to; to != "me@example.com" {
		t.Errorf("mail sent to %q, want me@example.com", to)
	}
}

func TestResetPasswordChangesPasswordAndLogsOutEverywhere(t *testing.T) {
	mailer := &fakeMailer{}
	e := newTestEnv(t, envOptions{mailer: mailer})
	device := e.client(t)
	expectStatus(t, "register", e.register(t, device, "me@example.com"), http.StatusCreated)
	expectStatus(t, "forgot", e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("me@example.com")), http.StatusAccepted)
	token := mailer.last(t).token

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword))
	expectStatus(t, "reset", r, http.StatusNoContent)

	expectStatus(t, "existing session after reset", e.do(t, device, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
	expectStatus(t, "old password", e.login(t, e.client(t), "me@example.com", goodPassword), http.StatusUnauthorized)
	expectStatus(t, "new password", e.login(t, e.client(t), "me@example.com", newPassword), http.StatusOK)

	reuse := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, "An0ther-password"))
	expectStatus(t, "reuse token", reuse, http.StatusBadRequest)
	expectBody(t, "reuse token", reuse, `{"error":"invalid or expired token"}`)
}

func TestResetTokenExpiresAfter30Minutes(t *testing.T) {
	e, _, token := newResetEnv(t)
	e.clock.Advance(30 * time.Minute)

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword))
	expectStatus(t, "reset after 30 minutes", r, http.StatusBadRequest)
	expectStatus(t, "old password still works", e.login(t, e.client(t), "me@example.com", goodPassword), http.StatusOK)
}

func TestResetPasswordValidation(t *testing.T) {
	e, _, token := newResetEnv(t)

	tests := map[string]struct {
		body  map[string]string
		field string
	}{
		"weak password":    {resetBody(token, "password"), "password"},
		"confirm mismatch": {map[string]string{"token": token, "password": newPassword, "confirm_password": "other"}, "confirm_password"},
	}
	for name, tt := range tests {
		r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", tt.body)
		expectStatus(t, name, r, http.StatusUnprocessableEntity)
		if fields, _ := r.json(t)["fields"].(map[string]any); fields[tt.field] == nil {
			t.Errorf("%s: body = %s, want fields.%s", name, r.body, tt.field)
		}
	}
	expectStatus(t, "token still usable after validation errors",
		e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword)), http.StatusNoContent)
}

func TestResetPasswordWithUnknownToken(t *testing.T) {
	e, _, _ := newResetEnv(t)
	for _, token := range []string{"", "not-a-real-token"} {
		r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword))
		expectStatus(t, "token "+token, r, http.StatusBadRequest)
	}
}

func TestResetPasswordDoesNotUnlock(t *testing.T) {
	e, _, token := newResetEnv(t)
	for range 3 {
		e.login(t, e.client(t), "me@example.com", "Wr0ngpass!")
	}
	expectStatus(t, "reset", e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword)), http.StatusNoContent)
	expectStatus(t, "new password on locked account", e.login(t, e.client(t), "me@example.com", newPassword), http.StatusLocked)
}

func TestMailerErrorReturnsInternalError(t *testing.T) {
	mailer := &fakeMailer{err: errors.New("smtp down")}
	e := newTestEnv(t, envOptions{mailer: mailer})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("me@example.com"))
	expectStatus(t, "forgot", r, http.StatusInternalServerError)
	if !strings.Contains(e.logs.String(), "smtp down") {
		t.Errorf("logs do not contain the mailer error:\n%s", e.logs)
	}
}

func TestResetLogsHaveNoTokenOrEmail(t *testing.T) {
	e, _, token := newResetEnv(t)
	e.do(t, e.client(t), http.MethodPost, "/api/auth/password/forgot", forgotBody("nobody@example.com"))
	e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody("wrong-token", newPassword))
	e.do(t, e.client(t), http.MethodPost, "/api/auth/password/reset", resetBody(token, newPassword))

	logs := e.logs.String()
	for _, secret := range []string{token, "me@example.com", "nobody@example.com", newPassword} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}
	for _, msg := range []string{`"msg":"password reset requested","user_id":1`, `"reason":"invalid_token"`, `"msg":"password reset","user_id":1`} {
		if !strings.Contains(logs, msg) {
			t.Errorf("logs do not contain %s:\n%s", msg, logs)
		}
	}
}
