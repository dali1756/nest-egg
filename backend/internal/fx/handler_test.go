package fx

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"nest-egg/backend/internal/platform/httpx/httpxtest"
)

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

const (
	buyBody = `{"traded_on":"2026-10-27","side":"buy","currency":"USD","foreign_amount":"1000","twd_amount":"32420","note":"10 月定期換匯"}`
	buyJSON = `{"id":1,"traded_on":"2026-10-27","side":"buy","currency":"USD","foreign_amount":"1000.00","twd_amount":"32420.00",` +
		`"trade_rate":"32.42","market_rate":"32.3","market_rate_date":"2026-10-27","spread":"120.00","note":"10 月定期換匯"}`
	previewQuery = "side=buy&currency=USD&traded_on=2026-10-27&foreign_amount=1000&twd_amount=32420"
)

func (api testAPI) createBuy(t *testing.T) {
	t.Helper()
	if rec := httpxtest.Send(userA, http.MethodPost, "/api/fx-trades", "", buyBody, api.h.CreateTrade); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
}

func wantInvalidFields(t *testing.T, rec *httptest.ResponseRecorder, want ...string) {
	t.Helper()
	var body struct {
		Fields map[string]string `json:"fields"`
	}
	if rec.Code != http.StatusUnprocessableEntity || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("response = %d %s, want 422", rec.Code, rec.Body)
	}
	if got := slices.Sorted(maps.Keys(body.Fields)); !slices.Equal(got, want) {
		t.Errorf("invalid fields = %v, want %v", got, want)
	}
}

func TestCreateTradeResponds201WithTheComputedRates(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.30000000")

	rec := httpxtest.Send(userA, http.MethodPost, "/api/fx-trades", "", buyBody, api.h.CreateTrade)

	httpxtest.WantResponse(t, rec, http.StatusCreated, buyJSON)
}

func TestCreateTradeWithoutAMarketRateRespondsWithNulls(t *testing.T) {
	api := newTestAPI(t)
	body := `{"traded_on":"2026-10-15","side":"sell","currency":"USD","foreign_amount":"500","twd_amount":"16050"}`

	rec := httpxtest.Send(userA, http.MethodPost, "/api/fx-trades", "", body, api.h.CreateTrade)

	httpxtest.WantResponse(t, rec, http.StatusCreated,
		`{"id":1,"traded_on":"2026-10-15","side":"sell","currency":"USD","foreign_amount":"500.00","twd_amount":"16050.00",`+
			`"trade_rate":"32.1","market_rate":null,"market_rate_date":null,"spread":null,"note":""}`)
}

func TestCreateTradeRejectsARateOrFee(t *testing.T) {
	for name, body := range map[string]string{
		"trade rate": `{"traded_on":"2026-10-27","side":"buy","currency":"USD","foreign_amount":"1000","twd_amount":"32420","trade_rate":"32.42"}`,
		"fee":        `{"traded_on":"2026-10-27","side":"buy","currency":"USD","foreign_amount":"1000","twd_amount":"32420","fee":"0"}`,
	} {
		api := newTestAPI(t)

		rec := httpxtest.Send(userA, http.MethodPost, "/api/fx-trades", "", body, api.h.CreateTrade)

		if rec.Code != http.StatusBadRequest || api.store.writes != 0 {
			t.Errorf("%s: response = %d %s with %d writes, want 400 and nothing stored", name, rec.Code, rec.Body, api.store.writes)
		}
	}
}

func TestCreateTradeRespondsWithTheInvalidFields(t *testing.T) {
	api := newTestAPI(t)
	body := `{"traded_on":"2026-10-29","side":"swap","currency":"TWD","foreign_amount":"0","twd_amount":"1.234","note":""}`

	rec := httpxtest.Send(userA, http.MethodPost, "/api/fx-trades", "", body, api.h.CreateTrade)

	wantInvalidFields(t, rec, "currency", "foreign_amount", "side", "traded_on", "twd_amount")
}

func TestListTradesRespondsWithTheMonth(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.3")
	api.createBuy(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades?month=2026-10", "", "", api.h.ListTrades)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"month":"2026-10","trades":[`+buyJSON+`]}`)
}

func TestListTradesOfAnEmptyMonthIsAnEmptyList(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades?month=2026-09", "", "", api.h.ListTrades)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"month":"2026-09","trades":[]}`)
}

func TestListTradesRejectsAMalformedMonth(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades?month=2026-13", "", "", api.h.ListTrades)

	wantInvalidFields(t, rec, "month")
}

func TestGetTrade(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.3")
	api.createBuy(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/1", "1", "", api.h.GetTrade)

	httpxtest.WantResponse(t, rec, http.StatusOK, buyJSON)
}

func TestGetTradeThatIsMissingOrNotOwnIsNotFound(t *testing.T) {
	api := newTestAPI(t)
	api.createBuy(t)

	for name, tc := range map[string]struct {
		user int64
		id   string
	}{
		"another user's": {userB, "1"},
		"not a number":   {userA, "abc"},
		"missing":        {userA, "999"},
	} {
		if rec := httpxtest.Send(tc.user, http.MethodGet, "/api/fx-trades/"+tc.id, tc.id, "", api.h.GetTrade); rec.Code != http.StatusNotFound {
			t.Errorf("%s: GET = %d, want 404", name, rec.Code)
		}
	}
}

func TestUpdateTradeRespondsWithTheChangedTrade(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.3")
	api.createBuy(t)
	body := `{"traded_on":"2026-10-27","side":"sell","currency":"USD","foreign_amount":"1000","twd_amount":"32200","note":""}`

	rec := httpxtest.Send(userA, http.MethodPut, "/api/fx-trades/1", "1", body, api.h.UpdateTrade)

	httpxtest.WantResponse(t, rec, http.StatusOK,
		`{"id":1,"traded_on":"2026-10-27","side":"sell","currency":"USD","foreign_amount":"1000.00","twd_amount":"32200.00",`+
			`"trade_rate":"32.2","market_rate":"32.3","market_rate_date":"2026-10-27","spread":"100.00","note":""}`)
}

func TestPutTradeOfAnotherUserIsNotFound(t *testing.T) {
	api := newTestAPI(t)
	api.createBuy(t)

	if rec := httpxtest.Send(userB, http.MethodPut, "/api/fx-trades/1", "1", buyBody, api.h.UpdateTrade); rec.Code != http.StatusNotFound {
		t.Errorf("B updates A's trade = %d, want 404", rec.Code)
	}
}

func TestDeleteTradeResponds204(t *testing.T) {
	api := newTestAPI(t)
	api.createBuy(t)

	rec := httpxtest.Send(userA, http.MethodDelete, "/api/fx-trades/1", "1", "", api.h.DeleteTrade)

	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("DELETE = %d %q, want 204 without a body", rec.Code, rec.Body)
	}
	if rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/1", "1", "", api.h.GetTrade); rec.Code != http.StatusNotFound {
		t.Errorf("GET after delete = %d, want 404", rec.Code)
	}
}

func TestPreviewTradeRespondsWithTheComputedRates(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.3")

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/preview?"+previewQuery, "", "", api.h.PreviewTrade)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"trade_rate":"32.42","market_rate":"32.3","market_rate_date":"2026-10-27","spread":"120.00"}`)
	if api.store.writes != 0 {
		t.Errorf("store writes = %d, want none", api.store.writes)
	}
}

func TestPreviewTradeWithoutAMarketRateRespondsWithNulls(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/preview?"+previewQuery, "", "", api.h.PreviewTrade)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"trade_rate":"32.42","market_rate":null,"market_rate_date":null,"spread":null}`)
}

func TestPreviewTradeRejectsAnIncompleteQuery(t *testing.T) {
	api := newTestAPI(t)

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/preview?side=buy&currency=USD&traded_on=2026-10-27&foreign_amount=1000", "", "", api.h.PreviewTrade)

	wantInvalidFields(t, rec, "twd_amount")
}

func TestPreviewTradeWithAnIDUsesTheStoredMarketRate(t *testing.T) {
	api := newTestAPI(t)
	api.rates.snapshot = usdSnapshot("2026-10-26", "32.4")
	api.createBuy(t)
	api.rates.snapshot = usdSnapshot("2026-10-27", "32.3")

	rec := httpxtest.Send(userA, http.MethodGet, "/api/fx-trades/preview?id=1&"+previewQuery, "", "", api.h.PreviewTrade)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"trade_rate":"32.42","market_rate":"32.4","market_rate_date":"2026-10-26","spread":"20.00"}`)
}

func TestPreviewTradeWithAnIDThatIsNotOwnIsNotFound(t *testing.T) {
	api := newTestAPI(t)
	api.createBuy(t)

	for name, tc := range map[string]struct {
		user int64
		id   string
	}{
		"another user's": {userB, "1"},
		"not a number":   {userA, "abc"},
	} {
		rec := httpxtest.Send(tc.user, http.MethodGet, "/api/fx-trades/preview?id="+tc.id+"&"+previewQuery, "", "", api.h.PreviewTrade)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: preview = %d, want 404", name, rec.Code)
		}
	}
}
