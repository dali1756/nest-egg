package rate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/httpx"
)

func getRate(store *fakeStore, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	httpx.Handler(NewHandler(newTestService(store, nil, evening)).Get).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/rates"+query, nil))
	return rec
}

func TestGetRateOnAWeekend(t *testing.T) {
	// NUMERIC(20,8) 讀回來帶著多餘的 0
	snap := Snapshot{Rate: Rate{Base: "USD", Quote: "TWD", Date: day(2026, 10, 16), Value: decimal.RequireFromString("32.33800000")}, Source: "cbc"}
	store := &fakeStore{snapshot: snap}
	rec := getRate(store, "?base=USD&quote=TWD&date=2026-10-17")

	want := `{"base":"USD","quote":"TWD","rate":"32.338","rate_date":"2026-10-16","source":"cbc"}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("response = %d %s, want 200 %s", rec.Code, rec.Body, want)
	}
	if !slices.Equal(store.onArgs, []string{"USD/TWD 2026-10-17"}) {
		t.Errorf("store.On calls = %v, want [USD/TWD 2026-10-17]", store.onArgs)
	}
}

func TestGetRateWithoutDateUsesToday(t *testing.T) {
	store := &fakeStore{snapshot: Snapshot{Rate: usdTWD(day(2026, 10, 19), "32.445"), Source: "cbc"}}
	if rec := getRate(store, "?base=USD&quote=TWD"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s, want 200", rec.Code, rec.Body)
	}
	if !slices.Equal(store.onArgs, []string{"USD/TWD 2026-10-19"}) {
		t.Errorf("store.On calls = %v, want [USD/TWD 2026-10-19]", store.onArgs)
	}
}

func TestGetRateRejectsBadQueries(t *testing.T) {
	tests := map[string]string{
		"?base=XYZ&quote=TWD":                 "base",
		"?quote=TWD":                          "base",
		"?base=USD&quote=TWD&date=2026-10-20": "date",
		"?base=USD&quote=TWD&date=2026/10/17": "date",
	}
	for query, field := range tests {
		rec := getRate(&fakeStore{}, query)
		var body struct {
			Fields map[string]string `json:"fields"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusUnprocessableEntity || body.Fields[field] == "" {
			t.Errorf("GET /api/rates%s = %d %s, want 422 with fields.%s", query, rec.Code, rec.Body, field)
		}
	}
}

func TestGetRateNotFound(t *testing.T) {
	rec := getRate(&fakeStore{err: fmt.Errorf("rate: %w", apperr.ErrNotFound)}, "?base=TWD&quote=USD")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d %s, want 404", rec.Code, rec.Body)
	}
}
