package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserIDReadsTheLoggedInUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(WithUser(r.Context(), User{ID: 42, Email: "me@example.com"}))

	if id, err := UserID(r); id != 42 || err != nil {
		t.Errorf("UserID = %d, %v; want 42", id, err)
	}
}

func TestUserIDWithoutALoggedInUserIsAnError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, err := UserID(r); err == nil {
		t.Error("UserID without a user returned no error")
	}
}
