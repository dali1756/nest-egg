package auth

import (
	"log/slog"
	"net/http"
)

type errorHandler struct {
	log *slog.Logger
}

func newErrorHandler(log *slog.Logger) errorHandler {
	return errorHandler{log: log}
}

func (e errorHandler) Wrap(handler func(w http.ResponseWriter, r *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err == nil {
			return
		}
		e.log.ErrorContext(r.Context(), "auth request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}` + "\n"))
	})
}

type slogLogger struct {
	l *slog.Logger
}

func (s slogLogger) Info(msg string)  { s.l.Info("authboss", "detail", msg) }
func (s slogLogger) Error(msg string) { s.l.Error("authboss", "detail", msg) }
