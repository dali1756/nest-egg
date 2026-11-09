package portfolio

import (
	"context"
	"errors"
	"fmt"

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

func (r *Repository) SetOpening(ctx context.Context, userID int64, cur string, v OpeningValues) (bool, error) {
	marketRate, marketRateDate := v.MarketRate.Columns()
	created, err := r.q.SetOpeningBalance(ctx, dbgen.SetOpeningBalanceParams{
		UserID: userID, Currency: cur, OpenedOn: v.OpenedOn, Amount: v.Amount, TwdCost: v.TWDCost,
		MarketRate: marketRate, MarketRateDate: marketRateDate,
	})
	if err != nil {
		return false, fmt.Errorf("upsert %s opening balance: %w", cur, err)
	}
	return created, nil
}

func (r *Repository) Opening(ctx context.Context, userID int64, cur string) (Opening, error) {
	row, err := r.q.GetOpeningBalance(ctx, dbgen.GetOpeningBalanceParams{UserID: userID, Currency: cur})
	if errors.Is(err, pgx.ErrNoRows) {
		return Opening{}, fmt.Errorf("%s opening balance: %w", cur, apperr.ErrNotFound)
	}
	if err != nil {
		return Opening{}, fmt.Errorf("select %s opening balance: %w", cur, err)
	}
	return openingFromRow(row), nil
}

func (r *Repository) Openings(ctx context.Context, userID int64) ([]Opening, error) {
	rows, err := r.q.ListOpeningBalances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select opening balances: %w", err)
	}
	openings := make([]Opening, 0, len(rows))
	for _, row := range rows {
		// ListOpeningBalances 與 GetOpeningBalance 查詢的欄位相同，sqlc 產生的兩個 struct 可以直接轉換
		openings = append(openings, openingFromRow(dbgen.GetOpeningBalanceRow(row)))
	}
	return openings, nil
}

func (r *Repository) DeleteOpening(ctx context.Context, userID int64, cur string) error {
	n, err := r.q.DeleteOpeningBalance(ctx, dbgen.DeleteOpeningBalanceParams{UserID: userID, Currency: cur})
	if err != nil {
		return fmt.Errorf("delete %s opening balance: %w", cur, err)
	}
	if n == 0 {
		return fmt.Errorf("%s opening balance: %w", cur, apperr.ErrNotFound)
	}
	return nil
}

func (r *Repository) Movements(ctx context.Context, userID int64) ([]Movement, error) {
	rows, err := r.q.ListBalanceMovements(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select balance movements: %w", err)
	}
	movements := make([]Movement, 0, len(rows))
	for _, row := range rows {
		movements = append(movements, Movement{
			Source:     Source(row.Source),
			ID:         row.ID,
			Date:       taipei.FromDBDate(row.OccurredOn),
			CreatedAt:  row.CreatedAt,
			Kind:       Kind(row.Kind),
			Currency:   row.Currency,
			Amount:     row.Amount,
			TWDAmount:  row.TwdAmount,
			MarketRate: row.MarketRate,
		})
	}
	return movements, nil
}

func openingFromRow(row dbgen.GetOpeningBalanceRow) Opening {
	return Opening{
		Currency:   row.Currency,
		MinorUnit:  int(row.MinorUnit),
		OpenedOn:   taipei.FromDBDate(row.OpenedOn),
		Amount:     row.Amount,
		TWDCost:    row.TwdCost,
		MarketRate: rate.MarketRateFromColumns(row.MarketRate, row.MarketRateDate),
	}
}
