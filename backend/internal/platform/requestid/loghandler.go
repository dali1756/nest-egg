package requestid

import (
	"context"
	"log/slog"
)

func NewLogHandler(next slog.Handler) slog.Handler {
	return logHandler{next: next}
}

type logHandler struct {
	next slog.Handler
}

func (h logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h logHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := FromContext(ctx); id != "" {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.next.Handle(ctx, r)
}

func (h logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return logHandler{next: h.next.WithAttrs(attrs)}
}

func (h logHandler) WithGroup(name string) slog.Handler {
	return logHandler{next: h.next.WithGroup(name)}
}
