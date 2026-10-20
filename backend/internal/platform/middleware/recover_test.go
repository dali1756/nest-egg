package middleware

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nest-egg/backend/internal/platform/requestid"
)

func captureJSONLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(requestid.NewLogHandler(slog.NewJSONHandler(&buf, nil))))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func panicking(v any) http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(v) })
}

func TestRecoverTurnsPanicIntoInternalError(t *testing.T) {
	logs := captureJSONLogs(t)
	rec := httptest.NewRecorder()
	requestid.Middleware(Recover(panicking("boom"))).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	id := rec.Header().Get(requestid.Header)

	want := fmt.Sprintf(`{"error":"internal error","request_id":"%s"}`, id) + "\n"
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != want {
		t.Errorf("response = %d %s, want 500 %s", rec.Code, rec.Body, want)
	}
	for _, s := range []string{`"level":"ERROR"`, `"msg":"panic recovered"`, `"panic":"boom"`, `"stack":"goroutine`, `"path":"/api/x"`, `"request_id":"` + id + `"`} {
		if !strings.Contains(logs.String(), s) {
			t.Errorf("log %s does not contain %s", logs, s)
		}
	}
}

func TestRecoverKeepsAResponseAlreadyWritten(t *testing.T) {
	logs := captureJSONLogs(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("late boom")
	})
	rec := httptest.NewRecorder()
	Recover(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
		t.Errorf("response = %d %q, want the original 200 \"partial\" untouched", rec.Code, rec.Body)
	}
	if !strings.Contains(logs.String(), "late boom") {
		t.Errorf("panic after writing was not logged: %s", logs)
	}
}

func TestRecoverLetsAbortHandlerThrough(t *testing.T) {
	logs := captureJSONLogs(t)
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", v)
		}
		if logs.Len() != 0 {
			t.Errorf("ErrAbortHandler was logged: %s", logs)
		}
	}()
	Recover(panicking(http.ErrAbortHandler)).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
