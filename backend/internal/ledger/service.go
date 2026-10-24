package ledger

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
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

type MarketRate struct {
	Value decimal.Decimal
	Date  time.Time
}

type Entry struct {
	ID         int64
	OccurredOn time.Time
	Category   Category
	Currency   string
	MinorUnit  int
	Amount     decimal.Decimal
	MarketRate *MarketRate
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
	MarketRate *MarketRate
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

type Currencies interface {
	Get(ctx context.Context, code string) (currency.Currency, error)
}

type MarketRates interface {
	On(ctx context.Context, base, quote string, date time.Time) (rate.Snapshot, error)
}

type Service struct {
	store      Store
	currencies Currencies
	rates      MarketRates
	now        func() time.Time
}

func NewService(store Store, currencies Currencies, rates MarketRates) *Service {
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
	if v.MarketRate, err = s.marketRate(ctx, v.Currency, v.OccurredOn); err != nil {
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
		if v.MarketRate, err = s.marketRate(ctx, v.Currency, v.OccurredOn); err != nil {
			return Entry{}, err
		}
	}
	if err := s.store.UpdateEntry(ctx, userID, id, v); err != nil {
		return Entry{}, fmt.Errorf("update entry %d: %w", id, err)
	}
	return s.store.Entry(ctx, userID, id)
}

func (s *Service) Month(ctx context.Context, userID int64, month string) (MonthLedger, error) {
	first, err := s.firstDayOf(month)
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
	return MonthLedger{Month: first.Format(monthLayout), Entries: entries, Totals: totals}, nil
}

const monthLayout = "2006-01"

func (s *Service) firstDayOf(month string) (time.Time, error) {
	if month == "" {
		today := taipei.Today(s.now())
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, taipei.Location), nil
	}
	first, err := time.ParseInLocation(monthLayout, month, taipei.Location)
	if err != nil {
		return time.Time{}, &apperr.ValidationError{Fields: map[string]string{"month": "must be a month in YYYY-MM format"}}
	}
	return first, nil
}

const baseCurrency = "TWD"

func (s *Service) marketRate(ctx context.Context, cur string, date time.Time) (*MarketRate, error) {
	if cur == baseCurrency {
		return nil, nil
	}
	snap, err := s.rates.On(ctx, cur, baseCurrency, date)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("look up %s market rate: %w", cur, err)
	}
	return &MarketRate{Value: snap.Value, Date: snap.Date}, nil
}

const maxNoteLength = 200

var amountPattern = regexp.MustCompile(`^[0-9]{1,16}(\.[0-9]+)?$`)

func (s *Service) validate(ctx context.Context, userID int64, in EntryInput) (EntryValues, error) {
	date, dateProblem := s.parseOccurredOn(in.OccurredOn)
	categoryProblem, err := s.categoryProblem(ctx, userID, in.CategoryID)
	if err != nil {
		return EntryValues{}, err
	}
	cur, currencyProblem, err := s.lookupCurrency(ctx, in.Currency)
	if err != nil {
		return EntryValues{}, err
	}
	amount, amountProblem := parseAmount(in.Amount, cur)

	fields := nonEmpty(map[string]string{
		"occurred_on": dateProblem,
		"category_id": categoryProblem,
		"currency":    currencyProblem,
		"amount":      amountProblem,
		"note":        noteProblem(in.Note),
	})
	if len(fields) > 0 {
		return EntryValues{}, &apperr.ValidationError{Fields: fields}
	}
	return EntryValues{OccurredOn: date, CategoryID: in.CategoryID, Currency: in.Currency, Amount: amount, Note: in.Note}, nil
}

func nonEmpty(problems map[string]string) map[string]string {
	fields := map[string]string{}
	for field, problem := range problems {
		if problem != "" {
			fields[field] = problem
		}
	}
	return fields
}

func (s *Service) parseOccurredOn(raw string) (time.Time, string) {
	date, err := taipei.ParseDate(raw)
	switch {
	case err != nil:
		return time.Time{}, "must be a date in YYYY-MM-DD format"
	case date.After(taipei.Today(s.now())):
		return time.Time{}, "must not be in the future"
	}
	return date, ""
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

func (s *Service) lookupCurrency(ctx context.Context, code string) (*currency.Currency, string, error) {
	if code == "" {
		return nil, "required", nil
	}
	c, err := s.currencies.Get(ctx, code)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return nil, "unknown currency", nil
	case err != nil:
		return nil, "", fmt.Errorf("check currency: %w", err)
	}
	return &c, "", nil
}

func parseAmount(raw string, cur *currency.Currency) (decimal.Decimal, string) {
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

func noteProblem(note string) string {
	if utf8.RuneCountInString(note) > maxNoteLength {
		return fmt.Sprintf("must be at most %d characters", maxNoteLength)
	}
	return ""
}

func decimalPlaces(amount string) int {
	if i := strings.IndexByte(amount, '.'); i >= 0 {
		return len(amount) - i - 1
	}
	return 0
}
