package auth_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
	"nest-egg/backend/internal/platform/requestid"
)

const goodPassword = "Passw0rd!"

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testEnv struct {
	srv   *httptest.Server
	store *authtest.MemStore
	clock *fakeClock
	logs  *syncBuffer
}

type envOptions struct {
	cookieSecure bool
	mailer       auth.Mailer
}

func newTestEnv(t *testing.T, opts envOptions) *testEnv {
	t.Helper()
	e := &testEnv{
		store: authtest.NewMemStore(),
		clock: &fakeClock{now: time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)},
		logs:  &syncBuffer{},
	}
	log := slog.New(requestid.NewLogHandler(slog.NewJSONHandler(e.logs, nil)))
	svc, err := auth.NewService(e.store, auth.Config{BcryptCost: bcrypt.MinCost, Now: e.clock.Now, Mailer: opts.mailer}, log)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := auth.NewHandler(svc, auth.HandlerConfig{CookieSecure: opts.cookieSecure}, log)

	public, protected := http.NewServeMux(), http.NewServeMux()
	public.HandleFunc("POST /api/auth/register", h.Register)
	public.HandleFunc("POST /api/auth/login", h.Login)
	public.HandleFunc("POST /api/auth/logout", h.Logout)
	public.HandleFunc("POST /api/auth/password/forgot", h.ForgotPassword)
	public.HandleFunc("POST /api/auth/password/reset", h.ResetPassword)
	protected.HandleFunc("GET /api/auth/me", h.Me)
	protected.HandleFunc("POST /api/auth/password/change", h.ChangePassword)
	protected.HandleFunc("GET /api/protected", func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.CurrentUser(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-User", u.Email)
		w.WriteHeader(http.StatusNoContent)
	})
	public.Handle("/api/", h.RequireUser(protected))
	e.srv = httptest.NewServer(requestid.Middleware(public))
	t.Cleanup(e.srv.Close)
	return e
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
	status  int
	body    string
	header  http.Header
	cookies []*http.Cookie
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil {
		t.Fatalf("parse body %q: %v", r.body, err)
	}
	return m
}

func (e *testEnv) send(t *testing.T, c *http.Client, req *http.Request) response {
	t.Helper()
	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: res.StatusCode, body: string(b), header: res.Header, cookies: res.Cookies()}
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
	return e.send(t, c, req)
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

func (e *testEnv) login(t *testing.T, c *http.Client, email, password string) response {
	t.Helper()
	return e.do(t, c, http.MethodPost, "/api/auth/login", loginBody(email, password))
}

func expectStatus(t *testing.T, what string, r response, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("%s: status = %d, want %d (body %s)", what, r.status, want, r.body)
	}
}

func expectBody(t *testing.T, what string, r response, want string) {
	t.Helper()
	if got := strings.TrimSpace(r.body); got != want {
		t.Errorf("%s: body = %s, want %s", what, got, want)
	}
}

func sessionCookie(t *testing.T, r response) *http.Cookie {
	t.Helper()
	for _, c := range r.cookies {
		if c.Name == "nest_egg_session" || c.Name == "__Host-nest_egg_session" {
			return c
		}
	}
	t.Fatalf("no nest_egg_session cookie in response (cookies %v)", r.cookies)
	return nil
}

func TestRegisterLogsInThenLogout(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)

	r := e.register(t, c, "Me@Example.com")
	expectStatus(t, "register", r, http.StatusCreated)
	expectBody(t, "register", r, `{"email":"me@example.com"}`)
	ck := sessionCookie(t, r)
	if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" || ck.MaxAge != 7*24*60*60 || ck.Secure {
		t.Errorf("cookie = %+v, want HttpOnly, SameSite=Lax, Path=/, MaxAge=604800, not Secure", ck)
	}

	me := e.do(t, c, http.MethodGet, "/api/auth/me", nil)
	expectStatus(t, "me after register", me, http.StatusOK)
	expectBody(t, "me", me, `{"email":"me@example.com"}`)

	out := e.do(t, c, http.MethodPost, "/api/auth/logout", nil)
	expectStatus(t, "logout", out, http.StatusNoContent)
	if ck := sessionCookie(t, out); ck.MaxAge >= 0 {
		t.Errorf("logout cookie MaxAge = %d, want < 0 (deleted)", ck.MaxAge)
	}
	expectStatus(t, "me after logout", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
}

func TestSecureCookieUsesHostPrefix(t *testing.T) {
	e := newTestEnv(t, envOptions{cookieSecure: true})
	ck := sessionCookie(t, e.register(t, e.client(t), "me@example.com"))
	if !ck.Secure || ck.Name != "__Host-nest_egg_session" || ck.Path != "/" || ck.Domain != "" {
		t.Fatalf("cookie = %+v, want __Host-nest_egg_session, Secure, Path=/, no Domain", ck)
	}

	me := func(name string) int {
		req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: name, Value: ck.Value})
		return e.send(t, http.DefaultClient, req).status
	}
	if got := me("__Host-nest_egg_session"); got != http.StatusOK {
		t.Errorf("me with __Host- cookie = %d, want 200", got)
	}
	if got := me("nest_egg_session"); got != http.StatusUnauthorized {
		t.Errorf("me with unprefixed cookie = %d, want 401 when cookies are secure", got)
	}
}

func TestLogoutRevokesCopiedCookie(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	stolen := sessionCookie(t, e.register(t, c, "me@example.com"))

	expectStatus(t, "logout", e.do(t, c, http.MethodPost, "/api/auth/logout", nil), http.StatusNoContent)

	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/auth/me", nil)
	req.AddCookie(stolen)
	expectStatus(t, "replay cookie after logout", e.send(t, http.DefaultClient, req), http.StatusUnauthorized)
	if n := e.store.SessionCount(); n != 0 {
		t.Errorf("sessions after logout = %d, want 0", n)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "first register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	r := e.register(t, e.client(t), "ME@example.com")
	expectStatus(t, "duplicate register", r, http.StatusConflict)
	expectBody(t, "duplicate register", r, `{"error":"email already registered"}`)
	if len(r.cookies) != 0 {
		t.Errorf("duplicate register set cookies %v, want none", r.cookies)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	tests := map[string]struct {
		body  map[string]string
		field string
	}{
		"weak password":       {registerBody("me@example.com", "password", "password"), "password"},
		"password with space": {registerBody("me@example.com", "Pass w0rd!", "Pass w0rd!"), "password"},
		"confirm mismatch":    {registerBody("me@example.com", goodPassword, "Passw0rd?"), "confirm_password"},
		"invalid email":       {registerBody("not-an-email", goodPassword, goodPassword), "email"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, envOptions{})
			c := e.client(t)

			r := e.do(t, c, http.MethodPost, "/api/auth/register", tt.body)
			expectStatus(t, "register", r, http.StatusUnprocessableEntity)
			m := r.json(t)
			fields, _ := m["fields"].(map[string]any)
			if m["error"] != "invalid input" || fields[tt.field] == nil {
				t.Errorf("body = %v, want error \"invalid input\" with fields.%s", m, tt.field)
			}
			if len(r.cookies) != 0 {
				t.Errorf("cookies %v, want none", r.cookies)
			}
			expectStatus(t, "register valid afterwards", e.register(t, c, "me@example.com"), http.StatusCreated)
		})
	}
}

func TestInvalidJSONIsBadRequest(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/api/auth/login", strings.NewReader(`{"email":`))
	r := e.send(t, e.client(t), req)
	expectStatus(t, "login", r, http.StatusBadRequest)
	expectBody(t, "login", r, `{"error":"invalid json"}`)
}

func TestLoginWithCorrectPassword(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	c := e.client(t)
	r := e.login(t, c, " ME@example.com", goodPassword)
	expectStatus(t, "login", r, http.StatusOK)
	expectBody(t, "login", r, `{"email":"me@example.com"}`)
	sessionCookie(t, r)
	expectStatus(t, "me after login", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusOK)
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)

	c := e.client(t)
	wrong := e.login(t, c, "me@example.com", "Wr0ngpass!")
	unknown := e.login(t, c, "nobody@example.com", goodPassword)
	expectStatus(t, "wrong password", wrong, http.StatusUnauthorized)
	expectStatus(t, "unknown email", unknown, http.StatusUnauthorized)
	if wrong.body != unknown.body {
		t.Errorf("bodies differ: wrong password %s, unknown email %s", wrong.body, unknown.body)
	}
	expectStatus(t, "me after failed logins", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)
}

func TestUnknownEmailTakesAsLongAsWrongPassword(t *testing.T) {
	store := authtest.NewMemStore()
	svc, err := auth.NewService(store, auth.Config{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if _, _, err := svc.Register(ctx, auth.RegisterInput{Email: "me@example.com", Password: goodPassword, ConfirmPassword: goodPassword}); err != nil {
		t.Fatal(err)
	}
	fastest := func(email string) time.Duration {
		best := time.Hour
		for range 2 {
			start := time.Now()
			if _, _, err := svc.Login(ctx, email, "Wr0ngpass!"); !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Fatalf("Login(%s) err = %v, want ErrInvalidCredentials", email, err)
			}
			best = min(best, time.Since(start))
		}
		return best
	}
	wrong := fastest("me@example.com")
	store.ClearLoginFailures(ctx, 1)
	unknown := fastest("nobody@example.com")
	if unknown < wrong/2 {
		t.Errorf("unknown email took %v, wrong password %v: the difference reveals whether an email is registered", unknown, wrong)
	}
}

func TestLockAfterThreeFailures(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)
	c := e.client(t)

	expectStatus(t, "failure 1", e.login(t, c, "me@example.com", "Wr0ngpass!"), http.StatusUnauthorized)
	expectStatus(t, "failure 2", e.login(t, c, "me@example.com", "Wr0ngpass!"), http.StatusUnauthorized)
	third := e.login(t, c, "me@example.com", "Wr0ngpass!")
	expectStatus(t, "failure 3", third, http.StatusLocked)
	expectBody(t, "failure 3", third, `{"error":"account locked"}`)
	if !e.store.Locked("me@example.com") {
		t.Fatal("account not locked after 3 failures")
	}

	expectStatus(t, "correct password while locked", e.login(t, c, "me@example.com", goodPassword), http.StatusLocked)
	expectStatus(t, "failed logins create no session", e.do(t, c, http.MethodGet, "/api/auth/me", nil), http.StatusUnauthorized)

	e.store.Unlock("me@example.com")
	expectStatus(t, "login after unlock", e.login(t, c, "me@example.com", goodPassword), http.StatusOK)
}

func TestSuccessfulLoginResetsFailures(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	expectStatus(t, "register", e.register(t, e.client(t), "me@example.com"), http.StatusCreated)
	c := e.client(t)

	e.login(t, c, "me@example.com", "Wr0ngpass!")
	e.login(t, c, "me@example.com", "Wr0ngpass!")
	expectStatus(t, "correct password", e.login(t, c, "me@example.com", goodPassword), http.StatusOK)
	e.login(t, c, "me@example.com", "Wr0ngpass!")
	expectStatus(t, "2nd failure after success", e.login(t, c, "me@example.com", "Wr0ngpass!"), http.StatusUnauthorized)
	if e.store.Locked("me@example.com") {
		t.Error("locked after 2+2 failures separated by a success, want unlocked")
	}
}

func TestLockDoesNotEndExistingSessions(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	owner := e.client(t)
	expectStatus(t, "register", e.register(t, owner, "me@example.com"), http.StatusCreated)

	attacker := e.client(t)
	for range 3 {
		e.login(t, attacker, "me@example.com", "Wr0ngpass!")
	}
	if !e.store.Locked("me@example.com") {
		t.Fatal("account not locked")
	}
	expectStatus(t, "owner's session", e.do(t, owner, http.MethodGet, "/api/auth/me", nil), http.StatusOK)
}

func TestSessionIdleTimeout(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)

	e.clock.Advance(24*time.Hour - time.Minute)
	expectStatus(t, "just under 24h idle", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusNoContent)
	e.clock.Advance(24*time.Hour - time.Minute)
	expectStatus(t, "activity extends the session", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusNoContent)

	e.clock.Advance(24 * time.Hour)
	r := e.do(t, c, http.MethodGet, "/api/protected", nil)
	expectStatus(t, "24h idle", r, http.StatusUnauthorized)
	if ck := sessionCookie(t, r); ck.MaxAge >= 0 {
		t.Errorf("expired session cookie MaxAge = %d, want < 0 (deleted)", ck.MaxAge)
	}
	if n := e.store.SessionCount(); n != 0 {
		t.Errorf("sessions = %d, want expired session deleted", n)
	}
}

func TestTouchIsThrottledToOncePerMinute(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)

	e.clock.Advance(30 * time.Second)
	expectStatus(t, "request after 30s", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusNoContent)
	e.clock.Advance(24*time.Hour - 20*time.Second)
	expectStatus(t, "24h after the last recorded activity", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusUnauthorized)
}

func TestSessionLifetimeIsSevenDays(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)

	for elapsed := 20 * time.Hour; elapsed < 7*24*time.Hour; elapsed += 20 * time.Hour {
		e.clock.Advance(20 * time.Hour)
		expectStatus(t, "active use at "+elapsed.String(), e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusNoContent)
	}
	e.clock.Advance(8 * time.Hour)
	expectStatus(t, "7 days after login", e.do(t, c, http.MethodGet, "/api/protected", nil), http.StatusUnauthorized)
}

func TestRequireUser(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)

	r := e.do(t, c, http.MethodGet, "/api/protected", nil)
	expectStatus(t, "without cookie", r, http.StatusUnauthorized)
	expectBody(t, "without cookie", r, `{"error":"unauthorized"}`)
	if len(r.cookies) != 0 {
		t.Errorf("without cookie: set cookies %v, want none", r.cookies)
	}

	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: "nest_egg_session", Value: "forged"})
	r = e.send(t, http.DefaultClient, req)
	expectStatus(t, "forged cookie", r, http.StatusUnauthorized)
	if ck := sessionCookie(t, r); ck.MaxAge >= 0 {
		t.Errorf("forged cookie: MaxAge = %d, want < 0 (deleted)", ck.MaxAge)
	}

	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)
	r = e.do(t, c, http.MethodGet, "/api/protected", nil)
	expectStatus(t, "logged in", r, http.StatusNoContent)
	if got := r.header.Get("X-User"); got != "me@example.com" {
		t.Errorf("CurrentUser email = %q, want me@example.com", got)
	}
}

func TestStoreErrorReturnsInternalError(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	expectStatus(t, "register", e.register(t, c, "me@example.com"), http.StatusCreated)
	e.store.SetError(errors.New("connection refused"))

	for _, r := range []response{
		e.login(t, e.client(t), "me@example.com", goodPassword),
		e.do(t, c, http.MethodGet, "/api/auth/me", nil),
	} {
		expectStatus(t, "with store error", r, http.StatusInternalServerError)
		id := r.header.Get("X-Request-ID")
		expectBody(t, "with store error", r, `{"error":"internal error","request_id":"`+id+`"}`)
		if !strings.Contains(e.logs.String(), `"request_id":"`+id+`"`) {
			t.Errorf("no log line carries request_id %s:\n%s", id, e.logs)
		}
	}
	if !strings.Contains(e.logs.String(), "connection refused") {
		t.Errorf("logs do not contain the store error:\n%s", e.logs)
	}
}

func TestLogsHaveUserIDsButNoSecretsOrEmails(t *testing.T) {
	e := newTestEnv(t, envOptions{})
	c := e.client(t)
	reg := e.register(t, c, "me@example.com")
	token := sessionCookie(t, reg).Value
	e.do(t, c, http.MethodPost, "/api/auth/logout", nil)
	e.login(t, c, "nobody@example.com", "Wr0ngpass!")
	for range 3 {
		e.login(t, c, "me@example.com", "Wr0ngpass!")
	}
	e.login(t, c, "me@example.com", goodPassword)

	logs := e.logs.String()
	for _, secret := range []string{"me@example.com", "nobody@example.com", goodPassword, "Wr0ngpass!", token, e.store.PasswordHash("me@example.com")} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}
	for _, msg := range []string{"user registered", "logout", `"reason":"unknown_email"`, `"reason":"wrong_password"`, "account locked", `"reason":"locked"`, `"user_id":1`} {
		if !strings.Contains(logs, msg) {
			t.Errorf("logs do not contain %s:\n%s", msg, logs)
		}
	}
}
