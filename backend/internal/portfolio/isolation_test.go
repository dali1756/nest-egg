package portfolio

import (
	"net/http"
	"strings"
	"testing"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/platform/httpx/httpxtest"
	"nest-egg/backend/internal/rate"
)

type isolationFixture struct {
	h    *Handler
	repo *Repository
	a, b int64
}

func newIsolationFixture(t *testing.T) isolationFixture {
	t.Helper()
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	svc := NewService(repo, currency.NewService(currency.NewRepository(pool)), rate.NewRepository(pool))
	if _, err := pool.Exec(t.Context(), `INSERT INTO rate_snapshots (base, quote, rate_date, rate, source) VALUES ('USD', 'TWD', '2026-10-01', 32, 'cbc')`); err != nil {
		t.Fatalf("insert rate: %v", err)
	}
	f := isolationFixture{h: NewHandler(svc), repo: repo, a: dbtest.InsertUser(t, pool, "a@example.com"), b: dbtest.InsertUser(t, pool, "b@example.com")}

	// B 的期初餘額存的是舊的匯率 30，用來確認 A 的試算不會拿到它
	bOpening := OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), MarketRate: marketRate("30", "2026-10-01")}
	if _, err := repo.SetOpening(t.Context(), f.b, "USD", bOpening); err != nil {
		t.Fatalf("B's opening: %v", err)
	}
	bIncome := insertCategory(t, pool, f.b, "income")
	insertEntry(t, pool, f.b, bIncome, "2026-10-05", "TWD", "50000", "")
	insertTrade(t, pool, f.b, "buy", "2026-10-07", "1000", "32420")
	return f
}

const bOpeningJSON = `{"currency":"USD","opened_on":"2026-10-01","amount":"10000.00","twd_cost":null,` +
	`"market_rate":"30","market_rate_date":"2026-10-01","cost":"300000.00","average_cost":"30.0000"}`

func TestUsersCannotSeeOrDeleteEachOthersOpenings(t *testing.T) {
	f := newIsolationFixture(t)

	for _, attempt := range []struct {
		method  string
		handler httpx.Handler
	}{
		{http.MethodGet, f.h.GetOpening},
		{http.MethodDelete, f.h.DeleteOpening},
	} {
		rec := sendCurrency(f.a, attempt.method, "/api/opening-balances/USD", "USD", "", attempt.handler)
		if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"error":"not found"}` {
			t.Errorf("A %s B's USD opening: %d %s, want 404 not found", attempt.method, rec.Code, rec.Body)
		}
	}
	httpxtest.WantResponse(t, httpxtest.Send(f.a, http.MethodGet, "/api/opening-balances", "", "", f.h.ListOpenings),
		http.StatusOK, `{"opening_balances":[]}`)
	httpxtest.WantResponse(t, sendCurrency(f.b, http.MethodGet, "/api/opening-balances/USD", "USD", "", f.h.GetOpening),
		http.StatusOK, bOpeningJSON)
}

func TestSettingAnOpeningOnlyChangesTheUsersOwn(t *testing.T) {
	f := newIsolationFixture(t)

	rec := sendCurrency(f.a, http.MethodPut, "/api/opening-balances/USD", "USD", `{"opened_on":"2026-10-01","amount":"500"}`, f.h.SetOpening)

	httpxtest.WantResponse(t, rec, http.StatusCreated, `{"currency":"USD","opened_on":"2026-10-01","amount":"500.00","twd_cost":null,`+
		`"market_rate":"32","market_rate_date":"2026-10-01","cost":"16000.00","average_cost":"32.0000"}`)
	httpxtest.WantResponse(t, sendCurrency(f.b, http.MethodGet, "/api/opening-balances/USD", "USD", "", f.h.GetOpening),
		http.StatusOK, bOpeningJSON)
}

func TestPreviewDoesNotUseAnotherUsersStoredRate(t *testing.T) {
	f := newIsolationFixture(t)

	rec := httpxtest.Send(f.a, http.MethodGet, "/api/opening-balances/preview?currency=USD&opened_on=2026-10-01&amount=10000", "", "", f.h.PreviewOpening)

	httpxtest.WantResponse(t, rec, http.StatusOK, `{"market_rate":"32","market_rate_date":"2026-10-01","cost":"320000.00","average_cost":"32.0000"}`)
}

func TestSummaryLeavesOutAnotherUsersRecords(t *testing.T) {
	f := newIsolationFixture(t)

	httpxtest.WantResponse(t, httpxtest.Send(f.a, http.MethodGet, "/api/portfolio/summary", "", "", f.h.Summary),
		http.StatusOK, `{"cash":[]}`)
}
