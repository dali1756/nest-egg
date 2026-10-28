package fx

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

func (r *Repository) CreateTrade(ctx context.Context, userID int64, v TradeValues) (int64, error) {
	marketRate, marketRateDate := v.MarketRate.Columns()
	id, err := r.q.CreateFxTrade(ctx, dbgen.CreateFxTradeParams{
		UserID: userID, TradedOn: v.TradedOn, Side: string(v.Side), Currency: v.Currency,
		ForeignAmount: v.ForeignAmount, TwdAmount: v.TWDAmount,
		MarketRate: marketRate, MarketRateDate: marketRateDate, Note: v.Note,
	})
	if err != nil {
		return 0, fmt.Errorf("insert fx trade: %w", err)
	}
	return id, nil
}

func (r *Repository) Trade(ctx context.Context, userID, id int64) (Trade, error) {
	row, err := r.q.GetFxTrade(ctx, dbgen.GetFxTradeParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Trade{}, fmt.Errorf("fx trade %d: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return Trade{}, fmt.Errorf("select fx trade %d: %w", id, err)
	}
	return tradeFromRow(row), nil
}

func (r *Repository) UpdateTrade(ctx context.Context, userID, id int64, v TradeValues) error {
	marketRate, marketRateDate := v.MarketRate.Columns()
	n, err := r.q.UpdateFxTrade(ctx, dbgen.UpdateFxTradeParams{
		ID: id, UserID: userID, TradedOn: v.TradedOn, Side: string(v.Side), Currency: v.Currency,
		ForeignAmount: v.ForeignAmount, TwdAmount: v.TWDAmount,
		MarketRate: marketRate, MarketRateDate: marketRateDate, Note: v.Note,
	})
	if err != nil {
		return fmt.Errorf("update fx trade %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("fx trade %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}

func (r *Repository) DeleteTrade(ctx context.Context, userID, id int64) error {
	n, err := r.q.DeleteFxTrade(ctx, dbgen.DeleteFxTradeParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete fx trade %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("fx trade %d: %w", id, apperr.ErrNotFound)
	}
	return nil
}

func (r *Repository) Trades(ctx context.Context, userID int64, from, to time.Time) ([]Trade, error) {
	rows, err := r.q.ListFxTrades(ctx, dbgen.ListFxTradesParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, fmt.Errorf("select fx trades: %w", err)
	}
	trades := make([]Trade, 0, len(rows))
	for _, row := range rows {
		// ListFxTrades 與 GetFxTrade 查詢的欄位相同，sqlc 產生的兩個 struct 可以直接轉換
		trades = append(trades, tradeFromRow(dbgen.GetFxTradeRow(row)))
	}
	return trades, nil
}

func tradeFromRow(row dbgen.GetFxTradeRow) Trade {
	return Trade{
		ID:            row.ID,
		TradedOn:      taipei.FromDBDate(row.TradedOn),
		Side:          Side(row.Side),
		Currency:      row.Currency,
		MinorUnit:     int(row.MinorUnit),
		ForeignAmount: row.ForeignAmount,
		TWDAmount:     row.TwdAmount,
		MarketRate:    rate.MarketRateFromColumns(row.MarketRate, row.MarketRateDate),
		Note:          row.Note,
	}
}
