package ledger

import (
	"errors"
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

var errNoUser = errors.New("no user in request context")

func currentUserID(r *http.Request) (int64, error) {
	user, ok := auth.CurrentUser(r.Context())
	if !ok {
		return 0, errNoUser
	}
	return user.ID, nil
}

// entryID 解析路徑的 {id}，不是數字時和不存在一樣回 404
func entryID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ledger entry id %q: %w", r.PathValue("id"), apperr.ErrNotFound)
	}
	return id, nil
}

type categoryResponse struct {
	ID   int64  `json:"id"`
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

type categoriesResponse struct {
	Categories []categoryResponse `json:"categories"`
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	categories, err := h.svc.Categories(r.Context(), userID)
	if err != nil {
		return err
	}
	resp := categoriesResponse{Categories: make([]categoryResponse, 0, len(categories))}
	for _, c := range categories {
		resp.Categories = append(resp.Categories, categoryResponse{ID: c.ID, Kind: c.Kind, Name: c.Name})
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

type entryRequest struct {
	OccurredOn string `json:"occurred_on"`
	CategoryID int64  `json:"category_id"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	Note       string `json:"note"`
}

func (req entryRequest) input() EntryInput {
	return EntryInput{OccurredOn: req.OccurredOn, CategoryID: req.CategoryID, Currency: req.Currency, Amount: req.Amount, Note: req.Note}
}

type categoryRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type entryResponse struct {
	ID             int64       `json:"id"`
	OccurredOn     string      `json:"occurred_on"`
	Kind           Kind        `json:"kind"`
	Category       categoryRef `json:"category"`
	Currency       string      `json:"currency"`
	Amount         string      `json:"amount"`
	MarketRate     *string     `json:"market_rate"`
	MarketRateDate *string     `json:"market_rate_date"`
	Note           string      `json:"note"`
}

// 金額固定為幣別的小數位數，例如 120.00、匯率不補零，例如 32.508
func toEntryResponse(e Entry) entryResponse {
	resp := entryResponse{
		ID:         e.ID,
		OccurredOn: e.OccurredOn.Format(time.DateOnly),
		Kind:       e.Category.Kind,
		Category:   categoryRef{ID: e.Category.ID, Name: e.Category.Name},
		Currency:   e.Currency,
		Amount:     e.Amount.StringFixed(int32(e.MinorUnit)),
		Note:       e.Note,
	}
	if e.MarketRate != nil {
		rate, date := e.MarketRate.Value.String(), e.MarketRate.Date.Format(time.DateOnly)
		resp.MarketRate, resp.MarketRateDate = &rate, &date
	}
	return resp
}

type totalResponse struct {
	Currency string `json:"currency"`
	Income   string `json:"income"`
	Expense  string `json:"expense"`
	Net      string `json:"net"`
}

func toTotalResponse(t Total) totalResponse {
	places := int32(t.MinorUnit)
	return totalResponse{
		Currency: t.Currency,
		Income:   t.Income.StringFixed(places),
		Expense:  t.Expense.StringFixed(places),
		Net:      t.Net().StringFixed(places),
	}
}

type monthResponse struct {
	Month   string          `json:"month"`
	Entries []entryResponse `json:"entries"`
	Totals  []totalResponse `json:"totals"`
}

func (h *Handler) ListEntries(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	month, err := h.svc.Month(r.Context(), userID, r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	resp := monthResponse{
		Month:   month.Month,
		Entries: make([]entryResponse, 0, len(month.Entries)),
		Totals:  make([]totalResponse, 0, len(month.Totals)),
	}
	for _, e := range month.Entries {
		resp.Entries = append(resp.Entries, toEntryResponse(e))
	}
	for _, t := range month.Totals {
		resp.Totals = append(resp.Totals, toTotalResponse(t))
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	var req entryRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	e, err := h.svc.CreateEntry(r.Context(), userID, req.input())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusCreated, toEntryResponse(e))
}

func (h *Handler) GetEntry(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	id, err := entryID(r)
	if err != nil {
		return err
	}
	e, err := h.svc.Entry(r.Context(), userID, id)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *Handler) UpdateEntry(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	id, err := entryID(r)
	if err != nil {
		return err
	}
	var req entryRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	e, err := h.svc.UpdateEntry(r.Context(), userID, id, req.input())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) error {
	userID, err := currentUserID(r)
	if err != nil {
		return err
	}
	id, err := entryID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteEntry(r.Context(), userID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
