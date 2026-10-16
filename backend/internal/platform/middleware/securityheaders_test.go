package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var wantSecurityHeaders = map[string]string{
	"Content-Security-Policy":      "default-src 'none'; frame-ancestors 'none'",
	"X-Frame-Options":              "DENY",
	"X-Content-Type-Options":       "nosniff",
	"Referrer-Policy":              "no-referrer",
	"Cross-Origin-Resource-Policy": "same-origin",
	"Cache-Control":                "no-store",
}

func serveWithHeaders(hsts bool, status int) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
	rec := httptest.NewRecorder()
	SecurityHeaders(next, hsts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/test", nil))
	return rec
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError} {
		rec := serveWithHeaders(false, status)
		for name, want := range wantSecurityHeaders {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("status %d: %s = %q, want %q", status, name, got, want)
			}
		}
	}
}

func TestSecurityHeadersHSTSOnlyWhenEnabled(t *testing.T) {
	if got := serveWithHeaders(false, http.StatusOK).Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("hsts=false: Strict-Transport-Security = %q, want none", got)
	}
	if got := serveWithHeaders(true, http.StatusOK).Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("hsts=true: Strict-Transport-Security = %q, want max-age=31536000", got)
	}
}
