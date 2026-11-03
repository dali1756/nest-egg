package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"nest-egg/backend/internal/platform/apperr"
)

func requestWithID(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", id)
	return r
}

func TestParseIDParsesADecimalNumber(t *testing.T) {
	if id, err := ParseID("12", "fx trade"); id != 12 || err != nil {
		t.Errorf("ParseID = %d, %v; want 12", id, err)
	}
}

func TestParseIDThatIsNotANumberIsNotFound(t *testing.T) {
	for _, raw := range []string{"abc", "1.5", "", "99999999999999999999"} {
		if _, err := ParseID(raw, "fx trade"); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("ParseID(%q) error = %v, want not found", raw, err)
		}
	}
}

func TestPathIDParsesTheID(t *testing.T) {
	if id, err := PathID(requestWithID("12"), "fx trade"); id != 12 || err != nil {
		t.Errorf("PathID = %d, %v; want 12", id, err)
	}
}

func TestPathIDThatIsNotANumberIsNotFound(t *testing.T) {
	if _, err := PathID(requestWithID("abc"), "fx trade"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("PathID error = %v, want not found", err)
	}
}
