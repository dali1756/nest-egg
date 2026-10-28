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

func TestPathIDParsesTheID(t *testing.T) {
	if id, err := PathID(requestWithID("12"), "fx trade"); id != 12 || err != nil {
		t.Errorf("PathID = %d, %v; want 12", id, err)
	}
}

func TestPathIDThatIsNotANumberIsNotFound(t *testing.T) {
	for _, raw := range []string{"abc", "1.5", "", "99999999999999999999"} {
		if _, err := PathID(requestWithID(raw), "fx trade"); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("PathID(%q) error = %v, want not found", raw, err)
		}
	}
}
