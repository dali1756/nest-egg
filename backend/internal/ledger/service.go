package ledger

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

type Kind string

const (
	KindIncome  Kind = "income"
	KindExpense Kind = "expense"
)

type Category struct {
	ID   int64
	Kind Kind
	Name string
}

var defaultCategories = []Category{
	{Kind: KindExpense, Name: "飲食"},
	{Kind: KindExpense, Name: "生活用品"},
	{Kind: KindExpense, Name: "美容美髮"},
	{Kind: KindExpense, Name: "其他"},
	{Kind: KindIncome, Name: "薪水"},
	{Kind: KindIncome, Name: "獎金"},
	{Kind: KindIncome, Name: "股息"},
	{Kind: KindIncome, Name: "其他"},
}

type Entry struct {
	ID         int64
	OccurredOn time.Time
	Category   Category
	Currency   string
	MinorUnit  int
	Amount     decimal.Decimal
	MarketRate *rate.MarketRate
	Note       string
}

type Total struct {
	Currency  string
	MinorUnit int
	Income    decimal.Decimal
	Expense   decimal.Decimal
}

func (t Total) Net() decimal.Decimal {
	return t.Income.Sub(t.Expense)
}

type MonthLedger struct {
	Month   string
	Entries []Entry
	Totals  []Total
}

type EntryInput struct {
	OccurredOn string
	CategoryID int64
	Currency   string
	Amount     string
	Note       string
}

type EntryValues struct {
	OccurredOn time.Time
	CategoryID int64
	Currency   string
	Amount     decimal.Decimal
	MarketRate *rate.MarketRate
	Note       string
}

type Store interface {
	SeedCategories(ctx context.Context, userID int64, categories []Category) error
	Categories(ctx context.Context, userID int64) ([]Category, error)
	Category(ctx context.Context, userID, id int64) (Category, error)
	CreateEntry(ctx context.Context, userID int64, v EntryValues) (int64, error)
	Entry(ctx context.Context, userID, id int64) (Entry, error)
	UpdateEntry(ctx context.Context, userID, id int64, v EntryValues) error
	// Entries 與 Totals 的期間是 [from, to)。
	Entries(ctx context.Context, userID int64, from, to time.Time) ([]Entry, error)
	Totals(ctx context.Context, userID int64, from, to time.Time) ([]Total, error)
	DeleteEntry(ctx context.Context, userID, id int64) error
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

func (s *Service) Categories(ctx context.Context, userID int64) ([]Category, error) {
	if err := s.store.SeedCategories(ctx, userID, defaultCategories); err != nil {
		return nil, fmt.Errorf("seed default categories: %w", err)
	}
	return s.store.Categories(ctx, userID)
}

func (s *Service) CreateEntry(ctx context.Context, userID int64, in EntryInput) (Entry, error) {
	v, err := s.validate(ctx, userID, in)
	if err != nil {
		return Entry{}, err
	}
	if v.MarketRate, err = rate.MarketRateInTWD(ctx, s.rates, v.Currency, v.OccurredOn); err != nil {
		return Entry{}, err
	}
	id, err := s.store.CreateEntry(ctx, userID, v)
	if err != nil {
		return Entry{}, fmt.Errorf("create entry: %w", err)
	}
	return s.store.Entry(ctx, userID, id)
}

func (s *Service) Entry(ctx context.Context, userID, id int64) (Entry, error) {
	return s.store.Entry(ctx, userID, id)
}

func (s *Service) DeleteEntry(ctx context.Context, userID, id int64) error {
	return s.store.DeleteEntry(ctx, userID, id)
}

func (s *Service) UpdateEntry(ctx context.Context, userID, id int64, in EntryInput) (Entry, error) {
	current, err := s.store.Entry(ctx, userID, id)
	if err != nil {
		return Entry{}, err
	}
	v, err := s.validate(ctx, userID, in)
	if err != nil {
		return Entry{}, err
	}
	v.MarketRate = current.MarketRate
	if !v.OccurredOn.Equal(current.OccurredOn) || v.Currency != current.Currency {
		if v.MarketRate, err = rate.MarketRateInTWD(ctx, s.rates, v.Currency, v.OccurredOn); err != nil {
			return Entry{}, err
		}
	}
	if err := s.store.UpdateEntry(ctx, userID, id, v); err != nil {
		return Entry{}, fmt.Errorf("update entry %d: %w", id, err)
	}
	return s.store.Entry(ctx, userID, id)
}

func (s *Service) Month(ctx context.Context, userID int64, month string) (MonthLedger, error) {
	first, err := validate.MonthStart(month, taipei.Today(s.now()))
	if err != nil {
		return MonthLedger{}, err
	}
	next := first.AddDate(0, 1, 0)
	entries, err := s.store.Entries(ctx, userID, first, next)
	if err != nil {
		return MonthLedger{}, err
	}
	totals, err := s.store.Totals(ctx, userID, first, next)
	if err != nil {
		return MonthLedger{}, err
	}
	return MonthLedger{Month: first.Format(validate.MonthLayout), Entries: entries, Totals: totals}, nil
}

func (s *Service) validate(ctx context.Context, userID int64, in EntryInput) (EntryValues, error) {
	date, dateProblem := validate.PastDate(in.OccurredOn, taipei.Today(s.now()))
	categoryProblem, err := s.categoryProblem(ctx, userID, in.CategoryID)
	if err != nil {
		return EntryValues{}, err
	}
	cur, currencyProblem, err := currency.Lookup(ctx, s.currencies, in.Currency)
	if err != nil {
		return EntryValues{}, err
	}
	amount, amountProblem := currency.ParseAmount(in.Amount, cur)

	if err := validate.Fields(map[string]string{
		"occurred_on": dateProblem,
		"category_id": categoryProblem,
		"currency":    currencyProblem,
		"amount":      amountProblem,
		"note":        validate.Note(in.Note),
	}); err != nil {
		return EntryValues{}, err
	}
	return EntryValues{OccurredOn: date, CategoryID: in.CategoryID, Currency: in.Currency, Amount: amount, Note: in.Note}, nil
}

func (s *Service) categoryProblem(ctx context.Context, userID, id int64) (string, error) {
	if id == 0 {
		return "required", nil
	}
	_, err := s.store.Category(ctx, userID, id)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return "unknown category", nil
	case err != nil:
		return "", fmt.Errorf("check category: %w", err)
	}
	return "", nil
}
