package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHandlerGet(t *testing.T) {
	tests := []struct {
		name     string
		pingErr  error
		wantCode int
		want     response
	}{
		{"database up", nil, http.StatusOK, response{Status: "ok", Database: "up"}},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, response{Status: "unavailable", Database: "down"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

			NewHandler(fakePinger{err: tt.pingErr}).Get(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			var got response
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got != tt.want {
				t.Errorf("body = %+v, want %+v", got, tt.want)
			}
		})
	}
}
