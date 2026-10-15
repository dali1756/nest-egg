package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusRecorderRecordsStatus(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	rec.WriteHeader(http.StatusServiceUnavailable)
	if rec.status != http.StatusServiceUnavailable {
		t.Errorf("rec.status = %d, want %d", rec.status, http.StatusServiceUnavailable)
	}
	if inner.Code != http.StatusServiceUnavailable {
		t.Errorf("inner.Code = %d, want %d", inner.Code, http.StatusServiceUnavailable)
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
	return &buf
}

func TestAccessLogLogsRequest(t *testing.T) {
	buf := captureLogs(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	AccessLog(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/test", nil))
	for _, want := range []string{"msg=request", "method=GET", "path=/api/test", "status=418"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log %q does not contain %q", buf.String(), want)
		}
	}
}

func TestAccessLogDefaultsTo200(t *testing.T) {
	buf := captureLogs(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	AccessLog(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(buf.String(), "status=200") {
		t.Errorf("log %q does not contain status=200", buf.String())
	}
}
