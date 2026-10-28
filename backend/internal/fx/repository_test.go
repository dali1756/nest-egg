package fx

import (
	"errors"
	"slices"
	"testing"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/taipei"
	"nest-egg/backend/internal/rate"
)

func usdBuy(date string, foreign, twd string) TradeValues {
	return TradeValues{
		TradedOn: taipeiDate(date), Side: SideBuy, Currency: "USD",
		ForeignAmount: decimal.RequireFromString(foreign), TWDAmount: decimal.RequireFromString(twd),
	}
}

func tradeIDs(trades []Trade) []int64 {
	ids := make([]int64, 0, len(trades))
	for _, t := range trades {
		ids = append(ids, t.ID)
	}
	return ids
}

func TestRepositoryStoresAndReadsATrade(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")

	for name, v := range map[string]TradeValues{
		"with a market rate": {
			TradedOn: taipeiDate("2026-10-17"), Side: SideBuy, Currency: "USD",
			ForeignAmount: decimal.RequireFromString("309.35"), TWDAmount: decimal.RequireFromString("10000"),
			MarketRate: &rate.MarketRate{Value: decimal.RequireFromString("32.338"), Date: taipeiDate("2026-10-16")},
			Note:       "10 月定期換匯",
		},
		"without a market rate": {
			TradedOn: taipeiDate("2008-01-01"), Side: SideSell, Currency: "USD",
			ForeignAmount: decimal.RequireFromString("500"), TWDAmount: decimal.RequireFromString("16050"),
		},
	} {
		id, err := repo.CreateTrade(ctx, a, v)
		if err != nil {
			t.Fatalf("%s: CreateTrade: %v", name, err)
		}
		got, err := repo.Trade(ctx, a, id)
		if err != nil {
			t.Fatalf("%s: Trade: %v", name, err)
		}
		wantTrade(t, got, Trade{
			ID: id, TradedOn: v.TradedOn, Side: v.Side, Currency: "USD", MinorUnit: 2,
			ForeignAmount: v.ForeignAmount, TWDAmount: v.TWDAmount, MarketRate: v.MarketRate, Note: v.Note,
		})
		if got.MarketRate != nil && got.MarketRate.Date.Location() != taipei.Location {
			t.Errorf("%s: market rate date location = %v, want Asia/Taipei", name, got.MarketRate.Date.Location())
		}
	}
}

func TestRepositoryUpdatesATrade(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	id, err := repo.CreateTrade(ctx, a, usdBuy("2026-10-27", "1000", "32420"))
	if err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	changed := TradeValues{
		TradedOn: taipeiDate("2026-10-15"), Side: SideSell, Currency: "USD",
		ForeignAmount: decimal.RequireFromString("500"), TWDAmount: decimal.RequireFromString("16050"),
		MarketRate: &rate.MarketRate{Value: decimal.RequireFromString("32.25"), Date: taipeiDate("2026-10-15")},
		Note:       "換回台幣",
	}
	if err := repo.UpdateTrade(ctx, a, id, changed); err != nil {
		t.Fatalf("UpdateTrade: %v", err)
	}
	got, err := repo.Trade(ctx, a, id)
	if err != nil {
		t.Fatalf("Trade: %v", err)
	}
	wantTrade(t, got, Trade{
		ID: id, TradedOn: changed.TradedOn, Side: SideSell, Currency: "USD", MinorUnit: 2,
		ForeignAmount: changed.ForeignAmount, TWDAmount: changed.TWDAmount, MarketRate: changed.MarketRate, Note: "換回台幣",
	})
}

func TestRepositoryListsTradesInAPeriodNewestFirst(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	create := func(date string) int64 {
		t.Helper()
		id, err := repo.CreateTrade(ctx, a, usdBuy(date, "1000", "32420"))
		if err != nil {
			t.Fatalf("CreateTrade %s: %v", date, err)
		}
		return id
	}
	create("2026-09-30")
	mid := create("2026-10-15")
	firstOfDay := create("2026-10-27")
	secondOfDay := create("2026-10-27")
	create("2026-11-01")

	got, err := repo.Trades(ctx, a, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil {
		t.Fatalf("Trades: %v", err)
	}
	if want := []int64{secondOfDay, firstOfDay, mid}; !slices.Equal(tradeIDs(got), want) {
		t.Errorf("trade ids = %v, want %v", tradeIDs(got), want)
	}
}

func TestRepositoryKeepsUsersApart(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a, b := dbtest.InsertUser(t, pool, "a@example.com"), dbtest.InsertUser(t, pool, "b@example.com")
	original := usdBuy("2026-10-27", "1000", "32420")
	id, err := repo.CreateTrade(ctx, a, original)
	if err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	if _, err := repo.Trade(ctx, b, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B reads A's trade: %v, want not found", err)
	}
	if err := repo.UpdateTrade(ctx, b, id, usdBuy("2026-10-01", "1", "30")); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B updates A's trade: %v, want not found", err)
	}
	if err := repo.DeleteTrade(ctx, b, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B deletes A's trade: %v, want not found", err)
	}
	listed, err := repo.Trades(ctx, b, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil || len(listed) != 0 {
		t.Errorf("B lists %v, %v; want nothing", tradeIDs(listed), err)
	}

	got, err := repo.Trade(ctx, a, id)
	if err != nil {
		t.Fatalf("A reads own trade: %v", err)
	}
	wantTrade(t, got, Trade{
		ID: id, TradedOn: original.TradedOn, Side: SideBuy, Currency: "USD", MinorUnit: 2,
		ForeignAmount: original.ForeignAmount, TWDAmount: original.TWDAmount,
	})
}

func TestRepositoryDeletesATrade(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	id, err := repo.CreateTrade(ctx, a, usdBuy("2026-10-27", "1000", "32420"))
	if err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	if err := repo.DeleteTrade(ctx, a, id); err != nil {
		t.Fatalf("DeleteTrade: %v", err)
	}
	if _, err := repo.Trade(ctx, a, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("Trade after delete: %v, want not found", err)
	}
	if err := repo.DeleteTrade(ctx, a, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("deleting twice: %v, want not found", err)
	}
}
