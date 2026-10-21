package rate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
)

type Store interface {
	Upsert(ctx context.Context, source string, rates []Rate) (UpsertResult, error)
	On(ctx context.Context, base, quote string, date time.Time) (Snapshot, error)
}

type CurrencyChecker interface {
	Exists(ctx context.Context, code string) (bool, error)
}

type Service struct {
	store      Store
	provider   RateProvider
	currencies CurrencyChecker
	now        func() time.Time
}

func NewService(store Store, provider RateProvider, currencies CurrencyChecker) *Service {
	return &Service{store: store, provider: provider, currencies: currencies, now: time.Now}
}

type UpdateResult struct {
	Source            string
	Fetched           int
	Inserted, Updated int64
	Latest            time.Time
}

func (s *Service) Update(ctx context.Context) (UpdateResult, error) {
	source := s.provider.Name()
	rates, err := s.provider.Fetch(ctx)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("fetch %s rates: %w", source, err)
	}
	if err := validate(rates, taipei.Today(s.now())); err != nil {
		return UpdateResult{}, fmt.Errorf("validate %s rates: %w", source, err)
	}
	res, err := s.store.Upsert(ctx, source, rates)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("store %s rates: %w", source, err)
	}
	result := UpdateResult{Source: source, Fetched: len(rates), Inserted: res.Inserted, Updated: res.Updated}
	for _, rt := range rates {
		if rt.Date.After(result.Latest) {
			result.Latest = rt.Date
		}
	}
	return result, nil
}

func validate(rates []Rate, today time.Time) error {
	if len(rates) == 0 {
		return errors.New("no rates")
	}
	type key struct{ base, quote, date string }
	seen := make(map[key]bool, len(rates))
	for _, rt := range rates {
		k := key{rt.Base, rt.Quote, rt.Date.Format(time.DateOnly)}
		if seen[k] {
			return fmt.Errorf("duplicate %s/%s rate on %s", rt.Base, rt.Quote, rt.Date.Format(time.DateOnly))
		}
		seen[k] = true
		if rt.Date.After(today) {
			return fmt.Errorf("%s/%s rate dated %s is in the future", rt.Base, rt.Quote, rt.Date.Format(time.DateOnly))
		}
		if !rt.Value.IsPositive() {
			return fmt.Errorf("%s/%s rate on %s is %s, want > 0", rt.Base, rt.Quote, rt.Date.Format(time.DateOnly), rt.Value)
		}
	}
	return nil
}

func (s *Service) On(ctx context.Context, base, quote, date string) (Snapshot, error) {
	fields := map[string]string{}
	for name, code := range map[string]string{"base": base, "quote": quote} {
		if code == "" {
			fields[name] = "required"
			continue
		}
		ok, err := s.currencies.Exists(ctx, code)
		if err != nil {
			return Snapshot{}, fmt.Errorf("check %s currency: %w", name, err)
		}
		if !ok {
			fields[name] = "unknown currency"
		}
	}

	today := taipei.Today(s.now())
	day := today
	if date != "" {
		d, err := taipei.ParseDate(date)
		switch {
		case err != nil:
			fields["date"] = "must be a date in YYYY-MM-DD format"
		case d.After(today):
			fields["date"] = "must not be in the future"
		default:
			day = d
		}
	}
	if len(fields) > 0 {
		return Snapshot{}, &apperr.ValidationError{Fields: fields}
	}
	return s.store.On(ctx, base, quote, day)
}
