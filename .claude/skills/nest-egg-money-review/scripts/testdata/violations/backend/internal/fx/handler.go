package fx

import "net/http"

type createRequest struct {
	UserID int64  `json:"user_id"`
	Note   string `json:"note"`
}

type tradeResponse struct {
	UserID int64 `json:"user_id,omitempty"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	_ = r.PathValue("user_id")
	_ = r.URL.Query().Get("user_id")
	_ = r.FormValue("user_id")
	return nil
}
