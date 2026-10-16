package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

const goodPassword = "Passw0rd!"

type testEnv struct {
	srv    *httptest.Server
	storer *memStorer
	logs   *bytes.Buffer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	storer := newMemStorer()
	logs := &bytes.Buffer{}
	a, err := New(Config{SessionSecret: testSecret, CookieSecure: false}, storer, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mux := http.NewServeMux()
	a.Routes(mux)
	mux.Handle("GET /api/protected", a.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	srv := httptest.NewServer(a.Middleware(mux))
	t.Cleanup(srv.Close)
	return &testEnv{srv: srv, storer: storer, logs: logs}
}

func (e *testEnv) client(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

type response struct {
	status int
	body   string
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil {
		t.Fatalf("parse body %q: %v", r.body, err)
	}
	return m
}

func (e *testEnv) do(t *testing.T, c *http.Client, method, path string, body any) response {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: res.StatusCode, body: string(b)}
}

func registerBody(email, password, confirm string) map[string]string {
	return map[string]string{"email": email, "password": password, "confirm_password": confirm}
}

func loginBody(email, password string) map[string]string {
	return map[string]string{"email": email, "password": password}
}

func (e *testEnv) register(t *testing.T, c *http.Client, email string) response {
	t.Helper()
	return e.do(t, c, http.MethodPost, "/api/auth/register", registerBody(email, goodPassword, goodPassword))
}

func expectStatus(t *testing.T, what string, r response, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("%s: status = %d, want %d (body %s)", what, r.status, want, r.body)
	}
}

func TestRegisterLogsInThenLogout(t *testing.T) {
	e := newTestEnv(t)
	c := e.client(t)

	r := e.register(t, c, "Me@Example.com")
	expectStatus(t, "register", r, http.StatusOK)
	if m := r.json(t); m["status"] != "success" || m["location"] != "/" {
		t.Errorf("register body = %v, want status success, location /", m)
	}
	if _, ok := e.storer.get("me@example.com"); !ok {
		t.Error("user me@example.com not stored")
	}

	me := e.do(t, c, http.MethodGet, "/api/auth/me", nil)
	expectStatus(t, "me after register", me, http.StatusOK)
	if m := me.json(t); len(m) != 1 || m["email"] != "me@example.com" {
		t.Errorf("me body = %v, want only {email: me@example.com}", m)
	}

	out := e.do(t, c, http.MethodPost, "/api/auth/logout", nil)
	expectStatus(t, "logout", out, http.StatusOK)
	if m := out.json(t); m["status"] != "success" || m["location"] != "/login" {
		t.Errorf("logout body = %v, want status success, location /login", m)
	}

	expectStatus(t, "me after logout", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
}

func TestRegisterDuplicateEmail(t *testing.T) {
	e := newTestEnv(t)
	expectStatus(t, "first register", e.register(t, e.client(t), "me@example.com"), http.StatusOK)

	c := e.client(t)
	r := e.register(t, c, "ME@example.com")
	expectStatus(t, "duplicate register", r, http.StatusConflict)
	if m := r.json(t); m["status"] != "failure" {
		t.Errorf("body = %v, want status failure", m)
	}
	expectStatus(t, "me after duplicate", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
}

func TestRegisterValidationErrors(t *testing.T) {
	tests := map[string]struct {
		body  map[string]string
		field string
	}{
		"weak password":    {registerBody("me@example.com", "short", "short"), "password"},
		"confirm mismatch": {registerBody("me@example.com", goodPassword, "Passw0rd?"), "confirm_password"},
		"invalid email":    {registerBody("not-an-email", goodPassword, goodPassword), "email"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			c := e.client(t)

			r := e.do(t, c, http.MethodPost, "/api/auth/register", tt.body)
			expectStatus(t, "register", r, http.StatusUnprocessableEntity)
			errs, _ := r.json(t)["errors"].(map[string]any)
			if _, ok := errs[tt.field]; !ok {
				t.Errorf("errors = %v, want key %q", errs, tt.field)
			}
			if _, ok := e.storer.get("me@example.com"); ok {
				t.Error("user stored despite validation error")
			}
			expectStatus(t, "me", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
		})
	}
}

func TestLoginWithCorrectPassword(t *testing.T) {
	e := newTestEnv(t)
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusOK)

	c := e.client(t)
	r := e.do(t, c, http.MethodPost, "/api/auth/login", loginBody("ME@example.com", goodPassword))
	expectStatus(t, "login", r, http.StatusOK)
	if m := r.json(t); m["status"] != "success" || m["location"] != "/" {
		t.Errorf("login body = %v, want status success, location /", m)
	}
	expectStatus(t, "me after login", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusOK)
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	e := newTestEnv(t)
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusOK)

	c := e.client(t)
	wrong := e.do(t, c, http.MethodPost, "/api/auth/login", loginBody("me@example.com", "Wr0ngpass!"))
	expectStatus(t, "wrong password", wrong, http.StatusUnauthorized)
	unknown := e.do(t, c, http.MethodPost, "/api/auth/login", loginBody("nobody@example.com", goodPassword))
	expectStatus(t, "unknown user", unknown, http.StatusUnauthorized)

	if wrong.body != unknown.body {
		t.Errorf("bodies differ: wrong password %s, unknown user %s", wrong.body, unknown.body)
	}
	expectStatus(t, "me after failed logins", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
}

func TestStorerErrorReturnsInternalError(t *testing.T) {
	e := newTestEnv(t)
	e.storer.setFail(errors.New("connection refused"))

	r := e.do(t, e.client(t), http.MethodPost, "/api/auth/login", loginBody("me@example.com", goodPassword))
	expectStatus(t, "login", r, http.StatusInternalServerError)
	if m := r.json(t); len(m) != 1 || m["error"] != "internal error" {
		t.Errorf("body = %v, want only {error: internal error}", m)
	}
}

func TestRequireUser(t *testing.T) {
	e := newTestEnv(t)
	c := e.client(t)

	expectStatus(t, "protected without login", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusUnauthorized)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusOK)
	expectStatus(t, "protected after login", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusNoContent)
}

func TestRequireUserStorerErrorReturnsInternalError(t *testing.T) {
	e := newTestEnv(t)
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusOK)
	e.storer.setFail(errors.New("connection refused"))

	r := e.do(t, c, http.MethodGet, "/api/auth/me", nil)
	expectStatus(t, "me", r, http.StatusInternalServerError)
	if m := r.json(t); len(m) != 1 || m["error"] != "internal error" {
		t.Errorf("body = %v, want only {error: internal error}", m)
	}
}
