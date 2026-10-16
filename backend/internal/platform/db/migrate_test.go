package db_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"nest-egg/backend/db/migrations"
	"nest-egg/backend/internal/platform/db"
	"nest-egg/backend/internal/platform/db/dbtest"
)

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = $1)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return exists
}

func appliedCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&n); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	return n
}

func TestMigrateCreatesUsersTable(t *testing.T) {
	pool := dbtest.EmptySchema(t)

	if err := db.Migrate(t.Context(), pool, migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, table := range []string{"users", "goose_db_version"} {
		if !tableExists(t, pool, table) {
			t.Errorf("table %s does not exist after Migrate", table)
		}
	}
}

func TestMigrateTwiceDoesNotReapply(t *testing.T) {
	pool := dbtest.EmptySchema(t)

	if err := db.Migrate(t.Context(), pool, migrations.FS); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	before := appliedCount(t, pool)
	if err := db.Migrate(t.Context(), pool, migrations.FS); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if after := appliedCount(t, pool); after != before {
		t.Errorf("applied migrations = %d after second Migrate, want %d", after, before)
	}
}

func TestMigrationsRoundTrip(t *testing.T) {
	pool := dbtest.EmptySchema(t)
	ctx := t.Context()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("Up: %v", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if tableExists(t, pool, "users") {
		t.Fatal("users still exists after Down")
	}
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("Up again: %v", err)
	}
	if !tableExists(t, pool, "users") {
		t.Error("users does not exist after Up again")
	}
}

func TestMigrateReturnsErrorOnBadSQL(t *testing.T) {
	pool := dbtest.EmptySchema(t)
	bad := fstest.MapFS{
		"00001_bad.sql": {Data: []byte("-- +goose Up\nCREATE TABLE (;\n\n-- +goose Down\nSELECT 1;\n")},
	}

	if err := db.Migrate(t.Context(), pool, bad); err == nil {
		t.Fatal("Migrate with invalid SQL: want error, got nil")
	}
}
