package portfolio

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/platform/validate"
	"nest-egg/backend/internal/rate"
)

type Opening struct {
	Currency     string
	MinorUnit    int
	OpenedOn     time.Time
	Amount       decimal.Decimal
	TWDCost      *decimal.Decimal
	MarketRate   *rate.MarketRate
	TWDMinorUnit int
}

func (o Opening) Cost() *decimal.Decimal {
	if o.TWDCost != nil {
		return o.TWDCost
	}
	if o.MarketRate == nil {
		return nil
	}
	cost := o.Amount.Mul(o.MarketRate.Value)
	return &cost
}

func (o Opening) AverageCost() *decimal.Decimal {
	cost := o.Cost()
	if cost == nil {
		return nil
	}
	average := cost.Div(o.Amount)
	return &average
}

type OpeningValues struct {
	OpenedOn   time.Time
	Amount     decimal.Decimal
	TWDCost    *decimal.Decimal
	MarketRate *rate.MarketRate
}

type Store interface {
	SetOpening(ctx context.Context, userID int64, cur string, v OpeningValues) (bool, error)
	Opening(ctx context.Context, userID int64, cur string) (Opening, error)
	Openings(ctx context.Context, userID int64) ([]Opening, error)
	DeleteOpening(ctx context.Context, userID int64, cur string) error
	Movements(ctx context.Context, userID int64) ([]Movement, error)
}

type Currencies interface {
	currency.Getter
	List(ctx context.Context) ([]currency.Currency, error)
}

type OpeningInput struct {
	OpenedOn string
	Amount   string
	TWDCost  string
}

type Service struct {
	store      Store
	currencies Currencies
	rates      rate.Finder
	now        func() time.Time
}

func NewService(store Store, currencies Currencies, rates rate.Finder) *Service {
	return &Service{store: store, currencies: currencies, rates: rates, now: time.Now}
}

func (s *Service) SetOpening(ctx context.Context, userID int64, code string, in OpeningInput) (o Opening, created bool, err error) {
	cur, err := s.currencies.Get(ctx, code)
	if err != nil {
		return Opening{}, false, err
	}
	v, err := s.openingValues(ctx, userID, cur, in)
	if err != nil {
		return Opening{}, false, err
	}
	if created, err = s.store.SetOpening(ctx, userID, code, v); err != nil {
		return Opening{}, false, fmt.Errorf("set %s opening balance: %w", code, err)
	}
	if o, err = s.Opening(ctx, userID, code); err != nil {
		return Opening{}, false, err
	}
	return o, created, nil
}

func (s *Service) PreviewOpening(ctx context.Context, userID int64, code string, in OpeningInput) (Opening, error) {
	cur, problem, err := currency.Lookup(ctx, s.currencies, code)
	if err != nil {
		return Opening{}, err
	}
	if problem != "" {
		return Opening{}, &apperr.ValidationError{Fields: map[string]string{"currency": problem}}
	}
	v, err := s.openingValues(ctx, userID, *cur, in)
	if err != nil {
		return Opening{}, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return Opening{}, err
	}
	return Opening{
		Currency: cur.Code, MinorUnit: cur.MinorUnit, OpenedOn: v.OpenedOn, Amount: v.Amount,
		TWDCost: v.TWDCost, MarketRate: v.MarketRate, TWDMinorUnit: twd.MinorUnit,
	}, nil
}

func (s *Service) Opening(ctx context.Context, userID int64, code string) (Opening, error) {
	o, err := s.store.Opening(ctx, userID, code)
	if err != nil {
		return Opening{}, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return Opening{}, err
	}
	o.TWDMinorUnit = twd.MinorUnit
	return o, nil
}

func (s *Service) Openings(ctx context.Context, userID int64) ([]Opening, error) {
	openings, err := s.store.Openings(ctx, userID)
	if err != nil {
		return nil, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return nil, err
	}
	for i := range openings {
		openings[i].TWDMinorUnit = twd.MinorUnit
	}
	return openings, nil
}

func (s *Service) DeleteOpening(ctx context.Context, userID int64, code string) error {
	return s.store.DeleteOpening(ctx, userID, code)
}

func (s *Service) Summary(ctx context.Context, userID int64) ([]CashBalance, error) {
	currencies, err := s.currencies.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}
	openings, err := s.store.Openings(ctx, userID)
	if err != nil {
		return nil, err
	}
	movements, err := s.store.Movements(ctx, userID)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestRates(ctx, currenciesIn(openings, movements))
	if err != nil {
		return nil, err
	}
	return Summarize(currencies, openings, movements, latest), nil
}

func (s *Service) latestRates(ctx context.Context, codes []string) (map[string]*rate.MarketRate, error) {
	today := taipei.Today(s.now())
	latest := map[string]*rate.MarketRate{}
	for _, code := range codes {
		r, err := rate.MarketRateInTWD(ctx, s.rates, code, today)
		if err != nil {
			return nil, err
		}
		latest[code] = r
	}
	return latest, nil
}

func (s *Service) twd(ctx context.Context) (currency.Currency, error) {
	twd, err := s.currencies.Get(ctx, currency.TWD)
	if err != nil {
		return currency.Currency{}, fmt.Errorf("look up %s: %w", currency.TWD, err)
	}
	return twd, nil
}

func (s *Service) openingValues(ctx context.Context, userID int64, cur currency.Currency, in OpeningInput) (OpeningValues, error) {
	twd, err := s.twd(ctx)
	if err != nil {
		return OpeningValues{}, err
	}
	date, dateProblem := validate.PastDate(in.OpenedOn, taipei.Today(s.now()))
	amount, amountProblem := currency.ParseAmount(in.Amount, &cur)
	twdCost, twdCostProblem := parseTWDCost(in.TWDCost, cur, twd)

	var marketRate *rate.MarketRate
	if dateProblem == "" {
		if marketRate, err = s.marketRateFor(ctx, userID, cur.Code, date); err != nil {
			return OpeningValues{}, err
		}
		if twdCostProblem == "" && cur.Code != currency.TWD && marketRate == nil && twdCost == nil {
			twdCostProblem = "required when there is no market rate for this date"
		}
	}

	if err := validate.Fields(map[string]string{
		"opened_on": dateProblem,
		"amount":    amountProblem,
		"twd_cost":  twdCostProblem,
	}); err != nil {
		return OpeningValues{}, err
	}
	return OpeningValues{OpenedOn: date, Amount: amount, TWDCost: twdCost, MarketRate: marketRate}, nil
}

func (s *Service) marketRateFor(ctx context.Context, userID int64, code string, date time.Time) (*rate.MarketRate, error) {
	current, err := s.store.Opening(ctx, userID, code)
	switch {
	case err == nil && current.OpenedOn.Equal(date):
		return current.MarketRate, nil
	case err != nil && !errors.Is(err, apperr.ErrNotFound):
		return nil, err
	}
	return rate.MarketRateInTWD(ctx, s.rates, code, date)
}

func parseTWDCost(raw string, cur, twd currency.Currency) (*decimal.Decimal, string) {
	if raw == "" {
		return nil, ""
	}
	if cur.Code == currency.TWD {
		return nil, "must be empty for TWD"
	}
	cost, problem := currency.ParseAmount(raw, &twd)
	if problem != "" {
		return nil, problem
	}
	return &cost, ""
}
