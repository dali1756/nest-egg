package httpx

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/requestid"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(requestid.NewLogHandler(slog.NewJSONHandler(&buf, nil))))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func serve(h http.Handler, req *http.Request) (*httptest.ResponseRecorder, string) {
	rec := httptest.NewRecorder()
	requestid.Middleware(h).ServeHTTP(rec, req)
	return rec, rec.Header().Get(requestid.Header)
}

func returning(err error) Handler {
	return func(http.ResponseWriter, *http.Request) error { return err }
}

func TestHandlerTurnsErrorsIntoResponses(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		bodyJSON string // %s 是 request ID
		errorLog bool
	}{
		{
			name:     "not found",
			err:      fmt.Errorf("get trade: %w", apperr.ErrNotFound),
			status:   http.StatusNotFound,
			bodyJSON: `{"error":"not found","request_id":"%s"}`,
		},
		{
			name:     "validation",
			err:      fmt.Errorf("create trade: %w", &apperr.ValidationError{Fields: map[string]string{"amount": "must be positive"}}),
			status:   http.StatusUnprocessableEntity,
			bodyJSON: `{"error":"invalid input","fields":{"amount":"must be positive"},"request_id":"%s"}`,
		},
		{
			name:     "unexpected",
			err:      fmt.Errorf("query: %w", errors.New("connection refused")),
			status:   http.StatusInternalServerError,
			bodyJSON: `{"error":"internal error","request_id":"%s"}`,
			errorLog: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			rec, id := serve(returning(tt.err), httptest.NewRequest(http.MethodGet, "/api/x/1", nil))

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if want := fmt.Sprintf(tt.bodyJSON, id) + "\n"; rec.Body.String() != want {
				t.Errorf("body = %s, want %s", rec.Body, want)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			errorLines := strings.Count(logs.String(), `"level":"ERROR"`)
			if !tt.errorLog {
				if errorLines != 0 {
					t.Errorf("logged %d ERROR lines for an expected error: %s", errorLines, logs)
				}
				return
			}
			if errorLines != 1 {
				t.Errorf("logged %d ERROR lines, want exactly 1: %s", errorLines, logs)
			}
			for _, want := range []string{"connection refused", `"request_id":"` + id + `"`, `"path":"/api/x/1"`} {
				if !strings.Contains(logs.String(), want) {
					t.Errorf("error log %s does not contain %s", logs, want)
				}
			}
		})
	}
}

func TestHandlerDoesNotLeakInternalErrors(t *testing.T) {
	captureLogs(t)
	rec, _ := serve(returning(errors.New("password authentication failed for user admin")), httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "admin") {
		t.Errorf("response %s leaks the internal error", rec.Body)
	}
}

func TestHandlerDoesNotWriteTwice(t *testing.T) {
	logs := captureLogs(t)
	h := Handler(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		return errors.New("stream broke")
	})
	rec, _ := serve(h, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
		t.Errorf("response = %d %q, want the original 200 \"partial\" untouched", rec.Code, rec.Body)
	}
	if !strings.Contains(logs.String(), "stream broke") {
		t.Errorf("error after writing was not logged: %s", logs)
	}
}

func TestHandlerSuccessUsesWriteJSON(t *testing.T) {
	h := Handler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJSON(w, http.StatusCreated, map[string]string{"id": "7"})
	})
	rec, _ := serve(h, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusCreated || rec.Body.String() != `{"id":"7"}`+"\n" {
		t.Errorf("response = %d %s, want 201 {\"id\":\"7\"}", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteInternalErrorWithoutRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != `{"error":"internal error"}`+"\n" {
		t.Errorf("response = %d %s, want 500 {\"error\":\"internal error\"}", rec.Code, rec.Body)
	}
}
