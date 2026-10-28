package fx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/ledger"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/platform/httpx/httpxtest"
	"nest-egg/backend/internal/rate"
)

func TestUsersCannotSeeOrChangeEachOthersTrades(t *testing.T) {
	pool := dbtest.Migrated(t)
	svc := NewService(NewRepository(pool), currency.NewService(currency.NewRepository(pool)), rate.NewRepository(pool))
	h := NewHandler(svc)
	a, b := dbtest.InsertUser(t, pool, "a@example.com"), dbtest.InsertUser(t, pool, "b@example.com")

	created := httpxtest.Send(a, http.MethodPost, "/api/fx-trades", "", buyBody, h.CreateTrade)
	var trade struct{ ID int64 }
	if err := json.Unmarshal(created.Body.Bytes(), &trade); created.Code != http.StatusCreated || err != nil {
		t.Fatalf("A creates a trade: %d %s", created.Code, created.Body)
	}
	httpxtest.WantNoUserID(t, created)
	id := fmt.Sprint(trade.ID)

	httpxtest.WantResponse(t, httpxtest.Send(b, http.MethodGet, "/api/fx-trades?month=2026-10", "", "", h.ListTrades),
		http.StatusOK, `{"month":"2026-10","trades":[]}`)

	bChange := `{"traded_on":"2026-10-01","side":"sell","currency":"USD","foreign_amount":"1","twd_amount":"1"}`
	for _, attempt := range []struct {
		method, path, id, body string
		handler                httpx.Handler
	}{
		{http.MethodGet, "/api/fx-trades/" + id, id, "", h.GetTrade},
		{http.MethodPut, "/api/fx-trades/" + id, id, bChange, h.UpdateTrade},
		{http.MethodDelete, "/api/fx-trades/" + id, id, "", h.DeleteTrade},
		{http.MethodGet, "/api/fx-trades/preview?id=" + id + "&" + previewQuery, "", "", h.PreviewTrade},
	} {
		rec := httpxtest.Send(b, attempt.method, attempt.path, attempt.id, attempt.body, attempt.handler)
		if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"error":"not found"}` {
			t.Errorf("B %s %s: %d %s, want 404 not found", attempt.method, attempt.path, rec.Code, rec.Body)
		}
	}
	if rec := httpxtest.Send(a, http.MethodGet, "/api/fx-trades/"+id, id, "", h.GetTrade); rec.Body.String() != created.Body.String() {
		t.Errorf("A's trade after B's attempts = %s, want it unchanged: %s", rec.Body, created.Body)
	}
}

func TestTradesAreNotLedgerEntries(t *testing.T) {
	pool := dbtest.Migrated(t)
	currencies := currency.NewService(currency.NewRepository(pool))
	trades := NewService(NewRepository(pool), currencies, rate.NewRepository(pool))
	entries := ledger.NewService(ledger.NewRepository(pool), currencies, rate.NewRepository(pool))
	a := dbtest.InsertUser(t, pool, "a@example.com")
	if _, err := trades.CreateTrade(t.Context(), a, buyInput()); err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	month, err := entries.Month(t.Context(), a, "2026-10")

	if err != nil || len(month.Entries) != 0 || len(month.Totals) != 0 {
		t.Errorf("ledger month = %+v, %v; want no entries or totals", month, err)
	}
}
