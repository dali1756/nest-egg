package requestid

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serveCapturingID(t *testing.T, req *http.Request) (header, fromCtx string) {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromCtx = FromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)
	return rec.Header().Get("X-Request-ID"), fromCtx
}

func TestMiddlewareGivesEachRequestItsOwnID(t *testing.T) {
	first, firstCtx := serveCapturingID(t, httptest.NewRequest(http.MethodGet, "/", nil))
	second, _ := serveCapturingID(t, httptest.NewRequest(http.MethodGet, "/", nil))

	if first == "" {
		t.Fatal("X-Request-ID is empty")
	}
	if firstCtx != first {
		t.Errorf("FromContext = %q, want the response header %q", firstCtx, first)
	}
	if first == second {
		t.Errorf("two requests got the same ID %q", first)
	}
}

func TestMiddlewareIgnoresTheClientsID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "attacker-chosen")

	header, fromCtx := serveCapturingID(t, req)
	if header == "attacker-chosen" || fromCtx == "attacker-chosen" {
		t.Errorf("header %q / context %q reused the client's X-Request-ID", header, fromCtx)
	}
	if header == "" {
		t.Error("X-Request-ID is empty")
	}
}

func TestFromContextWithoutIDIsEmpty(t *testing.T) {
	if got := FromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != "" {
		t.Errorf("FromContext = %q, want empty", got)
	}
}
