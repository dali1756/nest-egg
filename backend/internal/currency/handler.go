package currency

import (
	"net/http"

	"nest-egg/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type currencyResponse struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	MinorUnit int    `json:"minor_unit"`
}

type listResponse struct {
	Currencies []currencyResponse `json:"currencies"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	currencies, err := h.svc.List(r.Context())
	if err != nil {
		return err
	}
	resp := listResponse{Currencies: make([]currencyResponse, 0, len(currencies))}
	for _, c := range currencies {
		resp.Currencies = append(resp.Currencies, currencyResponse{Code: c.Code, Name: c.Name, MinorUnit: c.MinorUnit})
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}
