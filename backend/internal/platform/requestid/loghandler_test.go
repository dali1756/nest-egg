package requestid

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func logOnce(t *testing.T, ctx context.Context, wrap func(*slog.Logger) *slog.Logger) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger := wrap(slog.New(NewLogHandler(slog.NewJSONHandler(&buf, nil))))
	logger.InfoContext(ctx, "hello")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("parse log line %q: %v", buf.String(), err)
	}
	return line
}

func ctxWithID(t *testing.T) (context.Context, string) {
	t.Helper()
	var ctx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ctx = r.Context() })
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return ctx, rec.Header().Get(Header)
}

func same(l *slog.Logger) *slog.Logger { return l }

func TestLogHandlerAddsRequestID(t *testing.T) {
	ctx, id := ctxWithID(t)
	line := logOnce(t, ctx, same)
	if line["request_id"] != id {
		t.Errorf("request_id = %v, want %q (line %v)", line["request_id"], id, line)
	}
}

func TestLogHandlerWithoutIDAddsNothing(t *testing.T) {
	line := logOnce(t, context.Background(), same)
	if _, ok := line["request_id"]; ok {
		t.Errorf("line %v has request_id, want none outside a request", line)
	}
}

func TestLogHandlerKeepsIDAfterWithAttrs(t *testing.T) {
	ctx, id := ctxWithID(t)
	line := logOnce(t, ctx, func(l *slog.Logger) *slog.Logger { return l.With("component", "auth") })
	if line["request_id"] != id || line["component"] != "auth" {
		t.Errorf("line %v, want request_id %q and component auth", line, id)
	}
}

func TestLogHandlerKeepsIDAfterWithGroup(t *testing.T) {
	ctx, id := ctxWithID(t)
	line := logOnce(t, ctx, func(l *slog.Logger) *slog.Logger { return l.WithGroup("g") })
	group, _ := line["g"].(map[string]any)
	if group["request_id"] != id {
		t.Errorf("line %v, want g.request_id %q", line, id)
	}
}
