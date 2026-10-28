package fx

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/platform/validate"
	"nest-egg/backend/internal/rate"
)

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type Trade struct {
	ID            int64
	TradedOn      time.Time
	Side          Side
	Currency      string
	MinorUnit     int
	ForeignAmount decimal.Decimal
	TWDAmount     decimal.Decimal
	MarketRate    *rate.MarketRate
	Note          string
	TWDMinorUnit  int
}

const tradeRatePlaces = 8

func (t Trade) TradeRate() decimal.Decimal {
	return t.TWDAmount.DivRound(t.ForeignAmount, tradeRatePlaces)
}

func (t Trade) Spread() *decimal.Decimal {
	if t.MarketRate == nil {
		return nil
	}
	atMarket := t.ForeignAmount.Mul(t.MarketRate.Value)
	spread := t.TWDAmount.Sub(atMarket)
	if t.Side == SideSell {
		spread = atMarket.Sub(t.TWDAmount)
	}
	spread = spread.Round(int32(t.TWDMinorUnit))
	return &spread
}

type TradeValues struct {
	TradedOn      time.Time
	Side          Side
	Currency      string
	ForeignAmount decimal.Decimal
	TWDAmount     decimal.Decimal
	MarketRate    *rate.MarketRate
	Note          string
}

type Store interface {
	CreateTrade(ctx context.Context, userID int64, v TradeValues) (int64, error)
	Trade(ctx context.Context, userID, id int64) (Trade, error)
	UpdateTrade(ctx context.Context, userID, id int64, v TradeValues) error
	DeleteTrade(ctx context.Context, userID, id int64) error
	Trades(ctx context.Context, userID int64, from, to time.Time) ([]Trade, error)
}

type TradeInput struct {
	TradedOn      string
	Side          string
	Currency      string
	ForeignAmount string
	TWDAmount     string
	Note          string
}

type MonthTrades struct {
	Month  string
	Trades []Trade
}

type Service struct {
	store      Store
	currencies currency.Getter
	rates      rate.Finder
	now        func() time.Time
}

func NewService(store Store, currencies currency.Getter, rates rate.Finder) *Service {
	return &Service{store: store, currencies: currencies, rates: rates, now: time.Now}
}

func (s *Service) CreateTrade(ctx context.Context, userID int64, in TradeInput) (Trade, error) {
	v, err := s.validate(ctx, in)
	if err != nil {
		return Trade{}, err
	}
	if v.MarketRate, err = s.marketRateFor(ctx, v, nil); err != nil {
		return Trade{}, err
	}
	id, err := s.store.CreateTrade(ctx, userID, v)
	if err != nil {
		return Trade{}, fmt.Errorf("create fx trade: %w", err)
	}
	return s.Trade(ctx, userID, id)
}

func (s *Service) Trade(ctx context.Context, userID, id int64) (Trade, error) {
	t, err := s.store.Trade(ctx, userID, id)
	if err != nil {
		return Trade{}, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return Trade{}, err
	}
	t.TWDMinorUnit = twd.MinorUnit
	return t, nil
}

func (s *Service) UpdateTrade(ctx context.Context, userID, id int64, in TradeInput) (Trade, error) {
	current, err := s.store.Trade(ctx, userID, id)
	if err != nil {
		return Trade{}, err
	}
	v, err := s.validate(ctx, in)
	if err != nil {
		return Trade{}, err
	}
	if v.MarketRate, err = s.marketRateFor(ctx, v, &current); err != nil {
		return Trade{}, err
	}
	if err := s.store.UpdateTrade(ctx, userID, id, v); err != nil {
		return Trade{}, fmt.Errorf("update fx trade %d: %w", id, err)
	}
	return s.Trade(ctx, userID, id)
}

func (s *Service) DeleteTrade(ctx context.Context, userID, id int64) error {
	return s.store.DeleteTrade(ctx, userID, id)
}

func (s *Service) Month(ctx context.Context, userID int64, month string) (MonthTrades, error) {
	first, err := validate.MonthStart(month, taipei.Today(s.now()))
	if err != nil {
		return MonthTrades{}, err
	}
	trades, err := s.store.Trades(ctx, userID, first, first.AddDate(0, 1, 0))
	if err != nil {
		return MonthTrades{}, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return MonthTrades{}, err
	}
	for i := range trades {
		trades[i].TWDMinorUnit = twd.MinorUnit
	}
	return MonthTrades{Month: first.Format(validate.MonthLayout), Trades: trades}, nil
}

func (s *Service) PreviewNew(ctx context.Context, in TradeInput) (Trade, error) {
	return s.preview(ctx, in, nil)
}

func (s *Service) PreviewChange(ctx context.Context, userID, id int64, in TradeInput) (Trade, error) {
	current, err := s.store.Trade(ctx, userID, id)
	if err != nil {
		return Trade{}, err
	}
	return s.preview(ctx, in, &current)
}

func (s *Service) preview(ctx context.Context, in TradeInput, current *Trade) (Trade, error) {
	v, err := s.validate(ctx, in)
	if err != nil {
		return Trade{}, err
	}
	if v.MarketRate, err = s.marketRateFor(ctx, v, current); err != nil {
		return Trade{}, err
	}
	twd, err := s.twd(ctx)
	if err != nil {
		return Trade{}, err
	}
	return Trade{
		TradedOn: v.TradedOn, Side: v.Side, Currency: v.Currency, ForeignAmount: v.ForeignAmount, TWDAmount: v.TWDAmount,
		MarketRate: v.MarketRate, Note: v.Note, TWDMinorUnit: twd.MinorUnit,
	}, nil
}

func (s *Service) marketRateFor(ctx context.Context, v TradeValues, current *Trade) (*rate.MarketRate, error) {
	if current != nil && v.TradedOn.Equal(current.TradedOn) && v.Currency == current.Currency {
		return current.MarketRate, nil
	}
	return rate.MarketRateInTWD(ctx, s.rates, v.Currency, v.TradedOn)
}

func (s *Service) twd(ctx context.Context) (currency.Currency, error) {
	twd, err := s.currencies.Get(ctx, currency.TWD)
	if err != nil {
		return currency.Currency{}, fmt.Errorf("look up %s: %w", currency.TWD, err)
	}
	return twd, nil
}

func (s *Service) validate(ctx context.Context, in TradeInput) (TradeValues, error) {
	twd, err := s.twd(ctx)
	if err != nil {
		return TradeValues{}, err
	}
	date, dateProblem := validate.PastDate(in.TradedOn, taipei.Today(s.now()))
	foreign, currencyProblem, err := s.lookupForeignCurrency(ctx, in.Currency)
	if err != nil {
		return TradeValues{}, err
	}
	foreignAmount, foreignAmountProblem := currency.ParseAmount(in.ForeignAmount, foreign)
	twdAmount, twdAmountProblem := currency.ParseAmount(in.TWDAmount, &twd)

	if err := validate.Fields(map[string]string{
		"traded_on":      dateProblem,
		"side":           sideProblem(Side(in.Side)),
		"currency":       currencyProblem,
		"foreign_amount": foreignAmountProblem,
		"twd_amount":     twdAmountProblem,
		"note":           validate.Note(in.Note),
	}); err != nil {
		return TradeValues{}, err
	}
	return TradeValues{
		TradedOn: date, Side: Side(in.Side), Currency: in.Currency,
		ForeignAmount: foreignAmount, TWDAmount: twdAmount, Note: in.Note,
	}, nil
}

func (s *Service) lookupForeignCurrency(ctx context.Context, code string) (*currency.Currency, string, error) {
	if code == currency.TWD {
		return nil, "must be a foreign currency", nil
	}
	return currency.Lookup(ctx, s.currencies, code)
}

func sideProblem(side Side) string {
	if side != SideBuy && side != SideSell {
		return "must be buy or sell"
	}
	return ""
}
