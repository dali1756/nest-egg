package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"
	"github.com/aarondl/authboss/v3/defaults"
)

func TestStatusResponderCodes(t *testing.T) {
	tests := []struct {
		name string
		page string
		data authboss.HTMLData
		want int
	}{
		{"login bad credentials", "login", authboss.HTMLData{authboss.DataErr: "Invalid Credentials"}, http.StatusUnauthorized},
		{"register field errors", "register", authboss.HTMLData{authboss.DataValidation: map[string][]string{"password": {"too short"}}}, http.StatusUnprocessableEntity},
		{"register field and general errors", "register", authboss.HTMLData{authboss.DataValidation: map[string][]string{"": {"x"}, "email": {"bad"}}}, http.StatusUnprocessableEntity},
		{"register user exists", "register", authboss.HTMLData{authboss.DataValidation: map[string][]string{"": {"User already exists"}}}, http.StatusConflict},
		{"page data without errors", "login", authboss.HTMLData{}, http.StatusOK},
		{"nil data", "register", nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newStatusResponder(defaults.NewResponder(defaults.JSONRenderer{}))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)

			if err := r.Respond(rec, req, http.StatusOK, tt.page, tt.data); err != nil {
				t.Fatalf("Respond: %v", err)
			}
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
