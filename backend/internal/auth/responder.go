package auth

import (
	"net/http"

	"github.com/aarondl/authboss/v3"
)

type statusResponder struct {
	next authboss.HTTPResponder
}

func newStatusResponder(next authboss.HTTPResponder) authboss.HTTPResponder {
	return statusResponder{next: next}
}

func (s statusResponder) Respond(w http.ResponseWriter, r *http.Request, code int, page string, data authboss.HTMLData) error {
	if code == http.StatusOK {
		code = failureStatus(page, data)
	}
	return s.next.Respond(w, r, code, page, data)
}

func failureStatus(page string, data authboss.HTMLData) int {
	if page == "login" && data[authboss.DataErr] != nil {
		return http.StatusUnauthorized
	}
	if errs, ok := data[authboss.DataValidation].(map[string][]string); ok && len(errs) > 0 {
		for field := range errs {
			if field != "" {
				return http.StatusUnprocessableEntity
			}
		}
		return http.StatusConflict
	}
	return http.StatusOK
}
