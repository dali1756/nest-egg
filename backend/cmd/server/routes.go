package main

import (
	"net/http"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/fx"
	"nest-egg/backend/internal/ledger"
	"nest-egg/backend/internal/platform/health"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/portfolio"
	"nest-egg/backend/internal/rate"
)

type handlers struct {
	health    *health.Handler
	auth      *auth.Handler
	currency  *currency.Handler
	rate      *rate.Handler
	ledger    *ledger.Handler
	fx        *fx.Handler
	portfolio *portfolio.Handler
	rateLimit func(http.Handler) http.Handler
}

func (h handlers) limit(next http.Handler) http.Handler {
	return h.rateLimit(next)
}

func addRoutes(public, protected *http.ServeMux, h handlers) {
	public.HandleFunc("GET /api/health", h.health.Get)
	public.Handle("POST /api/auth/register", h.limit(httpx.Handler(h.auth.Register)))
	public.Handle("POST /api/auth/login", h.limit(httpx.Handler(h.auth.Login)))
	public.Handle("POST /api/auth/logout", httpx.Handler(h.auth.Logout))
	public.Handle("POST /api/auth/password/forgot", h.limit(httpx.Handler(h.auth.ForgotPassword)))
	public.Handle("POST /api/auth/password/reset", h.limit(httpx.Handler(h.auth.ResetPassword)))

	protected.Handle("GET /api/auth/me", httpx.Handler(h.auth.Me))
	protected.Handle("POST /api/auth/password/change", h.limit(httpx.Handler(h.auth.ChangePassword)))
	protected.Handle("GET /api/currencies", httpx.Handler(h.currency.List))
	protected.Handle("GET /api/rates", httpx.Handler(h.rate.Get))
	protected.Handle("GET /api/categories", httpx.Handler(h.ledger.ListCategories))
	protected.Handle("GET /api/ledger-entries", httpx.Handler(h.ledger.ListEntries))
	protected.Handle("POST /api/ledger-entries", httpx.Handler(h.ledger.CreateEntry))
	protected.Handle("GET /api/ledger-entries/{id}", httpx.Handler(h.ledger.GetEntry))
	protected.Handle("PUT /api/ledger-entries/{id}", httpx.Handler(h.ledger.UpdateEntry))
	protected.Handle("DELETE /api/ledger-entries/{id}", httpx.Handler(h.ledger.DeleteEntry))
	protected.Handle("GET /api/fx-trades", httpx.Handler(h.fx.ListTrades))
	protected.Handle("POST /api/fx-trades", httpx.Handler(h.fx.CreateTrade))
	protected.Handle("GET /api/fx-trades/preview", httpx.Handler(h.fx.PreviewTrade))
	protected.Handle("GET /api/fx-trades/{id}", httpx.Handler(h.fx.GetTrade))
	protected.Handle("PUT /api/fx-trades/{id}", httpx.Handler(h.fx.UpdateTrade))
	protected.Handle("DELETE /api/fx-trades/{id}", httpx.Handler(h.fx.DeleteTrade))
	protected.Handle("GET /api/portfolio/summary", httpx.Handler(h.portfolio.Summary))
	protected.Handle("GET /api/opening-balances", httpx.Handler(h.portfolio.ListOpenings))
	protected.Handle("GET /api/opening-balances/preview", httpx.Handler(h.portfolio.PreviewOpening))
	protected.Handle("GET /api/opening-balances/{currency}", httpx.Handler(h.portfolio.GetOpening))
	protected.Handle("PUT /api/opening-balances/{currency}", httpx.Handler(h.portfolio.SetOpening))
	protected.Handle("DELETE /api/opening-balances/{currency}", httpx.Handler(h.portfolio.DeleteOpening))
}

func newRouter(h handlers) (public, protected *http.ServeMux) {
	public, protected = http.NewServeMux(), http.NewServeMux()
	addRoutes(public, protected, h)
	public.Handle("/api/", h.auth.RequireUser(protected))
	return public, protected
}
