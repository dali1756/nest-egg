package httpxtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/httpx"
)

func ServeAs(userID int64, h httpx.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	if userID != 0 {
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: userID, Email: "me@example.com"}))
	}
	h.ServeHTTP(rec, req)
	return rec
}

func Send(userID int64, method, path, id, body string, h httpx.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if id != "" {
		req.SetPathValue("id", id)
	}
	return ServeAs(userID, h, req)
}

func WantResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status || strings.TrimSpace(rec.Body.String()) != body {
		t.Errorf("response = %d %s\nwant       %d %s", rec.Code, strings.TrimSpace(rec.Body.String()), status, body)
	}
	WantNoUserID(t, rec)
}

func WantNoUserID(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(rec.Body.String(), "user_id") {
		t.Errorf("response %s contains user_id", rec.Body)
	}
}
