package ledger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/rate"
)

func TestUsersCannotSeeOrChangeEachOthersEntries(t *testing.T) {
	pool := dbtest.Migrated(t)
	svc := NewService(NewRepository(pool), currency.NewService(currency.NewRepository(pool)), rate.NewRepository(pool))
	h := NewHandler(svc)
	a, b := insertUser(t, pool, "a@example.com"), insertUser(t, pool, "b@example.com")

	call := func(user int64, method, path, id, body string, handler httpx.Handler) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if id != "" {
			req.SetPathValue("id", id)
		}
		rec := serveAs(user, handler, req)
		if strings.Contains(rec.Body.String(), "user_id") {
			t.Errorf("%s %s as user %d: response %s contains user_id", method, path, user, rec.Body)
		}
		return rec
	}
	firstCategory := func(user int64) int64 {
		t.Helper()
		rec := call(user, http.MethodGet, "/api/categories", "", "", h.Categories)
		var resp struct{ Categories []struct{ ID int64 } }
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Categories) == 0 {
			t.Fatalf("categories for user %d: %d %s", user, rec.Code, rec.Body)
		}
		return resp.Categories[0].ID
	}
	aFood, bFood := firstCategory(a), firstCategory(b)

	created := call(a, http.MethodPost, "/api/ledger-entries", "",
		fmt.Sprintf(`{"occurred_on":"2026-10-23","category_id":%d,"currency":"TWD","amount":"120","note":"午餐"}`, aFood), h.CreateEntry)
	var entry struct{ ID int64 }
	if err := json.Unmarshal(created.Body.Bytes(), &entry); created.Code != http.StatusCreated || err != nil {
		t.Fatalf("A creates an entry: %d %s", created.Code, created.Body)
	}
	id := fmt.Sprint(entry.ID)

	if rec := call(b, http.MethodGet, "/api/ledger-entries?month=2026-10", "", "", h.ListEntries); strings.TrimSpace(rec.Body.String()) != `{"month":"2026-10","entries":[],"totals":[]}` {
		t.Errorf("B lists October: %d %s, want no entries and no totals", rec.Code, rec.Body)
	}
	bChange := fmt.Sprintf(`{"occurred_on":"2026-10-01","category_id":%d,"currency":"TWD","amount":"999"}`, bFood)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"GET":    call(b, http.MethodGet, "/api/ledger-entries/"+id, id, "", h.GetEntry),
		"PUT":    call(b, http.MethodPut, "/api/ledger-entries/"+id, id, bChange, h.UpdateEntry),
		"DELETE": call(b, http.MethodDelete, "/api/ledger-entries/"+id, id, "", h.DeleteEntry),
	} {
		if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"error":"not found"}` {
			t.Errorf("B %s A's entry: %d %s, want 404 not found", name, rec.Code, rec.Body)
		}
	}
	if rec := call(a, http.MethodGet, "/api/ledger-entries/"+id, id, "", h.GetEntry); rec.Body.String() != created.Body.String() {
		t.Errorf("A's entry after B's attempts = %s, want it unchanged: %s", rec.Body, created.Body)
	}

	stolenCategory := fmt.Sprintf(`{"occurred_on":"2026-10-23","category_id":%d,"currency":"TWD","amount":"1"}`, aFood)
	if rec := call(b, http.MethodPost, "/api/ledger-entries", "", stolenCategory, h.CreateEntry); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"category_id"`) {
		t.Errorf("B creates an entry with A's category: %d %s, want 422 on category_id", rec.Code, rec.Body)
	}
}
