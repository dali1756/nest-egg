package portfolio

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"nest-egg/backend/internal/platform/httpx"
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

func sendCurrency(userID int64, method, path, cur, body string, h httpx.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetPathValue("currency", cur)
	return httpxtest.ServeAs(userID, h, req)
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

const (
	usdOpeningBody = `{"opened_on":"2026-10-01","amount":"10000"}`
	usdOpeningJSON = `{"currency":"USD","opened_on":"2026-10-01","amount":"10000.00","twd_cost":null,` +
		`"market_rate":"32","market_rate_date":"2026-10-01","cost":"320000.00","average_cost":"32.0000"}`
	twdOpeningJSON = `{"currency":"TWD","opened_on":"2026-10-01","amount":"120000.00","twd_cost":null,` +
		`"market_rate":null,"market_rate_date":null,"cost":null,"average_cost":null}`
)

func (api testAPI) putUSD(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	return sendCurrency(userA, http.MethodPut, "/api/opening-balances/USD", "USD", body, api.h.SetOpening)
}

func TestSetOpeningResponds201ThenReplacesWith200(t *testing.T) {
	api := newTestAPI(t)
	api.rates.setUSD("2026-10-01", "32")

	httpxtest.WantResponse(t, api.putUSD(t, usdOpeningBody), http.StatusCreated, usdOpeningJSON)
	httpxtest.WantResponse(t, api.putUSD(t, `{"opened_on":"2026-10-01","amount":"9000.5"}`), http.StatusOK,
		`{"currency":"USD","opened_on":"2026-10-01","amount":"9000.50","twd_cost":null,`+
			`"market_rate":"32","market_rate_date":"2026-10-01","cost":"288016.00","average_cost":"32.0000"}`)
}

func TestSetOpeningWithATWDCostResponds(t *testing.T) {
	api := newTestAPI(t)
	api.rates.setUSD("2026-10-01", "32")

	rec := api.putUSD(t, `{"opened_on":"2026-10-01","amount":"10000","twd_cost":"315000"}`)

	httpxtest.WantResponse(t, rec, http.StatusCreated,
		`{"currency":"USD","opened_on":"2026-10-01","amount":"10000.00","twd_cost":"315000.00",`+
			`"market_rate":"32","market_rate_date":"2026-10-01","cost":"315000.00","average_cost":"31.5000"}`)
}

func TestSetTWDOpeningRespondsWithoutCostFields(t *testing.T) {
	api := newTestAPI(t)

	rec := sendCurrency(userA, http.MethodPut, "/api/opening-balances/TWD", "TWD", `{"opened_on":"2026-10-01","amount":"120000"}`, api.h.SetOpening)

	httpxtest.WantResponse(t, rec, http.StatusCreated, twdOpeningJSON)
}

func TestSetOpeningRejectsUnknownBodyFields(t *testing.T) {
	for name, body := range map[string]string{
		"market rate": `{"opened_on":"2026-10-01","amount":"10000","market_rate":"30"}`,
		"user id":     `{"opened_on":"2026-10-01","amount":"10000","user_id":2}`,
	} {
		api := newTestAPI(t)
		api.rates.setUSD("2026-10-01", "32")

		rec := api.putUSD(t, body)

		if rec.Code != http.StatusBadRequest || api.store.writes != 0 {
			t.Errorf("%s: response = %d %s with %d writes, want 400 and nothing stored", name, rec.Code, rec.Body, api.store.writes)
		}
	}
}

func TestSetOpeningRespondsWithTheInvalidFields(t *testing.T) {
	api := newTestAPI(t)

	rec := api.putUSD(t, `{"opened_on":"2026-11-04","amount":"0"}`)

	wantInvalidFields(t, rec, "amount", "opened_on")
}

func TestSetOpeningForAnUnknownCurrencyResponds404(t *testing.T) {
	api := newTestAPI(t)

	rec := sendCurrency(userA, http.MethodPut, "/api/opening-balances/XYZ", "XYZ", usdOpeningBody, api.h.SetOpening)

	httpxtest.WantResponse(t, rec, http.StatusNotFound, `{"error":"not found"}`)
}

func TestListOpeningsResponds(t *testing.T) {
	api := newTestAPI(t)

	httpxtest.WantResponse(t, httpxtest.Send(userA, http.MethodGet, "/api/opening-balances", "", "", api.h.ListOpenings),
		http.StatusOK, `{"opening_balances":[]}`)

	api.rates.setUSD("2026-10-01", "32")
	api.putUSD(t, usdOpeningBody)
	sendCurrency(userA, http.MethodPut, "/api/opening-balances/TWD", "TWD", `{"opened_on":"2026-10-01","amount":"120000"}`, api.h.SetOpening)

	httpxtest.WantResponse(t, httpxtest.Send(userA, http.MethodGet, "/api/opening-balances", "", "", api.h.ListOpenings),
		http.StatusOK, `{"opening_balances":[`+twdOpeningJSON+`,`+usdOpeningJSON+`]}`)
}

func TestGetOpeningResponds(t *testing.T) {
	api := newTestAPI(t)
	get := func() *httptest.ResponseRecorder {
		return sendCurrency(userA, http.MethodGet, "/api/opening-balances/USD", "USD", "", api.h.GetOpening)
	}

	httpxtest.WantResponse(t, get(), http.StatusNotFound, `{"error":"not found"}`)

	api.rates.setUSD("2026-10-01", "32")
	api.putUSD(t, usdOpeningBody)
	httpxtest.WantResponse(t, get(), http.StatusOK, usdOpeningJSON)
}

func TestDeleteOpeningResponds(t *testing.T) {
	api := newTestAPI(t)
	del := func() *httptest.ResponseRecorder {
		return sendCurrency(userA, http.MethodDelete, "/api/opening-balances/USD", "USD", "", api.h.DeleteOpening)
	}
	api.rates.setUSD("2026-10-01", "32")
	api.putUSD(t, usdOpeningBody)

	if rec := del(); rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("delete = %d %s, want 204 with no body", rec.Code, rec.Body)
	}
	httpxtest.WantResponse(t, del(), http.StatusNotFound, `{"error":"not found"}`)
}

func TestPreviewOpeningReadsTheQuery(t *testing.T) {
	api := newTestAPI(t)
	api.rates.setUSD("2026-10-01", "32")
	preview := func(query string) *httptest.ResponseRecorder {
		return httpxtest.Send(userA, http.MethodGet, "/api/opening-balances/preview?"+query, "", "", api.h.PreviewOpening)
	}

	httpxtest.WantResponse(t, preview("currency=USD&opened_on=2026-10-01&amount=10000"), http.StatusOK,
		`{"market_rate":"32","market_rate_date":"2026-10-01","cost":"320000.00","average_cost":"32.0000"}`)
	httpxtest.WantResponse(t, preview("currency=USD&opened_on=2026-10-01&amount=10000&twd_cost=315000"), http.StatusOK,
		`{"market_rate":"32","market_rate_date":"2026-10-01","cost":"315000.00","average_cost":"31.5000"}`)
	wantInvalidFields(t, preview("currency=USD&opened_on=2007-06-01&amount=10000"), "twd_cost")
	wantInvalidFields(t, preview("currency=XYZ&opened_on=2026-10-01&amount=10000"), "currency")
	if api.store.writes != 0 {
		t.Errorf("store writes = %d, want none", api.store.writes)
	}
}

func (api testAPI) summary(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	return httpxtest.Send(userA, http.MethodGet, "/api/portfolio/summary", "", "", api.h.Summary)
}

func TestSummaryRespondsWithEachCurrency(t *testing.T) {
	api := newTestAPI(t)
	storeHolding11000USD(api.store, userA)
	api.rates.setUSD("2026-11-03", "32.3")

	httpxtest.WantResponse(t, api.summary(t), http.StatusOK, `{"cash":[`+
		`{"currency":"TWD","name":"新臺幣","balance":"-32420.00","twd_value":"-32420.00","average_cost":null,"cost":null,`+
		`"market_rate":null,"market_rate_date":null,"unrealized_pnl":null,"unrealized_pnl_percent":null,`+
		`"unavailable_reason":null,"missing_rate_entry":null},`+
		`{"currency":"USD","name":"美元","balance":"11000.00","twd_value":"355300.00","average_cost":"32.0382","cost":"352420.00",`+
		`"market_rate":"32.3","market_rate_date":"2026-11-03","unrealized_pnl":"2880.00","unrealized_pnl_percent":"0.82",`+
		`"unavailable_reason":null,"missing_rate_entry":null}]}`)
}

func TestSummaryWithoutRecordsRespondsWithAnEmptyList(t *testing.T) {
	api := newTestAPI(t)

	httpxtest.WantResponse(t, api.summary(t), http.StatusOK, `{"cash":[]}`)
}

func TestSummaryRespondsWithTheReasonAndTheEntryWithoutARate(t *testing.T) {
	api := newTestAPI(t)
	api.store.put(userA, usdOpening("2026-10-01", "10000", "320000"))
	api.store.movements[userA] = []Movement{incomeWithoutRate(42, "2026-10-10", "100")}
	api.rates.setUSD("2026-11-03", "32.3")

	httpxtest.WantResponse(t, api.summary(t), http.StatusOK, `{"cash":[`+
		`{"currency":"USD","name":"美元","balance":"10100.00","twd_value":"326230.00","average_cost":null,"cost":null,`+
		`"market_rate":"32.3","market_rate_date":"2026-11-03","unrealized_pnl":null,"unrealized_pnl_percent":null,`+
		`"unavailable_reason":"missing_income_rate","missing_rate_entry":{"id":42,"occurred_on":"2026-10-10"}}]}`)
}

func TestSummaryFormatsTWDAmountsWithTWDDecimals(t *testing.T) {
	api := newTestAPI(t)
	api.store.put(userA, Opening{Currency: "JPY", MinorUnit: 0, OpenedOn: taipeiDate("2026-10-01"), Amount: dec("3000"), TWDCost: decPtr("650.5")})
	api.rates.snapshots["JPY 2026-11-03"] = rateSnapshot("JPY", "2026-11-03", "0.2")

	httpxtest.WantResponse(t, api.summary(t), http.StatusOK, `{"cash":[`+
		`{"currency":"JPY","name":"日圓","balance":"3000","twd_value":"600.00","average_cost":"0.2168","cost":"650.50",`+
		`"market_rate":"0.2","market_rate_date":"2026-11-03","unrealized_pnl":"-50.50","unrealized_pnl_percent":"-7.76",`+
		`"unavailable_reason":null,"missing_rate_entry":null}]}`)
}

func TestSetOpeningFormatsTheTWDCostWithTWDDecimals(t *testing.T) {
	api := newTestAPI(t)

	rec := sendCurrency(userA, http.MethodPut, "/api/opening-balances/JPY", "JPY", `{"opened_on":"2026-10-01","amount":"3000","twd_cost":"650.5"}`, api.h.SetOpening)

	httpxtest.WantResponse(t, rec, http.StatusCreated,
		`{"currency":"JPY","opened_on":"2026-10-01","amount":"3000","twd_cost":"650.50",`+
			`"market_rate":null,"market_rate_date":null,"cost":"650.50","average_cost":"0.2168"}`)
}
