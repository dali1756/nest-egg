package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"nest-egg/backend/internal/platform/httpx"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &wroteRecorder{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			slog.ErrorContext(r.Context(), "panic recovered",
				"panic", v, "stack", string(debug.Stack()), "method", r.Method, "path", r.URL.Path)
			if !ww.wrote {
				httpx.WriteInternalError(w, r)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

type wroteRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (w *wroteRecorder) WriteHeader(code int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *wroteRecorder) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

func (w *wroteRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
