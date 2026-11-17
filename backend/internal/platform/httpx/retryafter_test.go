package httpx

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetRetryAfterRoundsUpToWholeSeconds(t *testing.T) {
	tests := []struct {
		wait time.Duration
		want string
	}{
		{30 * time.Second, "30"},
		{29*time.Second + 200*time.Millisecond, "30"},
		{400 * time.Millisecond, "1"},
		{0, "1"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		SetRetryAfter(rec, tt.wait)
		if got := rec.Header().Get("Retry-After"); got != tt.want {
			t.Errorf("SetRetryAfter(%v) = %q, want %q", tt.wait, got, tt.want)
		}
	}
}
