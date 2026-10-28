package ledger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/platform/httpx/httpxtest"
	"nest-egg/backend/internal/rate"
)

func TestUsersCannotSeeOrChangeEachOthersEntries(t *testing.T) {
	pool := dbtest.Migrated(t)
	svc := NewService(NewRepository(pool), currency.NewService(currency.NewRepository(pool)), rate.NewRepository(pool))
	h := NewHandler(svc)
	a, b := dbtest.InsertUser(t, pool, "a@example.com"), dbtest.InsertUser(t, pool, "b@example.com")
	firstCategoryID := func(user int64) int64 {
		t.Helper()
		rec := httpxtest.Send(user, http.MethodGet, "/api/categories", "", "", h.ListCategories)
		var resp struct{ Categories []struct{ ID int64 } }
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Categories) == 0 {
			t.Fatalf("categories for user %d: %d %s", user, rec.Code, rec.Body)
		}
		return resp.Categories[0].ID
	}
	aCategoryID, bCategoryID := firstCategoryID(a), firstCategoryID(b)

	created := httpxtest.Send(a, http.MethodPost, "/api/ledger-entries", "",
		fmt.Sprintf(`{"occurred_on":"2026-10-23","category_id":%d,"currency":"TWD","amount":"120","note":"午餐"}`, aCategoryID), h.CreateEntry)
	var entry struct{ ID int64 }
	if err := json.Unmarshal(created.Body.Bytes(), &entry); created.Code != http.StatusCreated || err != nil {
		t.Fatalf("A creates an entry: %d %s", created.Code, created.Body)
	}
	httpxtest.WantNoUserID(t, created)
	id := fmt.Sprint(entry.ID)

	httpxtest.WantResponse(t, httpxtest.Send(b, http.MethodGet, "/api/ledger-entries?month=2026-10", "", "", h.ListEntries),
		http.StatusOK, `{"month":"2026-10","entries":[],"totals":[]}`)

	bChange := fmt.Sprintf(`{"occurred_on":"2026-10-01","category_id":%d,"currency":"TWD","amount":"999"}`, bCategoryID)
	for _, attempt := range []struct {
		method, body string
		handler      httpx.Handler
	}{
		{http.MethodGet, "", h.GetEntry},
		{http.MethodPut, bChange, h.UpdateEntry},
		{http.MethodDelete, "", h.DeleteEntry},
	} {
		rec := httpxtest.Send(b, attempt.method, "/api/ledger-entries/"+id, id, attempt.body, attempt.handler)
		if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"error":"not found"}` {
			t.Errorf("B %s A's entry: %d %s, want 404 not found", attempt.method, rec.Code, rec.Body)
		}
	}
	if rec := httpxtest.Send(a, http.MethodGet, "/api/ledger-entries/"+id, id, "", h.GetEntry); rec.Body.String() != created.Body.String() {
		t.Errorf("A's entry after B's attempts = %s, want it unchanged: %s", rec.Body, created.Body)
	}

	stolenCategory := fmt.Sprintf(`{"occurred_on":"2026-10-23","category_id":%d,"currency":"TWD","amount":"1"}`, aCategoryID)
	rec := httpxtest.Send(b, http.MethodPost, "/api/ledger-entries", "", stolenCategory, h.CreateEntry)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"category_id"`) {
		t.Errorf("B creates an entry with A's category: %d %s, want 422 on category_id", rec.Code, rec.Body)
	}
}
