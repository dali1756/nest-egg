package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbgen"
	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/rate"
)

type Repository struct {
	q *dbgen.Queries
}

func NewRepository(db dbgen.DBTX) *Repository {
	return &Repository{q: dbgen.New(db)}
}

func (r *Repository) SeedCategories(ctx context.Context, userID int64, categories []Category) error {
	arg := dbgen.SeedCategoriesParams{UserID: userID}
	for i, c := range categories {
		arg.Kinds = append(arg.Kinds, string(c.Kind))
		arg.Names = append(arg.Names, c.Name)
		arg.Positions = append(arg.Positions, int16(i+1))
	}
	if err := r.q.SeedCategories(ctx, arg); err != nil {
		return fmt.Errorf("seed categories: %w", err)
	}
	return nil
}

func (r *Repository) Categories(ctx context.Context, userID int64) ([]Category, error) {
	rows, err := r.q.ListCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select categories: %w", err)
	}
	categories := make([]Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, Category{ID: row.ID, Kind: Kind(row.Kind), Name: row.Name})
	}
	return categories, nil
}

func (r *Repository) Category(ctx context.Context, userID, id int64) (Category, error) {
	row, err := r.q.GetCategory(ctx, dbgen.GetCategoryParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, fmt.Errorf("category %d: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return Category{}, fmt.Errorf("select category %d: %w", id, err)
	}
	return Category{ID: row.ID, Kind: Kind(row.Kind), Name: row.Name}, nil
}

func (r *Repository) CreateEntry(ctx context.Context, userID int64, v EntryValues) (int64, error) {
	marketRate, marketRateDate := v.MarketRate.Columns()
	id, err := r.q.CreateEntry(ctx, dbgen.CreateEntryParams{
		UserID: userID, OccurredOn: v.OccurredOn, CategoryID: v.CategoryID, Currency: v.Currency,
		Amount: v.Amount, MarketRate: marketRate, MarketRateDate: marketRateDate, Note: v.Note,
	})
	if err != nil {
		return 0, fmt.Errorf("insert ledger entry: %w", err)
	}
	return id, nil
}

func (r *Repository) Entry(ctx context.Context, userID, id int64) (Entry, error) {
	row, err := r.q.GetEntry(ctx, dbgen.GetEntryParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, fmt.Errorf("ledger entry %d: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return Entry{}, fmt.Errorf("select ledger entry %d: %w", id, err)
	}
	return entryFromRow(row), nil
}

func (r *Repository) UpdateEntry(ctx context.Context, userID, id int64, v EntryValues) error {
	marketRate, marketRateDate := v.MarketRate.Columns()
	n, err := r.q.UpdateEntry(ctx, dbgen.UpdateEntryParams{
		ID: id, UserID: userID, OccurredOn: v.OccurredOn, CategoryID: v.CategoryID, Currency: v.Currency,
		Amount: v.Amount, MarketRate: marketRate, MarketRateDate: marketRateDate, Note: v.Note,
	})
	if err != nil {
		return fmt.Errorf("update ledger entry %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("ledger entry %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}

func (r *Repository) DeleteEntry(ctx context.Context, userID, id int64) error {
	n, err := r.q.DeleteEntry(ctx, dbgen.DeleteEntryParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete ledger entry %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("ledger entry %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}

func entryFromRow(row dbgen.GetEntryRow) Entry {
	e := Entry{
		ID:         row.ID,
		OccurredOn: taipei.FromDBDate(row.OccurredOn),
		Category:   Category{ID: row.CategoryID, Kind: Kind(row.CategoryKind), Name: row.CategoryName},
		Currency:   row.Currency,
		MinorUnit:  int(row.MinorUnit),
		Amount:     row.Amount,
		Note:       row.Note,
	}
	e.MarketRate = rate.MarketRateFromColumns(row.MarketRate, row.MarketRateDate)
	return e
}

func (r *Repository) Entries(ctx context.Context, userID int64, from, to time.Time) ([]Entry, error) {
	rows, err := r.q.ListEntries(ctx, dbgen.ListEntriesParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, fmt.Errorf("select ledger entries: %w", err)
	}
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		// ListEntries 與 GetEntry 查詢的欄位相同，sqlc 產生的兩個 struct 可以直接轉換
		entries = append(entries, entryFromRow(dbgen.GetEntryRow(row)))
	}
	return entries, nil
}

func (r *Repository) Totals(ctx context.Context, userID int64, from, to time.Time) ([]Total, error) {
	rows, err := r.q.SumEntries(ctx, dbgen.SumEntriesParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, fmt.Errorf("sum ledger entries: %w", err)
	}
	totals := make([]Total, 0, len(rows))
	for _, row := range rows {
		totals = append(totals, Total{Currency: row.Currency, MinorUnit: int(row.MinorUnit), Income: row.Income, Expense: row.Expense})
	}
	return totals, nil
}
