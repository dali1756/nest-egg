package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/requestid"
)

type Handler func(http.ResponseWriter, *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tw := &trackingWriter{ResponseWriter: w}
	err := h(tw, r)
	if err == nil {
		return
	}
	if tw.wrote {
		slog.ErrorContext(r.Context(), "request failed after response was written", "method", r.Method, "path", r.URL.Path, "err", err)
		return
	}

	var (
		invalid  *apperr.ValidationError
		badJSON  *badJSONError
		tooLarge *http.MaxBytesError
	)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "not found", nil)
	case errors.As(err, &invalid):
		WriteError(w, r, http.StatusUnprocessableEntity, "invalid input", invalid.Fields)
	case errors.As(err, &tooLarge):
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request body too large", nil)
	case errors.As(err, &badJSON):
		WriteError(w, r, http.StatusBadRequest, "invalid json", nil)
	default:
		slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		WriteInternalError(w, r)
	}
}

type errorResponse struct {
	Error     string            `json:"error"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, msg string, fields map[string]string) {
	_ = WriteJSON(w, status, errorResponse{Error: msg, Fields: fields, RequestID: requestid.FromContext(r.Context())})
}

func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusInternalServerError, "internal error", nil)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type trackingWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *trackingWriter) WriteHeader(code int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *trackingWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

func (w *trackingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
