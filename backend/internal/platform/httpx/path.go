package httpx

import (
	"fmt"
	"net/http"
	"strconv"

	"nest-egg/backend/internal/platform/apperr"
)

func PathID(r *http.Request, what string) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s id %q: %w", what, raw, apperr.ErrNotFound)
	}
	return id, nil
}
