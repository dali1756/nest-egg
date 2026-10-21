package rate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"nest-egg/backend/internal/platform/taipei"
)

func at(month time.Month, d, hour, min int) time.Time {
	return time.Date(2026, month, d, hour, min, 0, 0, taipei.Location)
}

func TestNextRun(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		haveToday bool
		want      time.Time
	}{
		{"before 17:30 waits for 17:30", at(10, 19, 9, 0), false, at(10, 19, 17, 30)},
		{"got today's rate", at(10, 19, 17, 30), true, at(10, 20, 17, 30)},
		{"failed or not published yet", at(10, 19, 17, 30), false, at(10, 19, 18, 30)},
		{"failed at 21:30", at(10, 19, 21, 30), false, at(10, 19, 22, 30)},
		{"failed at 22:00 retries no later than 22:30", at(10, 19, 22, 0), false, at(10, 19, 22, 30)},
		{"still missing at 22:30", at(10, 19, 22, 30), false, at(10, 20, 17, 30)},
		{"started at 23:00", at(10, 19, 23, 0), false, at(10, 20, 17, 30)},
		{"end of month", at(10, 31, 23, 0), true, at(11, 1, 17, 30)},
	}
	for _, tt := range tests {
		got := nextRun(tt.now.UTC(), tt.haveToday)
		if !got.Equal(tt.want) {
			t.Errorf("%s: nextRun(%s, haveToday=%v) = %s, want %s", tt.name,
				tt.now.Format("01-02 15:04"), tt.haveToday, got.In(taipei.Location).Format("01-02 15:04"), tt.want.Format("01-02 15:04"))
		}
	}
}

type fakeUpdate struct {
	started chan struct{}
	result  UpdateResult
	err     error
	panic   bool
	block   bool
}

func (f *fakeUpdate) Update(ctx context.Context) (UpdateResult, error) {
	f.started <- struct{}{}
	if f.panic {
		panic("boom")
	}
	if f.block {
		<-ctx.Done()
		return UpdateResult{}, fmt.Errorf("fetch fake rates: %w", ctx.Err())
	}
	return f.result, f.err
}

type logLine struct {
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	Job        string `json:"job"`
	Source     string `json:"source"`
	Fetched    int    `json:"fetched"`
	Inserted   int    `json:"inserted"`
	Updated    int    `json:"updated"`
	LatestDate string `json:"latest_date"`
	DurationMS *int   `json:"duration_ms"`
	Err        string `json:"err"`
	Stack      string `json:"stack"`
}

func runOnce(t *testing.T, f *fakeUpdate) []logLine {
	t.Helper()
	f.started = make(chan struct{}, 1)
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		NewUpdater(f, slog.New(slog.NewJSONHandler(&buf, nil))).Run(ctx)
	}()
	select {
	case <-f.started:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not update on start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx was cancelled")
	}

	var lines []logLine
	for _, raw := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(raw) == 0 {
			continue
		}
		var l logLine
		if err := json.Unmarshal(raw, &l); err != nil {
			t.Fatalf("parse log %q: %v", raw, err)
		}
		lines = append(lines, l)
	}
	return lines
}

func TestUpdaterLogsASuccessfulUpdate(t *testing.T) {
	lines := runOnce(t, &fakeUpdate{result: UpdateResult{Source: "cbc", Fetched: 4670, Inserted: 1, Updated: 2, Latest: day(2026, 10, 19)}})
	if len(lines) != 1 {
		t.Fatalf("log lines = %+v, want exactly 1", lines)
	}
	l := lines[0]
	if l.Level != "INFO" || l.Msg != "rates updated" || l.Job != "rates" || l.Source != "cbc" ||
		l.Fetched != 4670 || l.Inserted != 1 || l.Updated != 2 || l.LatestDate != "2026-10-19" || l.DurationMS == nil {
		t.Errorf("log = %+v, want INFO rates updated with job, source, counts, latest_date and duration_ms", l)
	}
}

func TestUpdaterLogsAFailureOnce(t *testing.T) {
	lines := runOnce(t, &fakeUpdate{err: errors.New("fetch cbc rates: connection refused")})
	if len(lines) != 1 {
		t.Fatalf("log lines = %+v, want exactly 1", lines)
	}
	if l := lines[0]; l.Level != "ERROR" || l.Msg != "rates update failed" || l.Job != "rates" || !strings.Contains(l.Err, "connection refused") {
		t.Errorf("log = %+v, want ERROR rates update failed with job and err", l)
	}
}

func TestUpdaterRecoversFromPanics(t *testing.T) {
	lines := runOnce(t, &fakeUpdate{panic: true})
	if len(lines) != 1 {
		t.Fatalf("log lines = %+v, want exactly 1", lines)
	}
	if l := lines[0]; l.Level != "ERROR" || l.Job != "rates" || !strings.Contains(l.Stack, "goroutine") {
		t.Errorf("log = %+v, want ERROR with job and stack", l)
	}
}

func TestUpdaterCancelledOnShutdownIsNotAnError(t *testing.T) {
	lines := runOnce(t, &fakeUpdate{block: true})
	for _, l := range lines {
		if l.Level == "ERROR" {
			t.Errorf("log = %+v, want no ERROR when the update is cancelled by shutdown", l)
		}
	}
}

func TestRunOnceReportsWhetherTodaysRateArrived(t *testing.T) {
	tests := []struct {
		name string
		f    *fakeUpdate
		want bool
	}{
		{"got today's rate", &fakeUpdate{result: UpdateResult{Latest: day(2026, 10, 19)}}, true},
		{"latest is yesterday", &fakeUpdate{result: UpdateResult{Latest: day(2026, 10, 16)}}, false},
		{"failed", &fakeUpdate{err: errors.New("connection refused")}, false},
		{"panicked", &fakeUpdate{panic: true}, false},
	}
	for _, tt := range tests {
		tt.f.started = make(chan struct{}, 1)
		u := NewUpdater(tt.f, slog.New(slog.DiscardHandler))
		u.now = func() time.Time { return evening }
		if got := u.runOnce(t.Context()); got != tt.want {
			t.Errorf("%s: runOnce = %v, want %v", tt.name, got, tt.want)
		}
	}
}
