package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/auth/authtest"
	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/config"
	"nest-egg/backend/internal/platform/health"
	"nest-egg/backend/internal/platform/ratelimit"
	"nest-egg/backend/internal/rate"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

// newTestHandler 只有一個使用者 me@example.com，密碼 Passw0rd!。
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newTestHandlerWithUsers(t, "me@example.com")
}

func newTestHandlerWithUsers(t *testing.T, emails ...string) http.Handler {
	t.Helper()
	public, protected := newRouter(newTestHandlers(t, config.Config{}, nil, emails...))
	testRoutes(protected)
	return withMiddleware(public, config.Config{})
}

// newTestHandlers 的使用者密碼都是 Passw0rd!；幣別與匯率用假的資料；limit 為 nil 時用正式的次數限制。
func newTestHandlers(t *testing.T, cfg config.Config, limit func(http.Handler) http.Handler, emails ...string) handlers {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := authtest.NewMemStore()
	for _, email := range emails {
		if _, err := store.CreateUser(t.Context(), email, string(hash)); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.NewService(store, auth.Config{BcryptCost: bcrypt.MinCost}, log)
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}
	if limit == nil {
		limit = newRateLimit(cfg)
	}
	currencies := currency.NewService(fakeCurrencyStore{})
	return handlers{
		health:    health.NewHandler(okPinger{}),
		auth:      auth.NewHandler(svc, auth.HandlerConfig{CookieSecure: cfg.CookieSecure}, log),
		currency:  currency.NewHandler(currencies),
		rate:      rate.NewHandler(rate.NewService(fakeRateStore{}, nil, currencies)),
		rateLimit: limit,
	}
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

func TestCredentialEndpointsAreRateLimited(t *testing.T) {
	h := newTestHandler(t)
	for i := 1; i <= 10; i++ {
		if rec := serve(h, loginRequest("Wr0ngpass!")); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("login %d = 429, want allowed within the first 10", i)
		}
	}
	rec := serve(h, loginRequest("Passw0rd!"))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("11th login = %d, Retry-After %q; want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}

	if rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/health", nil)); rec.Code != http.StatusOK {
		t.Errorf("health while limited = %d, want 200 (not rate limited)", rec.Code)
	}
}

func TestSpoofedForwardedForDoesNotBypassTheLimit(t *testing.T) {
	h := newTestHandler(t)
	for i := 1; i <= 10; i++ {
		req := loginRequest("Wr0ngpass!")
		req.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(i))
		serve(h, req)
	}
	req := loginRequest("Wr0ngpass!")
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	if rec := serve(h, req); rec.Code != http.StatusTooManyRequests {
		t.Errorf("11th login with a new X-Forwarded-For = %d, want 429 (header must be ignored without TRUSTED_PROXIES)", rec.Code)
	}
}

func TestTrustedProxyForwardedForIsUsed(t *testing.T) {
	cfg := config.Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")}}
	h := newHandler(newTestHandlers(t, cfg, nil), cfg)

	for i := 1; i <= 11; i++ {
		req := loginRequest("Wr0ngpass!")
		req.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(i))
		if rec := serve(h, req); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("login %d from a different client behind the proxy = 429, want each client counted separately", i)
		}
	}
}

func testRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/example", func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.CurrentUser(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(u.Email))
	})
	mux.HandleFunc("GET /api/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
}

func loginCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := serve(h, loginRequest("Passw0rd!"))
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("login = %d %s, want 200 with a session cookie", rec.Code, rec.Body)
	}
	return rec.Result().Cookies()[0]
}

func get(path string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func TestProtectedRoutesRequireLogin(t *testing.T) {
	h := newTestHandler(t)

	anon := serve(h, get("/api/example", nil))
	if anon.Code != http.StatusUnauthorized || strings.TrimSpace(anon.Body.String()) != `{"error":"unauthorized"}` {
		t.Errorf("anonymous /api/example = %d %s, want 401 {\"error\":\"unauthorized\"}", anon.Code, anon.Body)
	}

	authed := serve(h, get("/api/example", loginCookie(t, h)))
	if authed.Code != http.StatusOK || authed.Body.String() != "me@example.com" {
		t.Errorf("logged-in /api/example = %d %q, want 200 me@example.com", authed.Code, authed.Body)
	}
}

func TestUnknownAPIPathsHideBehindLogin(t *testing.T) {
	h := newTestHandler(t)
	if rec := serve(h, get("/api/does-not-exist", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous unknown path = %d, want 401", rec.Code)
	}
	if rec := serve(h, get("/api/does-not-exist", loginCookie(t, h))); rec.Code != http.StatusNotFound {
		t.Errorf("logged-in unknown path = %d, want 404", rec.Code)
	}
}

func TestEveryResponseHasARequestID(t *testing.T) {
	h := newTestHandler(t)
	crossSite := loginRequest("Passw0rd!")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	for i := 1; i <= 10; i++ {
		serve(h, loginRequest("Wr0ngpass!"))
	}

	for name, tc := range map[string]struct {
		req    *http.Request
		status int
	}{
		"health":       {httptest.NewRequest(http.MethodGet, "/api/health", nil), http.StatusOK},
		"cross-site":   {crossSite, http.StatusForbidden},
		"rate limited": {loginRequest("Wr0ngpass!"), http.StatusTooManyRequests},
		"anonymous":    {get("/api/example", nil), http.StatusUnauthorized},
	} {
		rec := serve(h, tc.req)
		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.status)
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Errorf("%s: no X-Request-ID header", name)
		}
	}
}

func TestPanicIsRecovered(t *testing.T) {
	var logs bytes.Buffer
	captureDefaultLogger(t, newLogger(&logs, config.Config{LogFormat: "json"}))
	h := newTestHandler(t)

	rec := serve(h, get("/api/panic", loginCookie(t, h)))
	id := rec.Header().Get("X-Request-ID")
	if want := `{"error":"internal error","request_id":"` + id + `"}`; rec.Code != http.StatusInternalServerError || strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("panic = %d %s, want 500 %s", rec.Code, rec.Body, want)
	}
	if id == "" || rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("panic response headers %v, want X-Request-ID and security headers", rec.Header())
	}

	type logLine struct {
		Msg       string `json:"msg"`
		RequestID string `json:"request_id"`
		Status    int    `json:"status"`
	}
	var panicLine, accessLine *logLine
	for _, raw := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var l logLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("parse log line %q: %v", raw, err)
		}
		if l.RequestID != id {
			continue
		}
		switch l.Msg {
		case "panic recovered":
			panicLine = &l
		case "request":
			accessLine = &l
		}
	}
	if panicLine == nil {
		t.Errorf("no panic log with request_id %s:\n%s", id, logs.String())
	}
	if accessLine == nil || accessLine.Status != http.StatusInternalServerError {
		t.Errorf("access log for request %s = %+v, want status 500", id, accessLine)
	}

	if rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/health", nil)); rec.Code != http.StatusOK {
		t.Errorf("request after panic = %d, want 200", rec.Code)
	}
}

func TestAccessLogIgnoresSpoofedForwardedFor(t *testing.T) {
	var logs bytes.Buffer
	captureDefaultLogger(t, newLogger(&logs, config.Config{LogFormat: "json"}))
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "203.0.113.7:51000"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	serve(h, req)

	if !strings.Contains(logs.String(), `"client_ip":"203.0.113.7"`) || strings.Contains(logs.String(), "198.51.100.1") {
		t.Errorf("access log %s, want client_ip 203.0.113.7 and no forwarded address", logs.String())
	}
}

func captureDefaultLogger(t *testing.T, l *slog.Logger) {
	t.Helper()
	old := slog.Default()
	slog.SetDefault(l)
	t.Cleanup(func() { slog.SetDefault(old) })
}

func TestIdentityComesOnlyFromTheSession(t *testing.T) {
	h := newTestHandlerWithUsers(t, "me@example.com", "other@example.com")
	cookie := loginCookie(t, h)

	for path, want := range map[string]string{
		"/api/auth/me?user_id=2": `{"email":"me@example.com"}`,
		"/api/example?user_id=2": `me@example.com`,
	} {
		req := get(path, cookie)
		req.Header.Set("X-User-ID", "2")
		rec := serve(h, req)
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
			t.Errorf("%s as A = %d %s, want 200 %s", path, rec.Code, rec.Body, want)
		}
	}
}

type fakeCurrencyStore struct{}

func (fakeCurrencyStore) List(context.Context) ([]currency.Currency, error) {
	return []currency.Currency{{Code: "TWD", Name: "新臺幣", MinorUnit: 2}, {Code: "USD", Name: "美元", MinorUnit: 2}}, nil
}

func (fakeCurrencyStore) Exists(_ context.Context, code string) (bool, error) {
	return code == "TWD" || code == "USD", nil
}

type fakeRateStore struct{}

func (fakeRateStore) Upsert(context.Context, string, []rate.Rate) (rate.UpsertResult, error) {
	return rate.UpsertResult{}, nil
}

func (fakeRateStore) On(_ context.Context, base, quote string, date time.Time) (rate.Snapshot, error) {
	return rate.Snapshot{Rate: rate.Rate{Base: base, Quote: quote, Date: date, Value: decimal.RequireFromString("32.445")}, Source: "cbc"}, nil
}

func loginCookieAs(t *testing.T, h http.Handler, email string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"`+email+`","password":"Passw0rd!"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("login as %s = %d %s, want 200 with a session cookie", email, rec.Code, rec.Body)
	}
	return rec.Result().Cookies()[0]
}

// 幣別與匯率是共用資料：每個使用者看到的內容相同，回應不含 user_id。
func TestSharedDataIsTheSameForEveryUser(t *testing.T) {
	h := newAppHandler(t, nil, "a@example.com", "b@example.com")
	a, b := loginCookieAs(t, h, "a@example.com"), loginCookieAs(t, h, "b@example.com")

	for _, path := range []string{"/api/currencies", "/api/rates?base=USD&quote=TWD&date=2026-10-17"} {
		ra, rb := serve(h, get(path, a)), serve(h, get(path, b))
		if ra.Code != http.StatusOK || rb.Code != http.StatusOK || ra.Body.String() != rb.Body.String() {
			t.Errorf("%s: A got %d %s, B got %d %s; want the same 200 response", path, ra.Code, ra.Body, rb.Code, rb.Body)
		}
		if strings.Contains(ra.Body.String(), "user_id") {
			t.Errorf("%s: response %s contains user_id", path, ra.Body)
		}
	}
}

// newAppHandler 是正式的路徑表與 middleware；limit 為 nil 時用正式的次數限制。
func newAppHandler(t *testing.T, limit func(http.Handler) http.Handler, emails ...string) http.Handler {
	t.Helper()
	return newHandler(newTestHandlers(t, config.Config{}, limit, emails...), config.Config{})
}

func request(method, path string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// apiRoutes 是 routes.go 應該有的每一條路徑；public 以外都要登入。
var apiRoutes = []struct {
	method, path string
	public       bool
}{
	{http.MethodGet, "/api/health", true},
	{http.MethodPost, "/api/auth/register", true},
	{http.MethodPost, "/api/auth/login", true},
	{http.MethodPost, "/api/auth/logout", true},
	{http.MethodPost, "/api/auth/password/forgot", true},
	{http.MethodPost, "/api/auth/password/reset", true},
	{http.MethodGet, "/api/auth/me", false},
	{http.MethodPost, "/api/auth/password/change", false},
	{http.MethodGet, "/api/currencies", false},
	{http.MethodGet, "/api/rates?base=USD&quote=TWD", false},
}

func TestEveryRouteHasTheRightProtection(t *testing.T) {
	h := newAppHandler(t, nil, "me@example.com")
	cookie := loginCookie(t, h)
	notRouted := func(code int) bool { return code == http.StatusNotFound || code == http.StatusMethodNotAllowed }

	for _, rt := range apiRoutes {
		name := rt.method + " " + rt.path
		anon := serve(h, request(rt.method, rt.path, nil))
		if rt.public {
			if anon.Code == http.StatusUnauthorized || notRouted(anon.Code) {
				t.Errorf("anonymous %s = %d %s, want a public route", name, anon.Code, anon.Body)
			}
			continue
		}
		if anon.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s = %d, want 401", name, anon.Code)
		}
		if authed := serve(h, request(rt.method, rt.path, cookie)); notRouted(authed.Code) {
			t.Errorf("logged-in %s = %d, want the route to exist", name, authed.Code)
		}
	}
}

func TestOnlyCredentialEndpointsAreRateLimited(t *testing.T) {
	h := newAppHandler(t, ratelimit.New(1, time.Hour, nil).Middleware, "me@example.com")
	cookie := loginCookie(t, h) // 用掉唯一的一次

	for _, rt := range []struct {
		method, path string
		cookie       *http.Cookie
	}{
		{http.MethodPost, "/api/auth/register", nil},
		{http.MethodPost, "/api/auth/login", nil},
		{http.MethodPost, "/api/auth/password/forgot", nil},
		{http.MethodPost, "/api/auth/password/reset", nil},
		{http.MethodPost, "/api/auth/password/change", cookie},
	} {
		if rec := serve(h, request(rt.method, rt.path, rt.cookie)); rec.Code != http.StatusTooManyRequests {
			t.Errorf("%s %s = %d, want 429", rt.method, rt.path, rec.Code)
		}
	}
	for _, rt := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/health", http.StatusOK},
		{http.MethodGet, "/api/auth/me", http.StatusOK},
		{http.MethodGet, "/api/currencies", http.StatusOK},
		{http.MethodPost, "/api/auth/logout", http.StatusNoContent},
	} {
		if rec := serve(h, request(rt.method, rt.path, cookie)); rec.Code != rt.want {
			t.Errorf("%s %s = %d, want %d (not rate limited)", rt.method, rt.path, rec.Code, rt.want)
		}
	}
}

// 變更密碼要先登入：沒登入的請求在檢查登入時就被擋下，不消耗這個 IP 的次數。
func TestAnonymousPasswordChangeDoesNotUseTheRateLimit(t *testing.T) {
	h := newAppHandler(t, ratelimit.New(1, time.Hour, nil).Middleware, "me@example.com")

	if rec := serve(h, request(http.MethodPost, "/api/auth/password/change", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous password change = %d, want 401", rec.Code)
	}
	if rec := serve(h, loginRequest("Passw0rd!")); rec.Code != http.StatusOK {
		t.Errorf("login after an anonymous password change = %d, want 200 (the token was not used)", rec.Code)
	}
}
