package currency

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
)

var usd = Currency{Code: "USD", Name: "美元", MinorUnit: 2}

type fakeGetter struct{ err error }

func (f fakeGetter) Get(_ context.Context, code string) (Currency, error) {
	if f.err != nil {
		return Currency{}, f.err
	}
	if code != usd.Code {
		return Currency{}, apperr.ErrNotFound
	}
	return usd, nil
}

func TestLookupFindsTheCurrency(t *testing.T) {
	got, problem, err := Lookup(context.Background(), fakeGetter{}, "USD")
	if err != nil || problem != "" || got == nil || *got != usd {
		t.Errorf("Lookup(USD) = %+v, %q, %v; want USD", got, problem, err)
	}
}

func TestLookupReportsMissingAndUnknownCurrencies(t *testing.T) {
	for code, want := range map[string]string{"": "required", "XYZ": "unknown currency"} {
		got, problem, err := Lookup(context.Background(), fakeGetter{}, code)
		if err != nil || got != nil || problem != want {
			t.Errorf("Lookup(%q) = %+v, %q, %v; want problem %q", code, got, problem, err, want)
		}
	}
}

func TestLookupPassesStoreErrorsOn(t *testing.T) {
	broken := errors.New("connection refused")
	if _, _, err := Lookup(context.Background(), fakeGetter{err: broken}, "USD"); !errors.Is(err, broken) {
		t.Errorf("Lookup error = %v, want the store error", err)
	}
}

func TestParseAmountAcceptsPositiveNumbers(t *testing.T) {
	for raw, want := range map[string]string{"120": "120", "32.5": "32.5", "1000.00": "1000"} {
		got, problem := ParseAmount(raw, &usd)
		if problem != "" || !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("ParseAmount(%q) = %s %q, want %s", raw, got, problem, want)
		}
	}
}

func TestParseAmountRejectsMalformedAndNonPositiveAmounts(t *testing.T) {
	for _, raw := range []string{"abc", "1e3", "-1", "+5", "0", "0.00", "", "12345678901234567"} {
		if _, problem := ParseAmount(raw, &usd); problem == "" {
			t.Errorf("ParseAmount(%q) accepted, want a problem", raw)
		}
	}
}

func TestParseAmountLimitsDecimalPlacesToTheCurrency(t *testing.T) {
	if _, problem := ParseAmount("1.234", &usd); problem == "" {
		t.Error("ParseAmount(1.234 USD) accepted, want at most 2 decimal places")
	}
}

func TestParseAmountSkipsTheDecimalPlacesForAnUnknownCurrency(t *testing.T) {
	if _, problem := ParseAmount("1.234", nil); problem != "" {
		t.Errorf("ParseAmount(1.234, unknown currency) = %q, want only the currency field to be reported", problem)
	}
}
