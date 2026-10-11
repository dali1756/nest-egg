package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	db Pinger
}

func NewHandler(db Pinger) *Handler {
	return &Handler{db: db}
}

type response struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// get 每次都實際 ping 資料庫：連得到回 200，連不到回 503。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	code, body := http.StatusOK, response{Status: "ok", Database: "up"}
	start := time.Now()
	if err := h.db.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "database ping failed", "err", err)
		code, body = http.StatusServiceUnavailable, response{Status: "unavailable", Database: "down"}
	} else {
		slog.DebugContext(ctx, "database ping ok", "duration", time.Since(start))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(ctx, "write health response", "err", err)
	}
}
