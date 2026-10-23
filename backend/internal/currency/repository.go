package currency

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbgen"
)

type Repository struct {
	q *dbgen.Queries
}

func NewRepository(db dbgen.DBTX) *Repository {
	return &Repository{q: dbgen.New(db)}
}

func (r *Repository) List(ctx context.Context) ([]Currency, error) {
	rows, err := r.q.ListCurrencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("select currencies: %w", err)
	}
	currencies := make([]Currency, 0, len(rows))
	for _, row := range rows {
		currencies = append(currencies, Currency{Code: row.Code, Name: row.Name, MinorUnit: int(row.MinorUnit)})
	}
	return currencies, nil
}

func (r *Repository) Exists(ctx context.Context, code string) (bool, error) {
	ok, err := r.q.CurrencyExists(ctx, code)
	if err != nil {
		return false, fmt.Errorf("check currency %q: %w", code, err)
	}
	return ok, nil
}

func (r *Repository) Get(ctx context.Context, code string) (Currency, error) {
	row, err := r.q.GetCurrency(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Currency{}, fmt.Errorf("currency %q: %w", code, apperr.ErrNotFound)
	}
	if err != nil {
		return Currency{}, fmt.Errorf("select currency %q: %w", code, err)
	}
	return Currency{Code: row.Code, Name: row.Name, MinorUnit: int(row.MinorUnit)}, nil
}
