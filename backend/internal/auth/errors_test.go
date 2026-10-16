package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondl/authboss/v3"
)

var (
	_ authboss.ErrorHandler = errorHandler{}
	_ authboss.Logger       = slogLogger{}
)

// logRecords 解析 JSON handler 寫出的每一行 log。
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

func TestErrorHandlerRespondsInternalError(t *testing.T) {
	var buf bytes.Buffer
	h := newErrorHandler(slog.New(slog.NewJSONHandler(&buf, nil)))
	wrapped := h.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("get user by email: connection refused")
	})

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"internal error"}` {
		t.Errorf("body = %s, want {\"error\":\"internal error\"}", body)
	}
	records := logRecords(t, &buf)
	if len(records) != 1 {
		t.Fatalf("got %d log records, want 1: %s", len(records), buf.String())
	}
	if records[0]["level"] != "ERROR" || !strings.Contains(records[0]["err"].(string), "connection refused") {
		t.Errorf("log record = %v, want ERROR with err containing the cause", records[0])
	}
}

func TestErrorHandlerPassesThroughSuccess(t *testing.T) {
	var buf bytes.Buffer
	h := newErrorHandler(slog.New(slog.NewJSONHandler(&buf, nil)))
	wrapped := h.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusTeapot)
		return nil
	})

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected log: %s", buf.String())
	}
}

func TestSlogLoggerUsesFixedMessage(t *testing.T) {
	var buf bytes.Buffer
	l := slogLogger{l: slog.New(slog.NewJSONHandler(&buf, nil))}

	l.Info("user me@example.com logged in")
	l.Error("request error: boom")

	records := logRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("got %d log records, want 2", len(records))
	}
	want := []struct{ level, detail string }{
		{"INFO", "user me@example.com logged in"},
		{"ERROR", "request error: boom"},
	}
	for i, w := range want {
		r := records[i]
		if r["msg"] != "authboss" || r["level"] != w.level || r["detail"] != w.detail {
			t.Errorf("record %d = %v, want msg=authboss level=%s detail=%q", i, r, w.level, w.detail)
		}
	}
}
