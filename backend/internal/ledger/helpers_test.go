package ledger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/rate"
)

func taipeiDate(s string) time.Time {
	d, err := taipei.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func usdSnapshot(date, value string) *rate.Snapshot {
	return &rate.Snapshot{
		Rate:   rate.Rate{Base: "USD", Quote: "TWD", Date: taipeiDate(date), Value: decimal.RequireFromString(value)},
		Source: "cbc",
	}
}

func wantMarketRate(t *testing.T, got *MarketRate, value, date string) {
	t.Helper()
	want := &MarketRate{Value: decimal.RequireFromString(value), Date: taipeiDate(date)}
	if !sameMarketRate(got, want) {
		t.Errorf("market rate = %+v, want %s on %s", got, value, date)
	}
}

func sameMarketRate(a, b *MarketRate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Value.Equal(b.Value) && a.Date.Equal(b.Date)
}

func wantEntry(t *testing.T, got, want Entry) {
	t.Helper()
	if !got.OccurredOn.Equal(want.OccurredOn) || got.OccurredOn.Location() != taipei.Location {
		t.Errorf("occurred on %v, want %v in Asia/Taipei", got.OccurredOn, want.OccurredOn)
	}
	if got.Category != want.Category {
		t.Errorf("category = %+v, want %+v", got.Category, want.Category)
	}
	if got.Currency != want.Currency || got.MinorUnit != want.MinorUnit {
		t.Errorf("currency = %s with %d decimals, want %s with %d", got.Currency, got.MinorUnit, want.Currency, want.MinorUnit)
	}
	if !got.Amount.Equal(want.Amount) {
		t.Errorf("amount = %s, want %s", got.Amount, want.Amount)
	}
	if !sameMarketRate(got.MarketRate, want.MarketRate) {
		t.Errorf("market rate = %+v, want %+v", got.MarketRate, want.MarketRate)
	}
	if got.Note != want.Note {
		t.Errorf("note = %q, want %q", got.Note, want.Note)
	}
}

func serveAs(user int64, h httpx.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	if user != 0 {
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: user, Email: "me@example.com"}))
	}
	h.ServeHTTP(rec, req)
	return rec
}

func send(user int64, method, path, id, body string, h httpx.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if id != "" {
		req.SetPathValue("id", id)
	}
	return serveAs(user, h, req)
}

func wantNoUserID(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(rec.Body.String(), "user_id") {
		t.Errorf("response %s contains user_id", rec.Body)
	}
}

func wantResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status || strings.TrimSpace(rec.Body.String()) != body {
		t.Errorf("response = %d %s\nwant       %d %s", rec.Code, strings.TrimSpace(rec.Body.String()), status, body)
	}
	wantNoUserID(t, rec)
}
