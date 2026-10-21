package rate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbgen"
	"nest-egg/backend/internal/platform/taipei"
)

type Repository struct {
	q *dbgen.Queries
}

func NewRepository(db dbgen.DBTX) *Repository {
	return &Repository{q: dbgen.New(db)}
}

type UpsertResult struct {
	Inserted, Updated int64
}

func (r *Repository) Upsert(ctx context.Context, source string, rates []Rate) (UpsertResult, error) {
	arg := dbgen.UpsertRatesParams{Source: source}
	for _, rt := range rates {
		arg.Bases = append(arg.Bases, rt.Base)
		arg.Quotes = append(arg.Quotes, rt.Quote)
		arg.Dates = append(arg.Dates, rt.Date)
		arg.Rates = append(arg.Rates, rt.Value)
	}
	row, err := r.q.UpsertRates(ctx, arg)
	if err != nil {
		return UpsertResult{}, fmt.Errorf("upsert rate_snapshots: %w", err)
	}
	return UpsertResult{Inserted: row.Inserted, Updated: row.Updated}, nil
}

func (r *Repository) On(ctx context.Context, base, quote string, date time.Time) (Snapshot, error) {
	row, err := r.q.RateOnOrBefore(ctx, dbgen.RateOnOrBeforeParams{Base: base, Quote: quote, OnDate: date})
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, fmt.Errorf("rate %s/%s on %s: %w", base, quote, date.Format(time.DateOnly), apperr.ErrNotFound)
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("select rate_snapshots: %w", err)
	}
	return Snapshot{
		Rate:   Rate{Base: base, Quote: quote, Date: taipei.FromDBDate(row.RateDate), Value: row.Rate},
		Source: row.Source,
	}, nil
}
