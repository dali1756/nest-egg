package rate

import (
	"net/http"
	"time"

	"nest-egg/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type rateResponse struct {
	Base     string `json:"base"`
	Quote    string `json:"quote"`
	Rate     string `json:"rate"`
	RateDate string `json:"rate_date"`
	Source   string `json:"source"`
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	snap, err := h.svc.On(r.Context(), q.Get("base"), q.Get("quote"), q.Get("date"))
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, rateResponse{
		Base:     snap.Base,
		Quote:    snap.Quote,
		Rate:     snap.Value.String(),
		RateDate: snap.Date.Format(time.DateOnly),
		Source:   snap.Source,
	})
}
