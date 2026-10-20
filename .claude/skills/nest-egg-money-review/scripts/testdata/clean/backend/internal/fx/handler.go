package fx

import (
	"net/http"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/httpx"
)

type createRequest struct {
	Note string `json:"note"`
}

// 使用者只從登入狀態取得
func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.CurrentUser(r.Context())
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	return h.svc.Create(r.Context(), user.ID, req.Note)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.CurrentUser(r.Context())
	return h.svc.Get(r.Context(), user.ID, r.PathValue("id"))
}
