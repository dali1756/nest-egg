package portfolio

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/taipei"
)

func wantOpening(t *testing.T, got, want Opening) {
	t.Helper()
	if got.Currency != want.Currency || got.MinorUnit != want.MinorUnit {
		t.Errorf("currency, minor unit = %s %d; want %s %d", got.Currency, got.MinorUnit, want.Currency, want.MinorUnit)
	}
	if !got.OpenedOn.Equal(want.OpenedOn) || got.OpenedOn.Location() != taipei.Location {
		t.Errorf("opened on %v, want %v in Asia/Taipei", got.OpenedOn, want.OpenedOn)
	}
	if !got.Amount.Equal(want.Amount) || !sameDecimal(got.TWDCost, want.TWDCost) {
		t.Errorf("amount, TWD cost = %s %v; want %s %v", got.Amount, got.TWDCost, want.Amount, want.TWDCost)
	}
	if !sameMarketRate(got.MarketRate, want.MarketRate) {
		t.Errorf("market rate = %+v, want %+v", got.MarketRate, want.MarketRate)
	}
	if got.MarketRate != nil && got.MarketRate.Date.Location() != taipei.Location {
		t.Errorf("market rate date location = %v, want Asia/Taipei", got.MarketRate.Date.Location())
	}
}

func TestRepositorySetsAnOpeningThenReplacesIt(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")

	first := OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), MarketRate: marketRate("32", "2026-10-01")}
	created, err := repo.SetOpening(ctx, a, "USD", first)
	if err != nil || !created {
		t.Fatalf("first SetOpening = %v, %v; want created", created, err)
	}
	got, err := repo.Opening(ctx, a, "USD")
	if err != nil {
		t.Fatalf("Opening: %v", err)
	}
	wantOpening(t, got, Opening{Currency: "USD", MinorUnit: 2, OpenedOn: first.OpenedOn, Amount: first.Amount, MarketRate: first.MarketRate})

	second := OpeningValues{OpenedOn: taipeiDate("2026-09-01"), Amount: dec("9000"), TWDCost: decPtr("290000"), MarketRate: marketRate("31.8", "2026-09-01")}
	created, err = repo.SetOpening(ctx, a, "USD", second)
	if err != nil || created {
		t.Fatalf("second SetOpening = %v, %v; want replaced", created, err)
	}
	all, err := repo.Openings(ctx, a)
	if err != nil {
		t.Fatalf("Openings: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("openings = %d, want 1 after replacing", len(all))
	}
	wantOpening(t, all[0], Opening{Currency: "USD", MinorUnit: 2, OpenedOn: second.OpenedOn, Amount: second.Amount, TWDCost: second.TWDCost, MarketRate: second.MarketRate})
}

func TestRepositoryReadsAnOpeningWithoutCostOrRate(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")

	if _, err := repo.SetOpening(ctx, a, "TWD", OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("120000")}); err != nil {
		t.Fatalf("SetOpening: %v", err)
	}
	got, err := repo.Opening(ctx, a, "TWD")
	if err != nil {
		t.Fatalf("Opening: %v", err)
	}
	wantOpening(t, got, Opening{Currency: "TWD", MinorUnit: 2, OpenedOn: taipeiDate("2026-10-01"), Amount: dec("120000")})
}

func TestRepositoryListsOpeningsByCurrency(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	for _, cur := range []string{"USD", "TWD"} {
		if _, err := repo.SetOpening(ctx, a, cur, OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("100")}); err != nil {
			t.Fatalf("SetOpening %s: %v", cur, err)
		}
	}

	all, err := repo.Openings(ctx, a)
	if err != nil {
		t.Fatalf("Openings: %v", err)
	}
	var codes []string
	for _, o := range all {
		codes = append(codes, o.Currency)
	}
	if !slices.Equal(codes, []string{"TWD", "USD"}) {
		t.Errorf("openings = %v, want [TWD USD]", codes)
	}
}

func TestRepositoryDeletesAnOpening(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	if _, err := repo.SetOpening(ctx, a, "USD", OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), TWDCost: decPtr("320000")}); err != nil {
		t.Fatalf("SetOpening: %v", err)
	}

	if err := repo.DeleteOpening(ctx, a, "USD"); err != nil {
		t.Fatalf("DeleteOpening: %v", err)
	}
	if _, err := repo.Opening(ctx, a, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("Opening after delete: err = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteOpening(ctx, a, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("deleting again: err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryCannotReachAnotherUsersOpening(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	b := dbtest.InsertUser(t, pool, "b@example.com")
	if _, err := repo.SetOpening(ctx, b, "USD", OpeningValues{OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), TWDCost: decPtr("320000")}); err != nil {
		t.Fatalf("SetOpening for B: %v", err)
	}

	if _, err := repo.Opening(ctx, a, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("A reads B's opening: err = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteOpening(ctx, a, "USD"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("A deletes B's opening: err = %v, want ErrNotFound", err)
	}
	if all, err := repo.Openings(ctx, a); err != nil || len(all) != 0 {
		t.Errorf("A's openings = %v, %v; want none", all, err)
	}
	if _, err := repo.Opening(ctx, b, "USD"); err != nil {
		t.Errorf("B's opening after A's attempts: %v, want it unchanged", err)
	}
}

func insertCategory(t *testing.T, pool *pgxpool.Pool, userID int64, kind string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO categories (user_id, kind, name, position) VALUES ($1, $2, $2, 0) RETURNING id`, userID, kind).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s category: %v", kind, err)
	}
	return id
}

func insertEntry(t *testing.T, pool *pgxpool.Pool, userID, categoryID int64, date, cur, amount, rate string) int64 {
	t.Helper()
	var marketRate, marketRateDate any
	if rate != "" {
		marketRate, marketRateDate = rate, date
	}
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO ledger_entries (user_id, occurred_on, category_id, currency, amount, market_rate, market_rate_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`, userID, date, categoryID, cur, amount, marketRate, marketRateDate).Scan(&id)
	if err != nil {
		t.Fatalf("insert ledger entry: %v", err)
	}
	return id
}

func insertTrade(t *testing.T, pool *pgxpool.Pool, userID int64, side, date, foreign, twd string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO fx_trades (user_id, traded_on, side, currency, foreign_amount, twd_amount)
		VALUES ($1, $2, $3, 'USD', $4, $5) RETURNING id`, userID, date, side, foreign, twd).Scan(&id)
	if err != nil {
		t.Fatalf("insert fx trade: %v", err)
	}
	return id
}

func TestRepositoryListsOwnLedgerEntriesAndFxTradesAsMovements(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := dbtest.InsertUser(t, pool, "a@example.com")
	b := dbtest.InsertUser(t, pool, "b@example.com")
	aIncome, aExpense := insertCategory(t, pool, a, "income"), insertCategory(t, pool, a, "expense")
	bIncome := insertCategory(t, pool, b, "income")

	salary := insertEntry(t, pool, a, aIncome, "2026-10-05", "USD", "100", "32.2")
	lunch := insertEntry(t, pool, a, aExpense, "2026-10-06", "TWD", "120", "")
	buy := insertTrade(t, pool, a, "buy", "2026-10-07", "1000", "32420")
	sell := insertTrade(t, pool, a, "sell", "2026-10-08", "500", "16050")
	insertEntry(t, pool, b, bIncome, "2026-10-05", "USD", "999", "32.2")
	insertTrade(t, pool, b, "buy", "2026-10-07", "999", "32000")

	got, err := repo.Movements(ctx, a)
	if err != nil {
		t.Fatalf("Movements: %v", err)
	}
	slices.SortFunc(got, func(x, y Movement) int { return cmp.Compare(x.Date.Unix(), y.Date.Unix()) })
	want := []Movement{
		{Source: SourceLedger, ID: salary, Date: taipeiDate("2026-10-05"), Kind: KindIncome, Currency: "USD", Amount: dec("100"), MarketRate: decPtr("32.2")},
		{Source: SourceLedger, ID: lunch, Date: taipeiDate("2026-10-06"), Kind: KindExpense, Currency: "TWD", Amount: dec("120")},
		{Source: SourceFx, ID: buy, Date: taipeiDate("2026-10-07"), Kind: KindBuy, Currency: "USD", Amount: dec("1000"), TWDAmount: dec("32420")},
		{Source: SourceFx, ID: sell, Date: taipeiDate("2026-10-08"), Kind: KindSell, Currency: "USD", Amount: dec("500"), TWDAmount: dec("16050")},
	}
	if len(got) != len(want) {
		t.Fatalf("movements = %+v, want only A's %d", got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Source != w.Source || g.ID != w.ID || g.Kind != w.Kind || g.Currency != w.Currency {
			t.Errorf("movement %d = %s %d %s %s; want %s %d %s %s", i, g.Source, g.ID, g.Kind, g.Currency, w.Source, w.ID, w.Kind, w.Currency)
		}
		if !g.Date.Equal(w.Date) || g.Date.Location() != taipei.Location {
			t.Errorf("movement %d date = %v, want %v in Asia/Taipei", i, g.Date, w.Date)
		}
		if !g.Amount.Equal(w.Amount) || !g.TWDAmount.Equal(w.TWDAmount) || !sameDecimal(g.MarketRate, w.MarketRate) {
			t.Errorf("movement %d amounts = %s %s %v; want %s %s %v", i, g.Amount, g.TWDAmount, g.MarketRate, w.Amount, w.TWDAmount, w.MarketRate)
		}
		if g.CreatedAt.IsZero() {
			t.Errorf("movement %d has no created_at", i)
		}
	}
}
