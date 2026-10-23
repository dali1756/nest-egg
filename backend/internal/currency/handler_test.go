package currency

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/httpx"
)

type fakeStore struct {
	currencies []Currency
	err        error
}

func (f fakeStore) List(context.Context) ([]Currency, error) { return f.currencies, f.err }

func (f fakeStore) Exists(_ context.Context, code string) (bool, error) {
	for _, c := range f.currencies {
		if c.Code == code {
			return true, f.err
		}
	}
	return false, f.err
}

func (f fakeStore) Get(_ context.Context, code string) (Currency, error) {
	for _, c := range f.currencies {
		if c.Code == code {
			return c, f.err
		}
	}
	return Currency{}, apperr.ErrNotFound
}

func serve(store Store) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	httpx.Handler(NewHandler(NewService(store)).List).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/currencies", nil))
	return rec
}

func TestListCurrencies(t *testing.T) {
	rec := serve(fakeStore{currencies: []Currency{{Code: "TWD", Name: "新臺幣", MinorUnit: 2}, {Code: "USD", Name: "美元", MinorUnit: 2}}})

	want := `{"currencies":[{"code":"TWD","name":"新臺幣","minor_unit":2},{"code":"USD","name":"美元","minor_unit":2}]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("response = %d %s, want 200 %s", rec.Code, rec.Body, want)
	}
	if strings.Contains(rec.Body.String(), "user_id") {
		t.Errorf("response %s contains user_id", rec.Body)
	}
}

func TestListCurrenciesStoreError(t *testing.T) {
	rec := serve(fakeStore{err: errors.New("connection refused")})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("response = %d %s, want 500 without the internal error", rec.Code, rec.Body)
	}
}

func TestServiceExists(t *testing.T) {
	svc := NewService(fakeStore{currencies: []Currency{{Code: "USD"}}})
	if ok, err := svc.Exists(t.Context(), "USD"); err != nil || !ok {
		t.Errorf("Exists(USD) = %v, %v; want true", ok, err)
	}
	if ok, err := svc.Exists(t.Context(), "XYZ"); err != nil || ok {
		t.Errorf("Exists(XYZ) = %v, %v; want false", ok, err)
	}
}
