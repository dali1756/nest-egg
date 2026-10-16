package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aarondl/authboss/v3"
)

func (a *Auth) Routes(mux *http.ServeMux) {
	mux.Handle(MountPath+"/", http.StripPrefix(MountPath, a.ab.Config.Core.Router))
	mux.Handle("GET "+MountPath+"/me", a.RequireUser(http.HandlerFunc(a.me)))
}

func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := a.ab.LoadCurrentUser(&r)
		switch {
		case errors.Is(err, authboss.ErrUserNotFound):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		case err != nil:
			a.log.ErrorContext(r.Context(), "load current user failed", "path", r.URL.Path, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		default:
			next.ServeHTTP(w, r)
		}
	})
}

type meResponse struct {
	Email string `json:"email"`
}

func (a *Auth) me(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(authboss.CTXKeyUser).(authboss.User)
	writeJSON(w, http.StatusOK, meResponse{Email: user.GetPID()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
