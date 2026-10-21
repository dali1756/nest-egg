package currency

import (
	"slices"
	"testing"

	"nest-egg/backend/internal/platform/db/dbtest"
)

func TestRepositoryListsCurrenciesByCode(t *testing.T) {
	repo := NewRepository(dbtest.Migrated(t))
	got, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Currency{{Code: "TWD", Name: "新臺幣", MinorUnit: 2}, {Code: "USD", Name: "美元", MinorUnit: 2}}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestRepositoryExists(t *testing.T) {
	repo := NewRepository(dbtest.Migrated(t))
	for code, want := range map[string]bool{"USD": true, "TWD": true, "XYZ": false, "usd": false} {
		got, err := repo.Exists(t.Context(), code)
		if err != nil || got != want {
			t.Errorf("Exists(%q) = %v, %v; want %v", code, got, err, want)
		}
	}
}
