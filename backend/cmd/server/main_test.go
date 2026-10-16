package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondl/authboss/v3"
	"golang.org/x/crypto/bcrypt"

	"nest-egg/backend/internal/auth"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

// oneUserStorer 只有一個使用者 me@example.com，密碼 Passw0rd!。
type oneUserStorer struct{ hash string }

func (s oneUserStorer) New(context.Context) authboss.User { return &auth.User{} }
func (s oneUserStorer) Create(context.Context, authboss.User) error {
	return authboss.ErrUserFound
}
func (s oneUserStorer) Save(context.Context, authboss.User) error { return nil }
func (s oneUserStorer) Load(_ context.Context, key string) (authboss.User, error) {
	if key != "me@example.com" {
		return nil, authboss.ErrUserNotFound
	}
	return &auth.User{ID: 1, Email: key, Password: s.hash}, nil
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	a, err := auth.New(auth.Config{SessionSecret: make([]byte, 32)}, oneUserStorer{hash: string(hash)}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	return newHandler(okPinger{}, a)
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

func TestHealthIsPublic(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	rec := serve(h, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok","database":"up"}` {
		t.Errorf("health = %d %s, want 200 {\"status\":\"ok\",\"database\":\"up\"}", rec.Code, rec.Body)
	}
}
