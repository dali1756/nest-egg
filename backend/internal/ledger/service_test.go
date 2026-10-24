package ledger

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
)

const (
	userA = int64(1)
	userB = int64(2)
)

var (
	aFood   = Category{ID: 11, Kind: KindExpense, Name: "飲食"}
	aSalary = Category{ID: 15, Kind: KindIncome, Name: "薪水"}
	bFood   = Category{ID: 21, Kind: KindExpense, Name: "飲食"}
)

var fixedNow = time.Date(2026, 10, 23, 16, 30, 0, 0, time.UTC)

func newTestService(t *testing.T) (*Service, *fakeStore, *fakeRates) {
	t.Helper()
	store := newFakeStore()
	store.categories[userA] = []Category{aFood, aSalary}
	store.categories[userB] = []Category{bFood}
	rates := &fakeRates{}
	svc := NewService(store, fakeCurrencies{}, rates)
	svc.now = func() time.Time { return fixedNow }
	return svc, store, rates
}

func validInput() EntryInput {
	return EntryInput{OccurredOn: "2026-10-23", CategoryID: aFood.ID, Currency: "TWD", Amount: "120", Note: "午餐"}
}

func TestCreateEntryRejectsInvalidFields(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*EntryInput)
		want []string
	}{
		"missing date":             {func(in *EntryInput) { in.OccurredOn = "" }, []string{"occurred_on"}},
		"date with slashes":        {func(in *EntryInput) { in.OccurredOn = "2026/10/23" }, []string{"occurred_on"}},
		"date in the future":       {func(in *EntryInput) { in.OccurredOn = "2026-10-25" }, []string{"occurred_on"}},
		"missing category":         {func(in *EntryInput) { in.CategoryID = 0 }, []string{"category_id"}},
		"another user's category":  {func(in *EntryInput) { in.CategoryID = bFood.ID }, []string{"category_id"}},
		"missing currency":         {func(in *EntryInput) { in.Currency = "" }, []string{"currency"}},
		"unknown currency":         {func(in *EntryInput) { in.Currency = "XYZ" }, []string{"currency"}},
		"zero amount":              {func(in *EntryInput) { in.Amount = "0" }, []string{"amount"}},
		"zero with decimals":       {func(in *EntryInput) { in.Amount = "0.00" }, []string{"amount"}},
		"negative amount":          {func(in *EntryInput) { in.Amount = "-5" }, []string{"amount"}},
		"plus sign":                {func(in *EntryInput) { in.Amount = "+5" }, []string{"amount"}},
		"exponent":                 {func(in *EntryInput) { in.Amount = "1e3" }, []string{"amount"}},
		"thousands separator":      {func(in *EntryInput) { in.Amount = "1,000" }, []string{"amount"}},
		"not a number":             {func(in *EntryInput) { in.Amount = "abc" }, []string{"amount"}},
		"empty amount":             {func(in *EntryInput) { in.Amount = "" }, []string{"amount"}},
		"trailing dot":             {func(in *EntryInput) { in.Amount = "10." }, []string{"amount"}},
		"more decimals than TWD":   {func(in *EntryInput) { in.Amount = "10.005" }, []string{"amount"}},
		"seventeen integer digits": {func(in *EntryInput) { in.Amount = "12345678901234567" }, []string{"amount"}},
		"note of 201 characters":   {func(in *EntryInput) { in.Note = strings.Repeat("午", 201) }, []string{"note"}},
		"several fields": {func(in *EntryInput) {
			in.OccurredOn, in.CategoryID, in.Amount = "", 0, "abc"
		}, []string{"amount", "category_id", "occurred_on"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, _ := newTestService(t)
			in := validInput()
			tc.edit(&in)

			_, err := svc.CreateEntry(t.Context(), userA, in)

			var invalid *apperr.ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("CreateEntry error = %v, want a ValidationError", err)
			}
			if got := slices.Sorted(maps.Keys(invalid.Fields)); !slices.Equal(got, tc.want) {
				t.Errorf("invalid fields = %v, want %v", got, tc.want)
			}
			if len(store.entries) != 0 {
				t.Errorf("stored %d entries, want none", len(store.entries))
			}
		})
	}
}

func TestCreateEntryAcceptsValidAmountsAndNotes(t *testing.T) {
	for name, edit := range map[string]func(*EntryInput){
		"smallest TWD amount":    func(in *EntryInput) { in.Amount = "0.01" },
		"one decimal":            func(in *EntryInput) { in.Amount = "10.5" },
		"sixteen integer digits": func(in *EntryInput) { in.Amount = "1234567890123456" },
		"leading zero":           func(in *EntryInput) { in.Amount = "0120" },
		"note of 200 characters": func(in *EntryInput) { in.Note = strings.Repeat("午", 200) },
		"no note":                func(in *EntryInput) { in.Note = "" },
		"today in Taipei":        func(in *EntryInput) { in.OccurredOn = "2026-10-24" },
		"income category":        func(in *EntryInput) { in.CategoryID = aSalary.ID },
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, _ := newTestService(t)
			in := validInput()
			edit(&in)
			if _, err := svc.CreateEntry(t.Context(), userA, in); err != nil {
				t.Errorf("CreateEntry(%+v) = %v, want success", in, err)
			}
		})
	}
}

func TestCreateEntryStoresTheEntry(t *testing.T) {
	svc, _, rates := newTestService(t)

	got, err := svc.CreateEntry(t.Context(), userA, validInput())
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	if got.ID == 0 {
		t.Error("entry ID = 0, want the stored ID")
	}
	wantEntry(t, got, Entry{
		OccurredOn: taipeiDate("2026-10-23"), Category: aFood, Currency: "TWD", MinorUnit: 2,
		Amount: decimal.RequireFromString("120"), Note: "午餐",
	})
	if len(rates.calls) != 0 {
		t.Errorf("looked up rates %v for a TWD entry, want none", rates.calls)
	}
}

func usdInput(date string) EntryInput {
	return EntryInput{OccurredOn: date, CategoryID: aSalary.ID, Currency: "USD", Amount: "50", Note: "股息"}
}

func TestCreateForeignEntryStoresTheLatestMarketRateOnOrBeforeItsDate(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-23", "32.508")

	got, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-24"))
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	wantMarketRate(t, got.MarketRate, "32.508", "2026-10-23")
	want := []rateCall{{"USD", "TWD", taipeiDate("2026-10-24")}}
	if !slices.Equal(rates.calls, want) {
		t.Errorf("rate lookups = %v, want %v", rates.calls, want)
	}
}

func TestCreateForeignEntryWithoutAMarketRateStoresNone(t *testing.T) {
	svc, _, _ := newTestService(t)

	got, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-24"))

	if err != nil || got.MarketRate != nil {
		t.Errorf("CreateEntry = %+v, %v; want the entry without a market rate", got.MarketRate, err)
	}
}

func TestCreateForeignEntryFailsWhenTheRateLookupFails(t *testing.T) {
	svc, store, rates := newTestService(t)
	rates.err = errRateLookup

	_, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-24"))

	var invalid *apperr.ValidationError
	if !errors.Is(err, errRateLookup) || errors.As(err, &invalid) {
		t.Errorf("CreateEntry error = %v, want the lookup error passed up", err)
	}
	if len(store.entries) != 0 {
		t.Errorf("stored %d entries, want none", len(store.entries))
	}
}

func TestUpdateEntryKeepsTheRateWhenDateAndCurrencyAreUnchanged(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-22", "32.508")
	created, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-23"))
	if err != nil {
		t.Fatal(err)
	}
	rates.snapshot = usdSnapshot("2026-10-23", "32.600")

	in := usdInput("2026-10-23")
	in.Note = "改備註"
	got, err := svc.UpdateEntry(t.Context(), userA, created.ID, in)
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	wantMarketRate(t, got.MarketRate, "32.508", "2026-10-22")
	if got.Note != "改備註" || len(rates.calls) != 1 {
		t.Errorf("note = %q, rate lookups = %d; want the new note and no new lookup", got.Note, len(rates.calls))
	}
}

func TestUpdateEntryLooksUpTheRateAgainWhenTheDateChanges(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-01", "32.100")
	created, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	rates.snapshot = usdSnapshot("2026-10-20", "32.617")

	got, err := svc.UpdateEntry(t.Context(), userA, created.ID, usdInput("2026-10-20"))
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	wantMarketRate(t, got.MarketRate, "32.617", "2026-10-20")
	if last := rates.calls[len(rates.calls)-1]; last != (rateCall{"USD", "TWD", taipeiDate("2026-10-20")}) {
		t.Errorf("last rate lookup = %v, want USD/TWD on 2026-10-20", last)
	}
}

func TestUpdateEntryToTWDClearsTheRate(t *testing.T) {
	svc, _, rates := newTestService(t)
	rates.snapshot = usdSnapshot("2026-10-22", "32.508")
	created, err := svc.CreateEntry(t.Context(), userA, usdInput("2026-10-23"))
	if err != nil {
		t.Fatal(err)
	}

	in := usdInput("2026-10-23")
	in.Currency = "TWD"
	got, err := svc.UpdateEntry(t.Context(), userA, created.ID, in)

	if err != nil || got.MarketRate != nil || got.Currency != "TWD" {
		t.Errorf("UpdateEntry = %+v, %v; want a TWD entry without a market rate", got, err)
	}
}

func TestUpdateEntryValidatesLikeCreate(t *testing.T) {
	svc, store, _ := newTestService(t)
	created, err := svc.CreateEntry(t.Context(), userA, validInput())
	if err != nil {
		t.Fatal(err)
	}

	in := validInput()
	in.Amount = "10.005"
	_, err = svc.UpdateEntry(t.Context(), userA, created.ID, in)

	var invalid *apperr.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields["amount"] == "" {
		t.Errorf("UpdateEntry error = %v, want an amount ValidationError", err)
	}
	if got := store.entries[created.ID].Amount; !got.Equal(decimal.RequireFromString("120")) {
		t.Errorf("stored amount = %s, want it unchanged at 120", got)
	}
}

func TestUpdateAnotherUsersEntryIsNotFound(t *testing.T) {
	svc, store, _ := newTestService(t)
	created, err := svc.CreateEntry(t.Context(), userA, validInput())
	if err != nil {
		t.Fatal(err)
	}

	in := validInput()
	in.CategoryID, in.Amount = bFood.ID, "999"
	_, err = svc.UpdateEntry(t.Context(), userB, created.ID, in)

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B updating A's entry: error = %v, want apperr.ErrNotFound", err)
	}
	if got := store.entries[created.ID].Amount; !got.Equal(decimal.RequireFromString("120")) {
		t.Errorf("A's amount = %s, want it unchanged at 120", got)
	}
}

func TestMonthDefaultsToTheCurrentTaipeiMonth(t *testing.T) {
	svc, store, _ := newTestService(t)
	svc.now = func() time.Time { return time.Date(2026, 10, 31, 16, 30, 0, 0, time.UTC) }

	got, err := svc.Month(t.Context(), userA, "")
	if err != nil {
		t.Fatalf("Month: %v", err)
	}

	if got.Month != "2026-11" {
		t.Errorf("month = %q, want 2026-11", got.Month)
	}
	want := monthCall{userA, taipeiDate("2026-11-01"), taipeiDate("2026-12-01")}
	if len(store.monthCalls) == 0 || store.monthCalls[0] != want {
		t.Errorf("store queried %v, want %v", store.monthCalls, want)
	}
}

func TestMonthQueriesFromTheFirstDayToTheNextMonth(t *testing.T) {
	for month, want := range map[string]struct{ from, to string }{
		"2026-10": {"2026-10-01", "2026-11-01"},
		"2026-12": {"2026-12-01", "2027-01-01"},
		"2026-02": {"2026-02-01", "2026-03-01"},
	} {
		svc, store, _ := newTestService(t)
		got, err := svc.Month(t.Context(), userA, month)
		if err != nil {
			t.Fatalf("Month(%s): %v", month, err)
		}
		wantCall := monthCall{userA, taipeiDate(want.from), taipeiDate(want.to)}
		if got.Month != month || len(store.monthCalls) == 0 || store.monthCalls[0] != wantCall {
			t.Errorf("Month(%s) = %q, queried %v; want %v", month, got.Month, store.monthCalls, wantCall)
		}
	}
}

func TestMonthRejectsMalformedMonths(t *testing.T) {
	for _, month := range []string{"2026-13", "202610", "2026-1", "2026-10-01", "abcd-ef"} {
		svc, _, _ := newTestService(t)
		_, err := svc.Month(t.Context(), userA, month)
		var invalid *apperr.ValidationError
		if !errors.As(err, &invalid) || invalid.Fields["month"] == "" {
			t.Errorf("Month(%q) error = %v, want a month ValidationError", month, err)
		}
	}
}

func TestTotalNetIsIncomeMinusExpense(t *testing.T) {
	for _, tc := range []struct{ income, expense, want string }{
		{"50000", "155.5", "49844.5"},
		{"20", "100", "-80"},
		{"12.34", "0", "12.34"},
	} {
		total := Total{Income: decimal.RequireFromString(tc.income), Expense: decimal.RequireFromString(tc.expense)}
		if net := total.Net(); !net.Equal(decimal.RequireFromString(tc.want)) {
			t.Errorf("%s - %s = %s, want %s", tc.income, tc.expense, net, tc.want)
		}
	}
}
