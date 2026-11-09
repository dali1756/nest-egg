package portfolio

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/rate"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type openingRequest struct {
	OpenedOn string `json:"opened_on"`
	Amount   string `json:"amount"`
	TWDCost  string `json:"twd_cost"`
}

func (req openingRequest) input() OpeningInput {
	return OpeningInput{OpenedOn: req.OpenedOn, Amount: req.Amount, TWDCost: req.TWDCost}
}

type openingCostResponse struct {
	MarketRate     *string `json:"market_rate"`
	MarketRateDate *string `json:"market_rate_date"`
	Cost           *string `json:"cost"`
	AverageCost    *string `json:"average_cost"`
}

type openingResponse struct {
	Currency string  `json:"currency"`
	OpenedOn string  `json:"opened_on"`
	Amount   string  `json:"amount"`
	TWDCost  *string `json:"twd_cost"`
	openingCostResponse
}

type openingsResponse struct {
	OpeningBalances []openingResponse `json:"opening_balances"`
}

type missingRateEntryResponse struct {
	ID         int64  `json:"id"`
	OccurredOn string `json:"occurred_on"`
}

type cashResponse struct {
	Currency             string                    `json:"currency"`
	Name                 string                    `json:"name"`
	Balance              string                    `json:"balance"`
	TWDValue             *string                   `json:"twd_value"`
	AverageCost          *string                   `json:"average_cost"`
	Cost                 *string                   `json:"cost"`
	MarketRate           *string                   `json:"market_rate"`
	MarketRateDate       *string                   `json:"market_rate_date"`
	UnrealizedPnL        *string                   `json:"unrealized_pnl"`
	UnrealizedPnLPercent *string                   `json:"unrealized_pnl_percent"`
	UnavailableReason    *UnavailableReason        `json:"unavailable_reason"`
	MissingRateEntry     *missingRateEntryResponse `json:"missing_rate_entry"`
}

type summaryResponse struct {
	Cash []cashResponse `json:"cash"`
}

func toOpeningCostResponse(o Opening) openingCostResponse {
	resp := openingCostResponse{
		Cost:        fixedString(o.Cost(), o.TWDMinorUnit),
		AverageCost: fixedString(o.AverageCost(), averageCostPlaces),
	}
	resp.MarketRate, resp.MarketRateDate = marketRateStrings(o.MarketRate)
	return resp
}

func toOpeningResponse(o Opening) openingResponse {
	return openingResponse{
		Currency:            o.Currency,
		OpenedOn:            o.OpenedOn.Format(time.DateOnly),
		Amount:              o.Amount.StringFixed(int32(o.MinorUnit)),
		TWDCost:             fixedString(o.TWDCost, o.TWDMinorUnit),
		openingCostResponse: toOpeningCostResponse(o),
	}
}

func toCashResponse(c CashBalance) cashResponse {
	resp := cashResponse{
		Currency:             c.Currency.Code,
		Name:                 c.Currency.Name,
		Balance:              c.Balance.StringFixed(int32(c.Currency.MinorUnit)),
		TWDValue:             fixedString(c.TWDValue, c.TWDMinorUnit),
		AverageCost:          fixedString(c.AverageCost, averageCostPlaces),
		Cost:                 fixedString(c.Cost, c.TWDMinorUnit),
		UnrealizedPnL:        fixedString(c.UnrealizedPnL, c.TWDMinorUnit),
		UnrealizedPnLPercent: fixedString(c.UnrealizedPnLPercent, percentPlaces),
	}
	resp.MarketRate, resp.MarketRateDate = marketRateStrings(c.MarketRate)
	if c.UnavailableReason != "" {
		resp.UnavailableReason = &c.UnavailableReason
	}
	if e := c.MissingRateEntry; e != nil {
		resp.MissingRateEntry = &missingRateEntryResponse{ID: e.ID, OccurredOn: e.OccurredOn.Format(time.DateOnly)}
	}
	return resp
}

// fixedString 把金額補足小數位數；算不出來的值回 nil，JSON 是 null。
func fixedString(d *decimal.Decimal, places int) *string {
	if d == nil {
		return nil
	}
	s := d.StringFixed(int32(places))
	return &s
}

func marketRateStrings(m *rate.MarketRate) (value, date *string) {
	if m == nil {
		return nil, nil
	}
	v, d := m.Value.String(), m.Date.Format(time.DateOnly)
	return &v, &d
}

func pathCurrency(r *http.Request) string {
	return r.PathValue("currency")
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	rows, err := h.svc.Summary(r.Context(), userID)
	if err != nil {
		return err
	}
	resp := summaryResponse{Cash: make([]cashResponse, 0, len(rows))}
	for _, row := range rows {
		resp.Cash = append(resp.Cash, toCashResponse(row))
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListOpenings(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	openings, err := h.svc.Openings(r.Context(), userID)
	if err != nil {
		return err
	}
	resp := openingsResponse{OpeningBalances: make([]openingResponse, 0, len(openings))}
	for _, o := range openings {
		resp.OpeningBalances = append(resp.OpeningBalances, toOpeningResponse(o))
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetOpening(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	o, err := h.svc.Opening(r.Context(), userID, pathCurrency(r))
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toOpeningResponse(o))
}

func (h *Handler) SetOpening(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	var req openingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	o, created, err := h.svc.SetOpening(r.Context(), userID, pathCurrency(r), req.input())
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return httpx.WriteJSON(w, status, toOpeningResponse(o))
}

func (h *Handler) DeleteOpening(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteOpening(r.Context(), userID, pathCurrency(r)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) PreviewOpening(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	in := OpeningInput{OpenedOn: q.Get("opened_on"), Amount: q.Get("amount"), TWDCost: q.Get("twd_cost")}
	o, err := h.svc.PreviewOpening(r.Context(), userID, q.Get("currency"), in)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toOpeningCostResponse(o))
}
