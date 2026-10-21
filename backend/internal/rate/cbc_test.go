package rate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func cbcServer(t *testing.T, h http.HandlerFunc) *CBCProvider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewCBCProvider(srv.URL)
}

func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCBCProviderParsesTheResponse(t *testing.T) {
	p := cbcServer(t, respondWith(http.StatusOK, testdata(t, "cbc_day.json")))
	if p.Name() != "cbc" {
		t.Errorf("Name = %q, want cbc", p.Name())
	}

	got, err := p.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	want := []Rate{
		usdTWD(day(2008, 10, 31), "33"),
		usdTWD(day(2026, 10, 14), "32.073"),
		usdTWD(day(2026, 10, 15), "32.265"),
		usdTWD(day(2026, 10, 16), "32.338"),
		usdTWD(day(2026, 10, 19), "32.445"),
	}
	if len(got) != len(want) {
		t.Fatalf("Fetch returned %d rates, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Base != w.Base || g.Quote != w.Quote || !g.Date.Equal(w.Date) || !g.Value.Equal(w.Value) {
			t.Errorf("rate[%d] = %s %s/%s %s, want %s %s/%s %s", i,
				g.Date.Format("2006-01-02"), g.Base, g.Quote, g.Value, w.Date.Format("2006-01-02"), w.Base, w.Quote, w.Value)
		}
	}
}

func TestCBCProviderSendsUserAgent(t *testing.T) {
	var ua string
	p := cbcServer(t, func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`[{"日期":"20261016","NTD_USD":"32.338"}]`))
	})
	if _, err := p.Fetch(t.Context()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if ua != "nest-egg" {
		t.Errorf("User-Agent = %q, want nest-egg", ua)
	}
}

func TestCBCProviderRejectsBadResponses(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"server error":     respondWith(http.StatusInternalServerError, `[{"日期":"20261016","NTD_USD":"32.338"}]`),
		"challenge page":   respondWith(http.StatusOK, testdata(t, "challenge.html")),
		"not an array":     respondWith(http.StatusOK, `{"日期":"20261016","NTD_USD":"32.338"}`),
		"bad date":         respondWith(http.StatusOK, `[{"日期":"2026-10-16","NTD_USD":"32.338"}]`),
		"bad number":       respondWith(http.StatusOK, `[{"日期":"20261016","NTD_USD":"--"}]`),
		"trailing garbage": respondWith(http.StatusOK, `[{"日期":"20261016","NTD_USD":"32.338"}] x`),
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := cbcServer(t, h).Fetch(t.Context())
			if err == nil {
				t.Errorf("Fetch = %d rates, want error", len(got))
			}
		})
	}
}

func TestCBCProviderRejectsBodiesLargerThan5MB(t *testing.T) {
	tooLarge := "[" + strings.Repeat(`{"日期":"20261016","NTD_USD":"32.338"},`, 150_000)
	_, err := cbcServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tooLarge))
		<-r.Context().Done()
	}).Fetch(t.Context())
	if err == nil || !strings.Contains(err.Error(), "larger than 5 MB") {
		t.Errorf("Fetch error = %v, want one saying the body is larger than 5 MB", err)
	}
}
