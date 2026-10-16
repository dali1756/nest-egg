package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestLimiter(burst int, every time.Duration) (*Limiter, *fakeClock) {
	clk := &fakeClock{now: time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)}
	l := New(burst, every, nil)
	l.now = clk.Now
	return l, clk
}

func TestAllowBurstThenRefill(t *testing.T) {
	l, clk := newTestLimiter(3, 10*time.Second)
	for i := 1; i <= 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d denied, want allowed within burst", i)
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait != 10*time.Second {
		t.Fatalf("4th request = %v, wait %v; want denied, wait 10s", ok, wait)
	}

	clk.Advance(4 * time.Second)
	if ok, wait := l.Allow("a"); ok || wait != 6*time.Second {
		t.Errorf("after 4s = %v, wait %v; want denied, wait 6s", ok, wait)
	}
	clk.Advance(6 * time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("after 10s denied, want one token refilled")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Error("second request after 10s allowed, want only one token refilled")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)
	l.Allow("a")
	if ok, _ := l.Allow("b"); !ok {
		t.Error("key b denied after key a used its token")
	}
}

func TestSweepDropsIdleBuckets(t *testing.T) {
	l, clk := newTestLimiter(2, time.Second)
	l.Allow("a")
	clk.Advance(time.Minute)
	l.Allow("b")
	if _, ok := l.tat["a"]; ok {
		t.Error("bucket a kept after it refilled, want swept")
	}
}

func TestTooManyKeysFailsClosed(t *testing.T) {
	l, _ := newTestLimiter(5, time.Second)
	l.maxKeys = 2
	l.Allow("a")
	l.Allow("b")
	if ok, _ := l.Allow("c"); ok {
		t.Error("new key allowed while the table is full, want denied")
	}
	if ok, _ := l.Allow("a"); !ok {
		t.Error("existing key denied while the table is full")
	}
}

func TestClientIP(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.1/32")}
	tests := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		want    string
	}{
		{"direct client", nil, "203.0.113.7:5000", nil, "203.0.113.7"},
		{"no proxies configured: header ignored", nil, "127.0.0.1:5000", []string{"198.51.100.1"}, "127.0.0.1"},
		{"no proxies configured: private peer", nil, "172.18.0.4:5000", []string{"198.51.100.1"}, "172.18.0.4"},
		{"untrusted peer cannot spoof", proxies, "203.0.113.7:5000", []string{"198.51.100.1"}, "203.0.113.7"},
		{"ipv6 grouped by /64", nil, "[2001:db8:1:2:aaaa::1]:5000", nil, "2001:db8:1:2::/64"},
		{"ipv4-mapped ipv6", nil, "[::ffff:203.0.113.7]:5000", nil, "203.0.113.7"},
		{"behind proxy", proxies, "10.0.0.2:5000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"spoofed entry left of the real client", proxies, "127.0.0.1:5000", []string{"1.1.1.1, 198.51.100.1"}, "198.51.100.1"},
		{"skips chained proxies", proxies, "10.0.0.2:5000", []string{"198.51.100.1", "10.0.0.3"}, "198.51.100.1"},
		{"client on an untrusted private network", proxies, "10.0.0.2:5000", []string{"192.168.1.10"}, "192.168.1.10"},
		{"only proxy hops", proxies, "10.0.0.2:5000", []string{"10.0.0.9"}, "10.0.0.2"},
		{"malformed header", proxies, "10.0.0.2:5000", []string{"not-an-ip"}, "10.0.0.2"},
		{"proxy without header", proxies, "10.0.0.2:5000", nil, "10.0.0.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := ClientIP(r, tt.trusted); got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTrustedProxies(t *testing.T) {
	got, err := ParseTrustedProxies(" 127.0.0.1, 10.1.2.3/8 ,::1,")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	want := []string{"127.0.0.1/32", "10.0.0.0/8", "::1/128"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("prefix %d = %s, want %s", i, got[i], want[i])
		}
	}
	if got, err := ParseTrustedProxies(""); err != nil || len(got) != 0 {
		t.Errorf("empty = %v, %v; want none", got, err)
	}
	if _, err := ParseTrustedProxies("10.0.0.0/8,localhost"); err == nil {
		t.Error("invalid entry: want error")
	}
}

func TestMiddlewareRejectsWith429(t *testing.T) {
	l, _ := newTestLimiter(1, 30*time.Second)
	calls := 0
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	serve := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))
		return rec
	}

	if rec := serve(); rec.Code != http.StatusNoContent {
		t.Fatalf("first request = %d, want 204", rec.Code)
	}
	rec := serve()
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "30" ||
		strings.TrimSpace(rec.Body.String()) != `{"error":"too many requests"}` {
		t.Errorf("second request = %d, Retry-After %q, body %s; want 429, 30, too many requests",
			rec.Code, rec.Header().Get("Retry-After"), rec.Body)
	}
	if calls != 1 {
		t.Errorf("next handler called %d times, want 1", calls)
	}
}
