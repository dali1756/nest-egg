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
	a := dbtest.InsertUser(t, pool, "a@example.com")

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
	a := dbtest.InsertUser(t, pool, "a@example.com")

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
	a, b := dbtest.InsertUser(t, pool, "a@example.com"), dbtest.InsertUser(t, pool, "b@example.com")
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

type seededUser struct {
	id           int64
	food, salary Category
}

func seedUser(t *testing.T, pool *pgxpool.Pool, email string) seededUser {
	t.Helper()
	repo := NewRepository(pool)
	id := dbtest.InsertUser(t, pool, email)
	if err := repo.SeedCategories(t.Context(), id, defaultCategories); err != nil {
		t.Fatalf("seed categories for %s: %v", email, err)
	}
	categories, err := repo.Categories(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return seededUser{
		id:     id,
		food:   findCategory(t, categories, KindExpense, "飲食"),
		salary: findCategory(t, categories, KindIncome, "薪水"),
	}
}

func findCategory(t *testing.T, categories []Category, kind Kind, name string) Category {
	t.Helper()
	for _, c := range categories {
		if c.Kind == kind && c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s category named %s in %v", kind, name, categories)
	return Category{}
}

func mustCreate(t *testing.T, repo *Repository, userID int64, v EntryValues) int64 {
	t.Helper()
	id, err := repo.CreateEntry(t.Context(), userID, v)
	if err != nil {
		t.Fatalf("CreateEntry(%+v): %v", v, err)
	}
	return id
}

func twdEntry(category Category, date, amount string) EntryValues {
	return EntryValues{OccurredOn: taipeiDate(date), CategoryID: category.ID, Currency: "TWD", Amount: decimal.RequireFromString(amount)}
}

func TestRepositoryCategoryOnlyFindsTheUsersOwn(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")

	got, err := repo.Category(t.Context(), a.id, a.food.ID)
	if err != nil || got != a.food {
		t.Errorf("A's own category = %+v, %v; want %+v", got, err, a.food)
	}
	if _, err := repo.Category(t.Context(), b.id, a.food.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B asking for A's category: error = %v, want apperr.ErrNotFound", err)
	}
}

func TestRepositoryCreateAndGetEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a := seedUser(t, pool, "a@example.com")
	lunch := EntryValues{
		OccurredOn: taipeiDate("2026-10-23"), CategoryID: a.food.ID, Currency: "TWD",
		Amount: decimal.RequireFromString("120"), Note: "午餐",
	}
	dividend := EntryValues{
		OccurredOn: taipeiDate("2026-10-24"), CategoryID: a.salary.ID, Currency: "USD", Amount: decimal.RequireFromString("12.34"),
		MarketRate: &MarketRate{Value: decimal.RequireFromString("32.508"), Date: taipeiDate("2026-10-23")},
	}

	for name, tc := range map[string]struct {
		values EntryValues
		want   Entry
	}{
		"TWD": {lunch, Entry{
			OccurredOn: lunch.OccurredOn, Category: a.food, Currency: "TWD", MinorUnit: 2, Amount: lunch.Amount, Note: "午餐",
		}},
		"USD": {dividend, Entry{
			OccurredOn: dividend.OccurredOn, Category: a.salary, Currency: "USD", MinorUnit: 2, Amount: dividend.Amount,
			MarketRate: dividend.MarketRate,
		}},
	} {
		id := mustCreate(t, repo, a.id, tc.values)
		got, err := repo.Entry(t.Context(), a.id, id)
		if err != nil {
			t.Fatalf("Entry(%s): %v", name, err)
		}
		if got.ID != id {
			t.Errorf("%s entry ID = %d, want %d", name, got.ID, id)
		}
		wantEntry(t, got, tc.want)
		if got.MarketRate != nil && got.MarketRate.Date.Location() != taipei.Location {
			t.Errorf("%s market rate date location = %v, want Asia/Taipei", name, got.MarketRate.Date.Location())
		}
	}
}

func TestRepositoryEntryOnlyFindsTheUsersOwn(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")
	id := mustCreate(t, repo, a.id, twdEntry(a.food, "2026-10-23", "120"))

	for name, tc := range map[string]struct{ user, id int64 }{"B reads A's entry": {b.id, id}, "missing id": {a.id, id + 1000}} {
		if _, err := repo.Entry(t.Context(), tc.user, tc.id); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: error = %v, want apperr.ErrNotFound", name, err)
		}
	}
}

func TestRepositoryUpdateEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")
	original := twdEntry(a.food, "2026-10-23", "120")
	id := mustCreate(t, repo, a.id, original)

	if err := repo.UpdateEntry(t.Context(), b.id, id, twdEntry(b.food, "2026-10-01", "999")); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B updates A's entry: error = %v, want apperr.ErrNotFound", err)
	}
	if err := repo.UpdateEntry(t.Context(), a.id, id+1000, original); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update a missing id: error = %v, want apperr.ErrNotFound", err)
	}
	if got, _ := repo.Entry(t.Context(), a.id, id); !got.Amount.Equal(decimal.RequireFromString("120")) {
		t.Errorf("A's amount after B's update = %s, want 120", got.Amount)
	}

	changed := EntryValues{
		OccurredOn: taipeiDate("2026-10-20"), CategoryID: a.salary.ID, Currency: "USD", Amount: decimal.RequireFromString("50"),
		MarketRate: &MarketRate{Value: decimal.RequireFromString("32.617"), Date: taipeiDate("2026-10-20")}, Note: "股息",
	}
	if err := repo.UpdateEntry(t.Context(), a.id, id, changed); err != nil {
		t.Fatalf("A updates own entry: %v", err)
	}
	got, err := repo.Entry(t.Context(), a.id, id)
	if err != nil {
		t.Fatal(err)
	}
	wantEntry(t, got, Entry{
		OccurredOn: changed.OccurredOn, Category: a.salary, Currency: "USD", MinorUnit: 2,
		Amount: changed.Amount, MarketRate: changed.MarketRate, Note: "股息",
	})
}

func TestRepositoryDeleteEntry(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")
	id := mustCreate(t, repo, a.id, twdEntry(a.food, "2026-10-23", "120"))

	if err := repo.DeleteEntry(t.Context(), b.id, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("B deletes A's entry: error = %v, want apperr.ErrNotFound", err)
	}
	if _, err := repo.Entry(t.Context(), a.id, id); err != nil {
		t.Errorf("A's entry after B's delete: %v, want it to still exist", err)
	}
	if err := repo.DeleteEntry(t.Context(), a.id, id); err != nil {
		t.Fatalf("A deletes own entry: %v", err)
	}
	if _, err := repo.Entry(t.Context(), a.id, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("entry after delete: error = %v, want apperr.ErrNotFound", err)
	}
	if err := repo.DeleteEntry(t.Context(), a.id, id); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("deleting twice: error = %v, want apperr.ErrNotFound", err)
	}
}

func TestRepositoryEntriesInAMonth(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")
	mustCreate(t, repo, a.id, twdEntry(a.food, "2026-09-30", "1"))
	firstOct1 := mustCreate(t, repo, a.id, twdEntry(a.food, "2026-10-01", "1"))
	secondOct1 := mustCreate(t, repo, a.id, twdEntry(a.food, "2026-10-01", "1"))
	oct23 := mustCreate(t, repo, a.id, twdEntry(a.food, "2026-10-23", "1"))
	mustCreate(t, repo, a.id, twdEntry(a.food, "2026-11-01", "1"))
	mustCreate(t, repo, b.id, twdEntry(b.food, "2026-10-15", "1"))

	got, err := repo.Entries(t.Context(), a.id, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
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
	a, b := seedUser(t, pool, "a@example.com"), seedUser(t, pool, "b@example.com")
	for _, v := range []EntryValues{
		twdEntry(a.salary, "2026-10-05", "50000"),
		twdEntry(a.food, "2026-10-23", "120"),
		twdEntry(a.food, "2026-10-31", "35.5"),
		{OccurredOn: taipeiDate("2026-10-10"), CategoryID: a.salary.ID, Currency: "USD", Amount: decimal.RequireFromString("12.34")},
		twdEntry(a.food, "2026-09-30", "1000"),
		twdEntry(a.food, "2026-11-01", "1000"),
	} {
		mustCreate(t, repo, a.id, v)
	}
	mustCreate(t, repo, b.id, twdEntry(b.food, "2026-10-15", "777"))

	got, err := repo.Totals(t.Context(), a.id, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}

	want := []struct{ currency, income, expense string }{{"TWD", "50000", "155.5"}, {"USD", "12.34", "0"}}
	if len(got) != len(want) {
		t.Fatalf("totals = %+v, want %v", got, want)
	}
	for i, w := range want {
		g := got[i]
		if g.Currency != w.currency || g.MinorUnit != 2 {
			t.Errorf("totals[%d] currency = %s with %d decimals, want %s with 2", i, g.Currency, g.MinorUnit, w.currency)
		}
		if !g.Income.Equal(decimal.RequireFromString(w.income)) || !g.Expense.Equal(decimal.RequireFromString(w.expense)) {
			t.Errorf("%s income %s expense %s, want income %s expense %s", g.Currency, g.Income, g.Expense, w.income, w.expense)
		}
	}
}

func TestRepositoryEmptyMonth(t *testing.T) {
	pool := dbtest.Migrated(t)
	repo := NewRepository(pool)
	a := seedUser(t, pool, "a@example.com")

	entries, err := repo.Entries(t.Context(), a.id, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil || len(entries) != 0 {
		t.Errorf("Entries = %v, %v; want none", entries, err)
	}
	totals, err := repo.Totals(t.Context(), a.id, taipeiDate("2026-10-01"), taipeiDate("2026-11-01"))
	if err != nil || len(totals) != 0 {
		t.Errorf("Totals = %v, %v; want none", totals, err)
	}
}
