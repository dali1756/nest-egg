package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"nest-egg/backend/internal/platform/httpx"
)

type HandlerConfig struct {
	CookieSecure bool
}

type Handler struct {
	svc          *Service
	cookieSecure bool
	cookieName   string
}

func NewHandler(svc *Service, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:          svc,
		cookieSecure: cfg.CookieSecure,
		cookieName:   sessionCookieName(cfg.CookieSecure),
	}
}

func sessionCookieName(secure bool) string {
	if secure {
		return "__Host-nest_egg_session"
	}
	return "nest_egg_session"
}

var errorStatuses = []struct {
	err     error
	status  int
	message string
}{
	{ErrNoSession, http.StatusUnauthorized, "unauthorized"},
	{ErrInvalidCredentials, http.StatusUnauthorized, "invalid credentials"},
	{ErrEmailTaken, http.StatusConflict, "email already registered"},
	{ErrAccountLocked, http.StatusLocked, "account locked"},
	{ErrWrongPassword, http.StatusForbidden, "current password is incorrect"},
	{ErrInvalidResetToken, http.StatusBadRequest, "invalid or expired token"},
	{ErrResetUnavailable, http.StatusServiceUnavailable, "password reset unavailable"},
}

func toHTTPError(err error) error {
	for _, s := range errorStatuses {
		if errors.Is(err, s.err) {
			return &httpx.StatusError{Status: s.status, Message: s.message}
		}
	}
	return err
}

type ctxKey struct{}

func (h *Handler) RequireUser(next http.Handler) http.Handler {
	return httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
		token := h.sessionToken(r)
		user, err := h.svc.Authenticate(r.Context(), token)
		if errors.Is(err, ErrNoSession) && token != "" {
			h.clearCookie(w)
		}
		if err != nil {
			return toHTTPError(err)
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		return nil
	})
}

// WithUser 回傳帶有登入使用者的 context，CurrentUser 從這裡讀
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func CurrentUser(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

var errNoUser = errors.New("no user in request context")

func UserID(r *http.Request) (int64, error) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		return 0, errNoUser
	}
	return u.ID, nil
}

type userResponse struct {
	Email string `json:"email"`
}

type registerRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) error {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, token, err := h.svc.Register(r.Context(), RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		return toHTTPError(err)
	}
	h.setCookie(w, token)
	return httpx.WriteJSON(w, http.StatusCreated, userResponse{Email: user.Email})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, token, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		return toHTTPError(err)
	}
	h.setCookie(w, token)
	return httpx.WriteJSON(w, http.StatusOK, userResponse{Email: user.Email})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	h.clearCookie(w)
	if err := h.svc.Logout(r.Context(), h.sessionToken(r)); err != nil {
		return toHTTPError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) error {
	var req forgotPasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.svc.RequestPasswordReset(r.Context(), req.Email); err != nil {
		return toHTTPError(err)
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

type resetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) error {
	var req resetPasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	err := h.svc.ResetPassword(r.Context(), ResetPasswordInput{
		Token:           req.Token,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		return toHTTPError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	var req changePasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, _ := CurrentUser(r.Context())
	token, err := h.svc.ChangePassword(r.Context(), user, ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		return toHTTPError(err)
	}
	h.setCookie(w, token)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) error {
	user, _ := CurrentUser(r.Context())
	return httpx.WriteJSON(w, http.StatusOK, userResponse{Email: user.Email})
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
