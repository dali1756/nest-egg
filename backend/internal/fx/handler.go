package fx

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type tradeRequest struct {
	TradedOn      string `json:"traded_on"`
	Side          string `json:"side"`
	Currency      string `json:"currency"`
	ForeignAmount string `json:"foreign_amount"`
	TWDAmount     string `json:"twd_amount"`
	Note          string `json:"note"`
}

func (req tradeRequest) input() TradeInput {
	return TradeInput{
		TradedOn: req.TradedOn, Side: req.Side, Currency: req.Currency,
		ForeignAmount: req.ForeignAmount, TWDAmount: req.TWDAmount, Note: req.Note,
	}
}

type ratesResponse struct {
	TradeRate      string  `json:"trade_rate"`
	MarketRate     *string `json:"market_rate"`
	MarketRateDate *string `json:"market_rate_date"`
	Spread         *string `json:"spread"`
}

type tradeResponse struct {
	ID            int64  `json:"id"`
	TradedOn      string `json:"traded_on"`
	Side          Side   `json:"side"`
	Currency      string `json:"currency"`
	ForeignAmount string `json:"foreign_amount"`
	TWDAmount     string `json:"twd_amount"`
	ratesResponse
	Note string `json:"note"`
}

func toRatesResponse(t Trade) ratesResponse {
	resp := ratesResponse{TradeRate: t.TradeRate().String()}
	if t.MarketRate != nil {
		rate, date := t.MarketRate.Value.String(), t.MarketRate.Date.Format(time.DateOnly)
		resp.MarketRate, resp.MarketRateDate = &rate, &date
	}
	if spread := t.Spread(); spread != nil {
		s := spread.StringFixed(int32(t.TWDMinorUnit))
		resp.Spread = &s
	}
	return resp
}

func toTradeResponse(t Trade) tradeResponse {
	return tradeResponse{
		ID:            t.ID,
		TradedOn:      t.TradedOn.Format(time.DateOnly),
		Side:          t.Side,
		Currency:      t.Currency,
		ForeignAmount: t.ForeignAmount.StringFixed(int32(t.MinorUnit)),
		TWDAmount:     t.TWDAmount.StringFixed(int32(t.TWDMinorUnit)),
		ratesResponse: toRatesResponse(t),
		Note:          t.Note,
	}
}

type monthResponse struct {
	Month  string          `json:"month"`
	Trades []tradeResponse `json:"trades"`
}

func (h *Handler) ListTrades(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	month, err := h.svc.Month(r.Context(), userID, r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	resp := monthResponse{Month: month.Month, Trades: make([]tradeResponse, 0, len(month.Trades))}
	for _, t := range month.Trades {
		resp.Trades = append(resp.Trades, toTradeResponse(t))
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateTrade(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	var req tradeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	t, err := h.svc.CreateTrade(r.Context(), userID, req.input())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusCreated, toTradeResponse(t))
}

func (h *Handler) GetTrade(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	id, err := httpx.PathID(r, "fx trade")
	if err != nil {
		return err
	}
	t, err := h.svc.Trade(r.Context(), userID, id)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toTradeResponse(t))
}

func (h *Handler) UpdateTrade(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	id, err := httpx.PathID(r, "fx trade")
	if err != nil {
		return err
	}
	var req tradeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	t, err := h.svc.UpdateTrade(r.Context(), userID, id, req.input())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toTradeResponse(t))
}

func (h *Handler) DeleteTrade(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	id, err := httpx.PathID(r, "fx trade")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteTrade(r.Context(), userID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) PreviewTrade(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	in := TradeInput{
		TradedOn: q.Get("traded_on"), Side: q.Get("side"), Currency: q.Get("currency"),
		ForeignAmount: q.Get("foreign_amount"), TWDAmount: q.Get("twd_amount"),
	}
	var t Trade
	if q.Has("id") {
		id, parseErr := strconv.ParseInt(q.Get("id"), 10, 64)
		if parseErr != nil {
			return fmt.Errorf("fx trade id %q: %w", q.Get("id"), apperr.ErrNotFound)
		}
		t, err = h.svc.PreviewChange(r.Context(), userID, id, in)
	} else {
		t, err = h.svc.PreviewNew(r.Context(), in)
	}
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toRatesResponse(t))
}
