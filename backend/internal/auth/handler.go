package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"nest-egg/backend/internal/platform/httpx"
)

type HandlerConfig struct {
	CookieSecure bool
	RateLimit    func(http.Handler) http.Handler
}

type Handler struct {
	svc          *Service
	cookieSecure bool
	cookieName   string
	rateLimit    func(http.Handler) http.Handler
	log          *slog.Logger
}

func NewHandler(svc *Service, cfg HandlerConfig, log *slog.Logger) *Handler {
	limit := cfg.RateLimit
	if limit == nil {
		limit = func(next http.Handler) http.Handler { return next }
	}
	return &Handler{
		svc:          svc,
		cookieSecure: cfg.CookieSecure,
		cookieName:   sessionCookieName(cfg.CookieSecure),
		rateLimit:    limit,
		log:          log,
	}
}

func sessionCookieName(secure bool) string {
	if secure {
		return "__Host-nest_egg_session"
	}
	return "nest_egg_session"
}

func (h *Handler) Routes(mux *http.ServeMux) {
	limit := h.rateLimit
	mux.Handle("POST /api/auth/register", limit(http.HandlerFunc(h.register)))
	mux.Handle("POST /api/auth/login", limit(http.HandlerFunc(h.login)))
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.Handle("GET /api/auth/me", h.RequireUser(http.HandlerFunc(h.me)))
	mux.Handle("POST /api/auth/password/forgot", limit(http.HandlerFunc(h.forgotPassword)))
	mux.Handle("POST /api/auth/password/reset", limit(http.HandlerFunc(h.resetPassword)))
	mux.Handle("POST /api/auth/password/change", limit(h.RequireUser(http.HandlerFunc(h.changePassword))))
}

type ctxKey struct{}

func (h *Handler) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := h.sessionToken(r)
		user, err := h.svc.Authenticate(r.Context(), token)
		switch {
		case errors.Is(err, ErrNoSession):
			if token != "" {
				h.clearCookie(w)
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
		case err != nil:
			h.internalError(w, r, err)
		default:
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
		}
	})
}

func CurrentUser(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

type userResponse struct {
	Email string `json:"email"`
}

type errorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

type registerRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, err := h.svc.Register(r.Context(), RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	var invalid *ValidationError
	switch {
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid input", Fields: invalid.Fields})
	case errors.Is(err, ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already registered")
	case err != nil:
		h.internalError(w, r, err)
	default:
		h.setCookie(w, token)
		writeJSON(w, http.StatusCreated, userResponse{Email: user.Email})
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, err := h.svc.Login(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, ErrAccountLocked):
		writeError(w, http.StatusLocked, "account locked")
	case err != nil:
		h.internalError(w, r, err)
	default:
		h.setCookie(w, token)
		writeJSON(w, http.StatusOK, userResponse{Email: user.Email})
	}
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.clearCookie(w)
	if err := h.svc.Logout(r.Context(), h.sessionToken(r)); err != nil {
		h.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	err := h.svc.RequestPasswordReset(r.Context(), req.Email)
	switch {
	case errors.Is(err, ErrResetUnavailable):
		writeError(w, http.StatusServiceUnavailable, "password reset unavailable")
	case err != nil:
		h.internalError(w, r, err)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

type resetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	err := h.svc.ResetPassword(r.Context(), ResetPasswordInput{
		Token:           req.Token,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	var invalid *ValidationError
	switch {
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid input", Fields: invalid.Fields})
	case errors.Is(err, ErrInvalidResetToken):
		writeError(w, http.StatusBadRequest, "invalid or expired token")
	case err != nil:
		h.internalError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, _ := CurrentUser(r.Context())
	token, err := h.svc.ChangePassword(r.Context(), user, ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	var invalid *ValidationError
	switch {
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid input", Fields: invalid.Fields})
	case errors.Is(err, ErrWrongPassword):
		writeError(w, http.StatusForbidden, "current password is incorrect")
	case errors.Is(err, ErrAccountLocked):
		writeError(w, http.StatusLocked, "account locked")
	case err != nil:
		h.internalError(w, r, err)
	default:
		h.setCookie(w, token)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, _ := CurrentUser(r.Context())
	writeJSON(w, http.StatusOK, userResponse{Email: user.Email})
}

func (h *Handler) sessionToken(r *http.Request) string {
	c, err := r.Cookie(h.cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (h *Handler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	err := json.NewDecoder(r.Body).Decode(v)
	var tooLarge *http.MaxBytesError
	switch {
	case err == nil:
		return true
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
	default:
		writeError(w, http.StatusBadRequest, "invalid json")
	}
	return false
}

func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "auth request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	httpx.WriteInternalError(w, r)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
