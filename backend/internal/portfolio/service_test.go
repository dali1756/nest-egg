package portfolio

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/rate"
)

const (
	userA = int64(1)
	userB = int64(2)
)

// 台北時間 2026-11-03 07:00；UTC 還是 11-02，用來確認「今天」是台北的日期
var fixedNow = time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC)

func newTestService(t *testing.T) (*Service, *fakeStore, *fakeRates) {
	t.Helper()
	store := newFakeStore()
	rates := &fakeRates{snapshots: map[string]rate.Snapshot{}}
	svc := NewService(store, fakeCurrencies{}, rates)
	svc.now = func() time.Time { return fixedNow }
	return svc, store, rates
}

func rateSnapshot(base, date, value string) rate.Snapshot {
	return rate.Snapshot{Rate: rate.Rate{Base: base, Quote: "TWD", Date: taipeiDate(date), Value: dec(value)}, Source: "cbc"}
}

func (f *fakeRates) setUSD(date, value string) {
	f.snapshots["USD "+date] = rateSnapshot("USD", date, value)
}

func usdOpeningInput() OpeningInput {
	return OpeningInput{OpenedOn: "2026-10-01", Amount: "10000"}
}

func wantFields(t *testing.T, err error, want ...string) {
	t.Helper()
	var invalid *apperr.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want a validation error on %v", err, want)
	}
	got := slices.Sorted(maps.Keys(invalid.Fields))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("invalid fields = %v, want %v", got, want)
	}
}

func TestSetOpeningRejectsInvalidFields(t *testing.T) {
	for name, tc := range map[string]struct {
		currency string
		edit     func(*OpeningInput)
		want     []string
	}{
		"missing date":                   {"USD", func(in *OpeningInput) { in.OpenedOn = "" }, []string{"opened_on"}},
		"malformed date":                 {"USD", func(in *OpeningInput) { in.OpenedOn = "2026/10/01" }, []string{"opened_on"}},
		"date in the future":             {"USD", func(in *OpeningInput) { in.OpenedOn = "2026-11-04" }, []string{"opened_on"}},
		"malformed amount":               {"USD", func(in *OpeningInput) { in.Amount = "1e4" }, []string{"amount"}},
		"zero amount":                    {"USD", func(in *OpeningInput) { in.Amount = "0" }, []string{"amount"}},
		"too many USD decimals":          {"USD", func(in *OpeningInput) { in.Amount = "10000.123" }, []string{"amount"}},
		"decimals in JPY":                {"JPY", func(in *OpeningInput) { in.Amount = "3000.5"; in.TWDCost = "650" }, []string{"amount"}},
		"TWD with a TWD cost":            {"TWD", func(in *OpeningInput) { in.TWDCost = "1000" }, []string{"twd_cost"}},
		"zero TWD cost":                  {"USD", func(in *OpeningInput) { in.TWDCost = "0" }, []string{"twd_cost"}},
		"too many TWD cost decimals":     {"USD", func(in *OpeningInput) { in.TWDCost = "320000.123" }, []string{"twd_cost"}},
		"no market rate and no TWD cost": {"USD", func(in *OpeningInput) { in.OpenedOn = "2007-06-01" }, []string{"twd_cost"}},
		"several fields":                 {"USD", func(in *OpeningInput) { in.OpenedOn, in.Amount = "2026-11-04", "0" }, []string{"opened_on", "amount"}},
		"bad amount and no rate":         {"USD", func(in *OpeningInput) { in.OpenedOn, in.Amount = "2007-06-01", "0" }, []string{"amount", "twd_cost"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, rates := newTestService(t)
			rates.setUSD("2026-10-01", "32")
			in := usdOpeningInput()
			tc.edit(&in)

			_, _, err := svc.SetOpening(t.Context(), userA, tc.currency, in)

			wantFields(t, err, tc.want...)
			if store.writes != 0 {
				t.Errorf("store writes = %d, want none", store.writes)
			}
		})
	}
}

func TestSetOpeningAcceptsATWDCostWhenThereIsNoMarketRate(t *testing.T) {
	svc, _, _ := newTestService(t)

	o, _, err := svc.SetOpening(t.Context(), userA, "USD", OpeningInput{OpenedOn: "2007-06-01", Amount: "10000", TWDCost: "330000"})
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}
	if o.MarketRate != nil || !sameDecimal(o.TWDCost, decPtr("330000")) {
		t.Errorf("market rate, TWD cost = %+v %v; want none, 330000", o.MarketRate, o.TWDCost)
	}
}

func TestSetOpeningForAnUnknownCurrencyIsNotFound(t *testing.T) {
	svc, store, _ := newTestService(t)

	_, _, err := svc.SetOpening(t.Context(), userA, "XYZ", usdOpeningInput())

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if store.writes != 0 {
		t.Errorf("store writes = %d, want none", store.writes)
	}
}

func TestSetOpeningAcceptsTodayInTaipei(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.setUSD("2026-11-03", "32.3")

	if _, _, err := svc.SetOpening(t.Context(), userA, "USD", OpeningInput{OpenedOn: "2026-11-03", Amount: "10000"}); err != nil {
		t.Errorf("SetOpening for today in Taipei: %v", err)
	}
}

func TestSetOpeningChecksTheTWDCostAgainstTWDDecimals(t *testing.T) {
	svc, _, _ := newTestService(t)

	if _, _, err := svc.SetOpening(t.Context(), userA, "JPY", OpeningInput{OpenedOn: "2026-10-01", Amount: "3000", TWDCost: "650.55"}); err != nil {
		t.Errorf("JPY with a TWD cost of 650.55: %v, want it to be allowed", err)
	}
}

func wantLookups(t *testing.T, rates *fakeRates, want ...rateLookup) {
	t.Helper()
	if !slices.Equal(rates.lookups, want) {
		t.Errorf("rate lookups = %v, want %v", rates.lookups, want)
	}
}

func TestSetOpeningStoresTheForeignMarketRate(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.setUSD("2026-10-01", "32")

	o, created, err := svc.SetOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if !created {
		t.Error("created = false, want true for a new opening")
	}
	want := marketRate("32", "2026-10-01")
	if !sameMarketRate(o.MarketRate, want) || !sameMarketRate(store.openings[openingKey{userA, "USD"}].MarketRate, want) {
		t.Errorf("market rate = %+v, want %+v", o.MarketRate, want)
	}
	wantLookups(t, rates, rateLookup{"USD", "TWD", "2026-10-01"})
}

func TestSetOpeningDoesNotLookUpATWDRate(t *testing.T) {
	svc, _, rates := newTestService(t)

	o, _, err := svc.SetOpening(t.Context(), userA, "TWD", OpeningInput{OpenedOn: "2026-10-01", Amount: "120000"})
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if o.MarketRate != nil || o.TWDCost != nil {
		t.Errorf("market rate, TWD cost = %+v %v; want none", o.MarketRate, o.TWDCost)
	}
	wantLookups(t, rates)
}

func storedUSDOpening(date, rateValue string) Opening {
	return Opening{Currency: "USD", MinorUnit: 2, OpenedOn: taipeiDate(date), Amount: dec("10000"), MarketRate: marketRate(rateValue, date)}
}

func TestReplacingAnOpeningOnTheSameDateKeepsItsMarketRate(t *testing.T) {
	svc, store, rates := newTestService(t)
	store.put(userA, storedUSDOpening("2026-10-01", "32"))
	rates.setUSD("2026-10-01", "32.1")

	o, created, err := svc.SetOpening(t.Context(), userA, "USD", OpeningInput{OpenedOn: "2026-10-01", Amount: "9000"})
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if created {
		t.Error("created = true, want false when replacing")
	}
	if !sameMarketRate(o.MarketRate, marketRate("32", "2026-10-01")) || !o.Amount.Equal(dec("9000")) {
		t.Errorf("amount, market rate = %s %+v; want 9000 with the stored rate 32", o.Amount, o.MarketRate)
	}
	wantLookups(t, rates)
}

func TestReplacingAnOpeningWithANewDateLooksUpTheRateAgain(t *testing.T) {
	svc, store, rates := newTestService(t)
	store.put(userA, storedUSDOpening("2026-10-01", "32"))
	rates.setUSD("2026-09-01", "31.8")

	o, _, err := svc.SetOpening(t.Context(), userA, "USD", OpeningInput{OpenedOn: "2026-09-01", Amount: "10000"})
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if !sameMarketRate(o.MarketRate, marketRate("31.8", "2026-09-01")) {
		t.Errorf("market rate = %+v, want 31.8 on 2026-09-01", o.MarketRate)
	}
	wantLookups(t, rates, rateLookup{"USD", "TWD", "2026-09-01"})
}

func TestSetOpeningIgnoresAnotherUsersStoredRate(t *testing.T) {
	svc, store, rates := newTestService(t)
	store.put(userB, storedUSDOpening("2026-10-01", "30"))
	rates.setUSD("2026-10-01", "32")

	o, created, err := svc.SetOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if !created || !sameMarketRate(o.MarketRate, marketRate("32", "2026-10-01")) {
		t.Errorf("created, market rate = %v %+v; want a new opening with the current rate 32", created, o.MarketRate)
	}
	if !sameMarketRate(store.openings[openingKey{userB, "USD"}].MarketRate, marketRate("30", "2026-10-01")) {
		t.Error("B's opening changed")
	}
}

func TestSetOpeningReturnsStoreErrors(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.setUSD("2026-10-01", "32")
	store.readErr = errors.New("connection reset")

	_, _, err := svc.SetOpening(t.Context(), userA, "USD", usdOpeningInput())

	if !errors.Is(err, store.readErr) {
		t.Errorf("error = %v, want the store error", err)
	}
	if store.writes != 0 {
		t.Errorf("store writes = %d, want none", store.writes)
	}
}

func TestOpeningReturnsTheStoredOneWithTheTWDMinorUnit(t *testing.T) {
	svc, store, _ := newTestService(t)
	stored := storedUSDOpening("2026-10-01", "32")
	store.put(userA, stored)

	got, err := svc.Opening(t.Context(), userA, "USD")
	if err != nil {
		t.Fatalf("Opening: %v", err)
	}

	wantOpening(t, got, stored)
	if got.TWDMinorUnit != 2 {
		t.Errorf("TWD minor unit = %d, want 2", got.TWDMinorUnit)
	}
}

func TestOpeningsListsOnlyTheUsersOwnWithTheTWDMinorUnit(t *testing.T) {
	svc, store, _ := newTestService(t)
	store.put(userA, Opening{Currency: "TWD", MinorUnit: 2, OpenedOn: taipeiDate("2026-10-01"), Amount: dec("120000")})
	store.put(userA, storedUSDOpening("2026-10-01", "32"))
	store.put(userB, storedUSDOpening("2026-10-01", "30"))

	got, err := svc.Openings(t.Context(), userA)
	if err != nil {
		t.Fatalf("Openings: %v", err)
	}

	if len(got) != 2 || got[0].Currency != "TWD" || got[1].Currency != "USD" || !sameMarketRate(got[1].MarketRate, marketRate("32", "2026-10-01")) {
		t.Fatalf("openings = %+v, want A's TWD and USD", got)
	}
	for _, o := range got {
		if o.TWDMinorUnit != 2 {
			t.Errorf("%s TWD minor unit = %d, want 2", o.Currency, o.TWDMinorUnit)
		}
	}
}

func TestSetOpeningReturnsTheTWDMinorUnit(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.setUSD("2026-10-01", "32")

	o, _, err := svc.SetOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("SetOpening: %v", err)
	}
	if o.TWDMinorUnit != 2 {
		t.Errorf("TWD minor unit = %d, want 2", o.TWDMinorUnit)
	}
}

func TestDeleteOpeningRemovesIt(t *testing.T) {
	svc, store, _ := newTestService(t)
	store.put(userA, storedUSDOpening("2026-10-01", "32"))

	if err := svc.DeleteOpening(t.Context(), userA, "USD"); err != nil {
		t.Fatalf("DeleteOpening: %v", err)
	}
	if _, ok := store.openings[openingKey{userA, "USD"}]; ok {
		t.Error("the opening is still stored")
	}
}

func TestOpeningThatIsNotSetIsNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)

	if _, err := svc.Opening(t.Context(), userA, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("Opening: err = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteOpening(t.Context(), userA, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("DeleteOpening: err = %v, want ErrNotFound", err)
	}
}

func TestPreviewOpeningCalculatesWithoutSaving(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.setUSD("2026-10-01", "32")

	o, err := svc.PreviewOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("PreviewOpening: %v", err)
	}

	wantOpening(t, o, Opening{Currency: "USD", MinorUnit: 2, OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), MarketRate: marketRate("32", "2026-10-01")})
	want(t, "cost", o.Cost(), 2, "320000.00")
	if o.TWDMinorUnit != 2 {
		t.Errorf("TWD minor unit = %d, want 2", o.TWDMinorUnit)
	}
	if store.writes != 0 || len(store.openings) != 0 {
		t.Errorf("store writes = %d, openings = %d; want nothing saved", store.writes, len(store.openings))
	}
}

func TestPreviewOpeningUsesTheStoredRateForTheSameDate(t *testing.T) {
	svc, store, rates := newTestService(t)
	store.put(userA, storedUSDOpening("2026-10-01", "32"))
	rates.setUSD("2026-10-01", "32.1")

	o, err := svc.PreviewOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("PreviewOpening: %v", err)
	}

	if !sameMarketRate(o.MarketRate, marketRate("32", "2026-10-01")) {
		t.Errorf("market rate = %+v, want the stored 32", o.MarketRate)
	}
	wantLookups(t, rates)
}

func TestPreviewOpeningIgnoresAnotherUsersStoredRate(t *testing.T) {
	svc, store, rates := newTestService(t)
	store.put(userB, storedUSDOpening("2026-10-01", "30"))
	rates.setUSD("2026-10-01", "32")

	o, err := svc.PreviewOpening(t.Context(), userA, "USD", usdOpeningInput())
	if err != nil {
		t.Fatalf("PreviewOpening: %v", err)
	}

	if !sameMarketRate(o.MarketRate, marketRate("32", "2026-10-01")) {
		t.Errorf("market rate = %+v, want the current 32, not B's stored rate", o.MarketRate)
	}
}

func TestPreviewOpeningRejectsInvalidFields(t *testing.T) {
	for name, tc := range map[string]struct {
		currency string
		in       OpeningInput
		want     []string
	}{
		"zero amount":                    {"USD", OpeningInput{OpenedOn: "2026-10-01", Amount: "0"}, []string{"amount"}},
		"no market rate and no TWD cost": {"USD", OpeningInput{OpenedOn: "2007-06-01", Amount: "10000"}, []string{"twd_cost"}},
		"unknown currency":               {"XYZ", usdOpeningInput(), []string{"currency"}},
		"missing currency":               {"", usdOpeningInput(), []string{"currency"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, rates := newTestService(t)
			rates.setUSD("2026-10-01", "32")

			_, err := svc.PreviewOpening(t.Context(), userA, tc.currency, tc.in)

			wantFields(t, err, tc.want...)
		})
	}
}

func storeHolding11000USD(store *fakeStore, userID int64) {
	openings, movements := holding11000USD()
	store.put(userID, openings[0])
	store.movements[userID] = movements
}

func TestSummaryCalculatesTheUsersOwnRecords(t *testing.T) {
	svc, store, rates := newTestService(t)
	storeHolding11000USD(store, userA)
	store.movements[userB] = []Movement{expense(9, "2026-10-09", "TWD", "999")}
	rates.setUSD("2026-11-03", "32.3")

	rows, err := svc.Summary(t.Context(), userA)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if got := currencyCodes(rows); !slices.Equal(got, []string{"TWD", "USD"}) {
		t.Fatalf("currencies = %v, want [TWD USD]", got)
	}
	wantBalance(t, rows, "TWD", "-32420.00")
	wantValueAndPnL(t, rowOf(t, rows, "USD"), "355300.00", "2880.00", "0.82")
}

func TestSummaryLooksUpEachForeignCurrencyForTodayInTaipei(t *testing.T) {
	svc, store, rates := newTestService(t)
	storeHolding11000USD(store, userA)
	store.movements[userA] = append(store.movements[userA], expense(2, "2026-10-09", "JPY", "3000"))

	if _, err := svc.Summary(t.Context(), userA); err != nil {
		t.Fatalf("Summary: %v", err)
	}

	slices.SortFunc(rates.lookups, func(a, b rateLookup) int { return strings.Compare(a.base, b.base) })
	wantLookups(t, rates, rateLookup{"JPY", "TWD", "2026-11-03"}, rateLookup{"USD", "TWD", "2026-11-03"})
}

func TestSummaryWithoutALatestRate(t *testing.T) {
	svc, store, _ := newTestService(t)
	storeHolding11000USD(store, userA)

	rows, err := svc.Summary(t.Context(), userA)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	wantUnavailable(t, rowOf(t, rows, "USD"), MissingLatestRate)
}

func TestSummaryReturnsRateSourceErrors(t *testing.T) {
	svc, store, rates := newTestService(t)
	storeHolding11000USD(store, userA)
	rates.err = errors.New("connection reset")

	if _, err := svc.Summary(t.Context(), userA); !errors.Is(err, rates.err) {
		t.Errorf("error = %v, want the rate source error", err)
	}
}
