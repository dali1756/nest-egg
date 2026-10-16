package auth

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/logout"
	_ "github.com/aarondl/authboss/v3/register"
)

const MountPath = "/api/auth"

type Config struct {
	SessionSecret []byte
	CookieSecure  bool
}

type Auth struct {
	ab  *authboss.Authboss
	log *slog.Logger
}

func New(cfg Config, storer authboss.CreatingServerStorer, log *slog.Logger) (*Auth, error) {
	sessionStorer, err := newSessionStorer(cfg.SessionSecret, cfg.CookieSecure)
	if err != nil {
		return nil, fmt.Errorf("create session storer: %w", err)
	}

	ab := authboss.New()
	ab.Config.Paths.Mount = MountPath
	ab.Config.Paths.AuthLoginOK = "/"
	ab.Config.Paths.RegisterOK = "/"
	ab.Config.Paths.LogoutOK = "/login"
	ab.Config.Modules.LogoutMethod = http.MethodPost

	ab.Config.Storage.Server = storer
	ab.Config.Storage.SessionState = sessionStorer

	ab.Config.Core.ViewRenderer = defaults.JSONRenderer{}
	defaults.SetCore(&ab.Config, true, false)
	ab.Config.Core.Logger = slogLogger{l: log}
	ab.Config.Core.ErrorHandler = newErrorHandler(log)
	ab.Config.Core.Responder = newStatusResponder(defaults.NewResponder(ab.Config.Core.ViewRenderer))
	redirector := defaults.NewRedirector(ab.Config.Core.ViewRenderer, authboss.FormValueRedirect)
	redirector.CorceRedirectTo200 = true
	ab.Config.Core.Redirector = redirector

	if err := ab.Init("auth", "register", "logout"); err != nil {
		return nil, fmt.Errorf("init authboss: %w", err)
	}
	return &Auth{ab: ab, log: log}, nil
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return a.ab.LoadClientStateMiddleware(next)
}
