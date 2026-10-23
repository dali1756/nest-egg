package ledger

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/db/dbtest"
	"nest-egg/backend/internal/platform/taipei"
)

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	return id
}

type kindName struct {
	kind Kind
	name string
}

func kindNames(categories []Category) []kindName {
	out := make([]kindName, 0, len(categories))
	for _, c := range categories {
		out = append(out, kindName{c.Kind, c.Name})
	}
	return out
}

var testDefaults = []Category{
	{Kind: KindExpense, Name: "飲食"},
	{Kind: KindIncome, Name: "薪水"},
	{Kind: KindExpense, Name: "生活用品"},
}

func TestRepositorySeedsCategoriesOnlyWhenUserHasNone(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")

	if err := repo.SeedCategories(ctx, a, testDefaults); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := repo.SeedCategories(ctx, a, []Category{{Kind: KindExpense, Name: "其他"}}); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	got, err := repo.Categories(ctx, a)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	want := []kindName{{KindExpense, "飲食"}, {KindIncome, "薪水"}, {KindExpense, "生活用品"}}
	if !slices.Equal(kindNames(got), want) {
		t.Errorf("categories = %v, want %v in the seeded order", kindNames(got), want)
	}
}

func TestRepositorySeedCategoriesWhileAnotherSeedIsUncommitted(t *testing.T) {
	pool := dbtest.Migrated(t)
	ctx := t.Context()
	a := insertUser(t, pool, "a@example.com")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := NewRepository(tx).SeedCategories(ctx, a, testDefaults); err != nil {
		t.Fatalf("seed in the first transaction: %v", err)
	}

	second := make(chan error, 1)
	go func() { second <- NewRepository(pool).SeedCategories(ctx, a, testDefaults) }()
	waitForBlockedQuery(t, pool)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit the first transaction: %v", err)
	}
	if err := <-second; err != nil {
		t.Errorf("second seed: %v, want it to skip the categories the first one created", err)
	}

	got, err := NewRepository(pool).Categories(ctx, a)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	if len(got) != len(testDefaults) {
		t.Errorf("%d categories, want %d", len(got), len(testDefaults))
	}
}

func waitForBlockedQuery(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := pool.QueryRow(t.Context(), `SELECT EXISTS (
			SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
			WHERE NOT l.granted AND a.query LIKE '%INSERT INTO categories%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the second seed never waited on the first transaction")
}

func TestRepositoryCategoriesBelongToEachUser(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	ctx := t.Context()
	a, b := insertUser(t, pool, "a@example.com"), insertUser(t, pool, "b@example.com")
	for _, user := range []int64{a, b} {
		if err := repo.SeedCategories(ctx, user, testDefaults); err != nil {
			t.Fatalf("seed user %d: %v", user, err)
		}
	}

	aCategories, err := repo.Categories(ctx, a)
	if err != nil {
		t.Fatalf("Categories(A): %v", err)
	}
	bCategories, err := repo.Categories(ctx, b)
	if err != nil {
		t.Fatalf("Categories(B): %v", err)
	}
	if len(aCategories) != len(testDefaults) || len(bCategories) != len(testDefaults) {
		t.Fatalf("A has %d, B has %d categories; want %d each", len(aCategories), len(bCategories), len(testDefaults))
	}
	for _, ac := range aCategories {
		for _, bc := range bCategories {
			if ac.ID == bc.ID {
				t.Errorf("A and B share category id %d", ac.ID)
			}
		}
	}
}

func seededUser(t *testing.T, pool *pgxpool.Pool, email string) (int64, []Category) {
	t.Helper()
	repo := NewRepository(pool)
	id := insertUser(t, pool, email)
	if err := repo.SeedCategories(t.Context(), id, defaultCategories); err != nil {
		t.Fatalf("seed categories for %s: %v", email, err)
	}
	categories, err := repo.Categories(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return id, categories
}

func mustCreate(t *testing.T, repo *Repository, userID int64, v EntryValues) int64 {
	t.Helper()
	id, err := repo.CreateEntry(t.Context(), userID, v)
	if err != nil {
		t.Fatalf("CreateEntry(%+v): %v", v, err)
	}
	return id
}

func TestRepositoryCategoryOnlyFindsTheUsersOwn(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, aCategories := seededUser(t, pool, "a@example.com")
	b, _ := seededUser(t, pool, "b@example.com")

	got, err := repo.Category(t.Context(), a, aCategories[0].ID)
	if err != nil || got != aCategories[0] {
		t.Errorf("A's own category = %+v, %v; want %+v", got, err, aCategories[0])
	}
	if _, err := repo.Category(t.Context(), b, aCategories[0].ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B asking for A's category: error = %v, want apperr.ErrNotFound", err)
	}
}

func TestRepositoryCreateAndGetEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, categories := seededUser(t, pool, "a@example.com")
	food, salary := categories[0], categories[4]

	twd := mustCreate(t, repo, a, EntryValues{
		OccurredOn: taipeiDate("2026-10-23"), CategoryID: food.ID, Currency: "TWD",
		Amount: decimal.RequireFromString("120"), Note: "午餐",
	})
	usd := mustCreate(t, repo, a, EntryValues{
		OccurredOn: taipeiDate("2026-10-24"), CategoryID: salary.ID, Currency: "USD", Amount: decimal.RequireFromString("12.34"),
		MarketRate: &MarketRate{Value: decimal.RequireFromString("32.508"), Date: taipeiDate("2026-10-23")},
	})

	got, err := repo.Entry(t.Context(), a, twd)
	if err != nil {
		t.Fatalf("Entry(TWD): %v", err)
	}
	if got.ID != twd || !got.OccurredOn.Equal(taipeiDate("2026-10-23")) || got.OccurredOn.Location() != taipei.Location ||
		got.Category != food || got.Currency != "TWD" || got.MinorUnit != 2 ||
		!got.Amount.Equal(decimal.RequireFromString("120")) || got.MarketRate != nil || got.Note != "午餐" {
		t.Errorf("TWD entry = %+v, want the 2026-10-23 TWD 120 飲食 entry", got)
	}

	got, err = repo.Entry(t.Context(), a, usd)
	if err != nil {
		t.Fatalf("Entry(USD): %v", err)
	}
	if got.Category != salary || got.Currency != "USD" || !got.Amount.Equal(decimal.RequireFromString("12.34")) || got.Note != "" {
		t.Errorf("USD entry = %+v, want the USD 12.34 薪水 entry", got)
	}
	wantMarketRate(t, got.MarketRate, "32.508", "2026-10-23")
	if got.MarketRate != nil && got.MarketRate.Date.Location() != taipei.Location {
		t.Errorf("market rate date location = %v, want Asia/Taipei", got.MarketRate.Date.Location())
	}
}

func TestRepositoryEntryOnlyFindsTheUsersOwn(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, categories := seededUser(t, pool, "a@example.com")
	b, _ := seededUser(t, pool, "b@example.com")
	id := mustCreate(t, repo, a, EntryValues{OccurredOn: taipeiDate("2026-10-23"), CategoryID: categories[0].ID, Currency: "TWD", Amount: decimal.RequireFromString("120")})

	for name, tc := range map[string]struct{ user, id int64 }{"B reads A's entry": {b, id}, "missing id": {a, id + 1000}} {
		if _, err := repo.Entry(t.Context(), tc.user, tc.id); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: error = %v, want apperr.ErrNotFound", name, err)
		}
	}
}

func TestRepositoryUpdateEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, aCategories := seededUser(t, pool, "a@example.com")
	b, bCategories := seededUser(t, pool, "b@example.com")
	original := EntryValues{OccurredOn: taipeiDate("2026-10-23"), CategoryID: aCategories[0].ID, Currency: "TWD", Amount: decimal.RequireFromString("120")}
	id := mustCreate(t, repo, a, original)

	stolen := EntryValues{OccurredOn: taipeiDate("2026-10-01"), CategoryID: bCategories[0].ID, Currency: "TWD", Amount: decimal.RequireFromString("999")}
	if err := repo.UpdateEntry(t.Context(), b, id, stolen); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B updates A's entry: error = %v, want apperr.ErrNotFound", err)
	}
	if err := repo.UpdateEntry(t.Context(), a, id+1000, original); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update a missing id: error = %v, want apperr.ErrNotFound", err)
	}
	if got, _ := repo.Entry(t.Context(), a, id); !got.Amount.Equal(decimal.RequireFromString("120")) {
		t.Errorf("A's amount after B's update = %s, want 120", got.Amount)
	}

	changed := EntryValues{
		OccurredOn: taipeiDate("2026-10-20"), CategoryID: aCategories[4].ID, Currency: "USD", Amount: decimal.RequireFromString("50"),
		MarketRate: &MarketRate{Value: decimal.RequireFromString("32.617"), Date: taipeiDate("2026-10-20")}, Note: "股息",
	}
	if err := repo.UpdateEntry(t.Context(), a, id, changed); err != nil {
		t.Fatalf("A updates own entry: %v", err)
	}
	got, err := repo.Entry(t.Context(), a, id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OccurredOn.Equal(taipeiDate("2026-10-20")) || got.Category != aCategories[4] || got.Currency != "USD" ||
		!got.Amount.Equal(decimal.RequireFromString("50")) || got.Note != "股息" {
		t.Errorf("updated entry = %+v, want the 2026-10-20 USD 50 薪水 entry", got)
	}
	wantMarketRate(t, got.MarketRate, "32.617", "2026-10-20")
}

func TestRepositoryDeleteEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, categories := seededUser(t, pool, "a@example.com")
	b, _ := seededUser(t, pool, "b@example.com")
	id := mustCreate(t, repo, a, EntryValues{OccurredOn: taipeiDate("2026-10-23"), CategoryID: categories[0].ID, Currency: "TWD", Amount: decimal.RequireFromString("120")})

	if err := repo.DeleteEntry(t.Context(), b, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B deletes A's entry: error = %v, want apperr.ErrNotFound", err)
	}
	if _, err := repo.Entry(t.Context(), a, id); err != nil {
		t.Errorf("A's entry after B's delete: %v, want it to still exist", err)
	}
	if err := repo.DeleteEntry(t.Context(), a, id); err != nil {
		t.Fatalf("A deletes own entry: %v", err)
	}
	if _, err := repo.Entry(t.Context(), a, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("entry after delete: error = %v, want apperr.ErrNotFound", err)
	}
	if err := repo.DeleteEntry(t.Context(), a, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("deleting twice: error = %v, want apperr.ErrNotFound", err)
	}
}

func TestRepositoryEntriesInAMonth(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, aCategories := seededUser(t, pool, "a@example.com")
	b, bCategories := seededUser(t, pool, "b@example.com")
	create := func(user int64, category Category, date string) int64 {
		return mustCreate(t, repo, user, EntryValues{OccurredOn: taipeiDate(date), CategoryID: category.ID, Currency: "TWD", Amount: decimal.RequireFromString("1")})
	}
	create(a, aCategories[0], "2026-09-30")
	firstOct1 := create(a, aCategories[0], "2026-10-01")
	secondOct1 := create(a, aCategories[0], "2026-10-01")
	oct23 := create(a, aCategories[0], "2026-10-23")
	create(a, aCategories[0], "2026-11-01")
	create(b, bCategories[0], "2026-10-15")

	got, err := repo.Entries(t.Context(), a, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	var ids []int64
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	if want := []int64{oct23, secondOct1, firstOct1}; !slices.Equal(ids, want) {
		t.Errorf("October entries = %v, want %v (newest date first, later entries first within a day)", ids, want)
	}
}

func TestRepositoryTotals(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, aCategories := seededUser(t, pool, "a@example.com")
	b, bCategories := seededUser(t, pool, "b@example.com")
	aFood, aSalary := aCategories[0], aCategories[4]
	create := func(user int64, category Category, date, cur, amount string) {
		mustCreate(t, repo, user, EntryValues{OccurredOn: taipeiDate(date), CategoryID: category.ID, Currency: cur, Amount: decimal.RequireFromString(amount)})
	}
	create(a, aSalary, "2026-10-05", "TWD", "50000")
	create(a, aFood, "2026-10-23", "TWD", "120")
	create(a, aFood, "2026-10-31", "TWD", "35.5")
	create(a, aSalary, "2026-10-10", "USD", "12.34")
	create(a, aFood, "2026-09-30", "TWD", "1000")
	create(a, aFood, "2026-11-01", "TWD", "1000")
	create(b, bCategories[0], "2026-10-15", "TWD", "777")

	got, err := repo.Totals(t.Context(), a, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}

	want := []struct{ currency, income, expense string }{{"TWD", "50000", "155.5"}, {"USD", "12.34", "0"}}
	if len(got) != len(want) {
		t.Fatalf("totals = %+v, want %v", got, want)
	}
	for i, w := range want {
		g := got[i]
		if g.Currency != w.currency || g.MinorUnit != 2 || !g.Income.Equal(decimal.RequireFromString(w.income)) || !g.Expense.Equal(decimal.RequireFromString(w.expense)) {
			t.Errorf("totals[%d] = %s income %s expense %s, want %s income %s expense %s", i, g.Currency, g.Income, g.Expense, w.currency, w.income, w.expense)
		}
	}
}

func TestRepositoryEmptyMonth(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, _ := seededUser(t, pool, "a@example.com")

	entries, err := repo.Entries(t.Context(), a, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil || len(entries) != 0 {
		t.Errorf("Entries = %v, %v; want none", entries, err)
	}
	totals, err := repo.Totals(t.Context(), a, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil || len(totals) != 0 {
		t.Errorf("Totals = %v, %v; want none", totals, err)
	}
}
