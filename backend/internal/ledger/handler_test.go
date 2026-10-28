package ledger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/httpx/httpxtest"
)

func TestCategoriesForANewUserAreTheDefaults(t *testing.T) {
	h := NewHandler(NewService(newFakeStore(), fakeCurrencies{}, &fakeRates{}))

	rec := httpxtest.ServeAs(1, h.ListCategories, httptest.NewRequest(http.MethodGet, "/api/categories", nil))

	want := `{"categories":[` +
		`{"id":1,"kind":"expense","name":"飲食"},{"id":2,"kind":"expense","name":"生活用品"},` +
		`{"id":3,"kind":"expense","name":"美容美髮"},{"id":4,"kind":"expense","name":"其他"},` +
		`{"id":5,"kind":"income","name":"薪水"},{"id":6,"kind":"income","name":"獎金"},` +
		`{"id":7,"kind":"income","name":"股息"},{"id":8,"kind":"income","name":"其他"}]}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("GET /api/categories = %d %s, want 200 %s", rec.Code, rec.Body, want)
	}
}

func TestCategoriesStoreErrorIsAnInternalError(t *testing.T) {
	store := newFakeStore()
	store.err = errStore
	h := NewHandler(NewService(store, fakeCurrencies{}, &fakeRates{}))

	rec := httpxtest.ServeAs(1, h.ListCategories, httptest.NewRequest(http.MethodGet, "/api/categories", nil))

	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errStore.Error()) {
		t.Errorf("GET /api/categories = %d %s, want 500 without the internal error", rec.Code, rec.Body)
	}
}

func TestCategoriesWithoutAUserIsAnInternalError(t *testing.T) {
	h := NewHandler(NewService(newFakeStore(), fakeCurrencies{}, &fakeRates{}))

	rec := httpxtest.ServeAs(0, h.ListCategories, httptest.NewRequest(http.MethodGet, "/api/categories", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("GET /api/categories without a user = %d, want 500", rec.Code)
	}
}

type testAPI struct {
	h     *Handler
	store *fakeStore
	rates *fakeRates
}

func newTestAPI(t *testing.T) testAPI {
	t.Helper()
	svc, store, rates := newTestService(t)
	return testAPI{h: NewHandler(svc), store: store, rates: rates}
}

const lunchBody = `{"occurred_on":"2026-10-23","category_id":11,"currency":"TWD","amount":"120","note":"午餐"}`
const lunchJSON = `{"id":1,"occurred_on":"2026-10-23","kind":"expense","category":{"id":11,"name":"飲食"},"currency":"TWD","amount":"120.00","market_rate":null,"market_rate_date":null,"note":"午餐"}`

func TestCreateEntryResponds201WithTheEntry(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", lunchBody, api.h.CreateEntry)

	httpxtest.WantResponse(t, rec, http.StatusCreated, lunchJSON)
}

func TestCreateForeignEntryRespondsWithTheMarketRate(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-23", "32.50800000")

	rec := httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "",
		`{"occurred_on":"2026-10-24","category_id":15,"currency":"USD","amount":"1000"}`, api.h.CreateEntry)

	httpxtest.WantResponse(t, rec, http.StatusCreated,
		`{"id":1,"occurred_on":"2026-10-24","kind":"income","category":{"id":15,"name":"薪水"},"currency":"USD","amount":"1000.00","market_rate":"32.508","market_rate_date":"2026-10-23","note":""}`)
}

func TestCreateEntryRejectsInvalidInput(t *testing.T) {
	api := newTestAPI(t)

	for name, tc := range map[string]struct {
		body   string
		status int
		want   string
	}{
		"invalid fields": {
			`{"occurred_on":"2026-10-25","category_id":21,"currency":"XYZ","amount":"1e3"}`, http.StatusUnprocessableEntity,
			`{"error":"invalid input","fields":{"amount":"must be a number like 120 or 32.5","category_id":"unknown category","currency":"unknown currency","occurred_on":"must not be in the future"}}`,
		},
		"unknown field":    {`{"occurred_on":"2026-10-23","category_id":11,"currency":"TWD","amounts":"120"}`, http.StatusBadRequest, `{"error":"invalid json"}`},
		"amount as number": {`{"occurred_on":"2026-10-23","category_id":11,"currency":"TWD","amount":120}`, http.StatusBadRequest, `{"error":"invalid json"}`},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", tc.body, api.h.CreateEntry)
			httpxtest.WantResponse(t, rec, tc.status, tc.want)
			if len(api.store.entries) != 0 {
				t.Errorf("stored %d entries, want none", len(api.store.entries))
			}
		})
	}
}

func TestGetEntry(t *testing.T) {
	api := newTestAPI(t)
	httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", lunchBody, api.h.CreateEntry)

	httpxtest.WantResponse(t, httpxtest.Send(userA, http.MethodGet, "/api/ledger-entries/1", "1", "", api.h.GetEntry), http.StatusOK, lunchJSON)
	for name, tc := range map[string]struct {
		user int64
		id   string
	}{"another user's": {userB, "1"}, "missing": {userA, "2"}, "not a number": {userA, "abc"}} {
		rec := httpxtest.Send(tc.user, http.MethodGet, "/api/ledger-entries/"+tc.id, tc.id, "", api.h.GetEntry)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s entry: %d %s, want 404", name, rec.Code, rec.Body)
		}
	}
}

func TestUpdateEntry(t *testing.T) {
	api := newTestAPI(t)
	httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", lunchBody, api.h.CreateEntry)
	dinner := `{"occurred_on":"2026-10-22","category_id":11,"currency":"TWD","amount":"150.5","note":"晚餐"}`

	if rec := httpxtest.Send(userB, http.MethodPut, "/api/ledger-entries/1", "1", dinner, api.h.UpdateEntry); rec.Code != http.StatusNotFound {
		t.Errorf("B updates A's entry: %d %s, want 404", rec.Code, rec.Body)
	}
	if rec := httpxtest.Send(userA, http.MethodPut, "/api/ledger-entries/abc", "abc", dinner, api.h.UpdateEntry); rec.Code != http.StatusNotFound {
		t.Errorf("update id abc: %d %s, want 404", rec.Code, rec.Body)
	}
	rec := httpxtest.Send(userA, http.MethodPut, "/api/ledger-entries/1", "1", dinner, api.h.UpdateEntry)

	httpxtest.WantResponse(t, rec, http.StatusOK,
		`{"id":1,"occurred_on":"2026-10-22","kind":"expense","category":{"id":11,"name":"飲食"},"currency":"TWD","amount":"150.50","market_rate":null,"market_rate_date":null,"note":"晚餐"}`)
}

func TestDeleteEntry(t *testing.T) {
	api := newTestAPI(t)
	httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", lunchBody, api.h.CreateEntry)

	if rec := httpxtest.Send(userB, http.MethodDelete, "/api/ledger-entries/1", "1", "", api.h.DeleteEntry); rec.Code != http.StatusNotFound {
		t.Errorf("B deletes A's entry: %d %s, want 404", rec.Code, rec.Body)
	}
	rec := httpxtest.Send(userA, http.MethodDelete, "/api/ledger-entries/1", "1", "", api.h.DeleteEntry)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("delete = %d %q, want 204 with no body", rec.Code, rec.Body)
	}
	if rec := httpxtest.Send(userA, http.MethodGet, "/api/ledger-entries/1", "1", "", api.h.GetEntry); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}

func TestListEntries(t *testing.T) {
	api := newTestAPI(t)
	httpxtest.Send(userA, http.MethodPost, "/api/ledger-entries", "", lunchBody, api.h.CreateEntry)
	api.store.totals = []Total{
		{Currency: "TWD", MinorUnit: 2, Income: decimal.RequireFromString("50000"), Expense: decimal.RequireFromString("155.5")},
		{Currency: "USD", MinorUnit: 2, Income: decimal.RequireFromString("12.34"), Expense: decimal.Zero},
	}

	rec := httpxtest.Send(userA, http.MethodGet, "/api/ledger-entries?month=2026-10", "", "", api.h.ListEntries)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"month":"2026-10","entries":[`+lunchJSON+`],"totals":[`+
		`{"currency":"TWD","income":"50000.00","expense":"155.50","net":"49844.50"},`+
		`{"currency":"USD","income":"12.34","expense":"0.00","net":"12.34"}]}`)
}

func TestListEntriesForAnEmptyMonth(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/ledger-entries", "", "", api.h.ListEntries)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"month":"2026-10","entries":[],"totals":[]}`)
}

func TestListEntriesRejectsAMalformedMonth(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/ledger-entries?month=2026-13", "", "", api.h.ListEntries)

	httpxtest.WantResponse(t, rec, http.StatusUnprocessableEntity, `{"error":"invalid input","fields":{"month":"must be a month in YYYY-MM format"}}`)
}
