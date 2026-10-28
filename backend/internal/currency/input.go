package currency

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
)

type Getter interface {
	Get(ctx context.Context, code string) (Currency, error)
}

func Lookup(ctx context.Context, currencies Getter, code string) (*Currency, string, error) {
	if code == "" {
		return nil, "required", nil
	}
	c, err := currencies.Get(ctx, code)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return nil, "unknown currency", nil
	case err != nil:
		return nil, "", fmt.Errorf("check currency: %w", err)
	}
	return &c, "", nil
}

var amountPattern = regexp.MustCompile(`^[0-9]{1,16}(\.[0-9]+)?$`)

func ParseAmount(raw string, cur *Currency) (decimal.Decimal, string) {
	if !amountPattern.MatchString(raw) {
		return decimal.Decimal{}, "must be a number like 120 or 32.5"
	}
	amount := decimal.RequireFromString(raw)
	switch {
	case !amount.IsPositive():
		return decimal.Decimal{}, "must be greater than 0"
	case cur != nil && decimalPlaces(raw) > cur.MinorUnit:
		return decimal.Decimal{}, fmt.Sprintf("must have at most %d decimal places", cur.MinorUnit)
	}
	return amount, ""
}

func decimalPlaces(amount string) int {
	if i := strings.IndexByte(amount, '.'); i >= 0 {
		return len(amount) - i - 1
	}
	return 0
}
