package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

// newTestHandler 只有一個使用者 me@example.com，密碼 Passw0rd!。
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := authtest.NewMemStore()
	if _, err := store.CreateUser(t.Context(), "me@example.com", string(hash)); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.NewService(store, auth.Config{BcryptCost: bcrypt.MinCost}, log)
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}
	return newHandler(okPinger{}, auth.NewHandler(svc, false, log), false)
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginRequest(password string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"email":"me@example.com","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestCrossSiteLoginIsRejected(t *testing.T) {
	h := newTestHandler(t)
	req := loginRequest("Passw0rd!")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	rec := serve(h, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if c := rec.Result().Cookies(); len(c) != 0 {
		t.Errorf("cross-site login set cookies %v, want none", c)
	}
}

func TestForeignOriginLogoutIsRejected(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")

	if rec := serve(h, req); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestSameOriginLoginSucceeds(t *testing.T) {
	h := newTestHandler(t)
	req := loginRequest("Passw0rd!")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if c := rec.Result().Cookies(); len(c) != 1 {
		t.Errorf("got %d cookies, want 1 session cookie", len(c))
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	h := newTestHandler(t)
	big := `{"email":"me@example.com","password":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")

	rec := serve(h, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 (body %s)", rec.Code, rec.Body)
	}
}

func TestServerHasTimeouts(t *testing.T) {
	srv := newServer(":0", http.NotFoundHandler())
	timeouts := map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	}
	for name, d := range timeouts {
		if d <= 0 {
			t.Errorf("%s = %v, want > 0", name, d)
		}
	}
}

func TestSecurityHeadersAreWired(t *testing.T) {
	h := newTestHandler(t)
	blocked := loginRequest("Passw0rd!")
	blocked.Header.Set("Sec-Fetch-Site", "cross-site")

	for name, req := range map[string]*http.Request{
		"health":             httptest.NewRequest(http.MethodGet, "/api/health", nil),
		"cross-site blocked": blocked,
	} {
		rec := serve(h, req)
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", name, got)
		}
		if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("%s: Strict-Transport-Security = %q, want none when hsts is off", name, got)
		}
	}
}

func TestHealthIsPublic(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	rec := serve(h, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok","database":"up"}` {
		t.Errorf("health = %d %s, want 200 {\"status\":\"ok\",\"database\":\"up\"}", rec.Code, rec.Body)
	}
}
