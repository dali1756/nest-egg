package main

import (
	"net/http"

	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/ledger"
	"nest-egg/backend/internal/platform/health"
	"nest-egg/backend/internal/platform/httpx"
	"nest-egg/backend/internal/rate"
)

type handlers struct {
	health    *health.Handler
	auth      *auth.Handler
	currency  *currency.Handler
	rate      *rate.Handler
	ledger    *ledger.Handler
	rateLimit func(http.Handler) http.Handler
}

func (h handlers) limit(f http.HandlerFunc) http.Handler {
	return h.rateLimit(f)
}

func addRoutes(public, protected *http.ServeMux, h handlers) {
	public.HandleFunc("GET /api/health", h.health.Get)
	public.Handle("POST /api/auth/register", h.limit(h.auth.Register))
	public.Handle("POST /api/auth/login", h.limit(h.auth.Login))
	public.HandleFunc("POST /api/auth/logout", h.auth.Logout)
	public.Handle("POST /api/auth/password/forgot", h.limit(h.auth.ForgotPassword))
	public.Handle("POST /api/auth/password/reset", h.limit(h.auth.ResetPassword))

	protected.HandleFunc("GET /api/auth/me", h.auth.Me)
	protected.Handle("POST /api/auth/password/change", h.limit(h.auth.ChangePassword))
	protected.Handle("GET /api/currencies", httpx.Handler(h.currency.List))
	protected.Handle("GET /api/rates", httpx.Handler(h.rate.Get))
	protected.Handle("GET /api/categories", httpx.Handler(h.ledger.Categories))
	protected.Handle("GET /api/ledger-entries", httpx.Handler(h.ledger.ListEntries))
	protected.Handle("POST /api/ledger-entries", httpx.Handler(h.ledger.CreateEntry))
	protected.Handle("GET /api/ledger-entries/{id}", httpx.Handler(h.ledger.GetEntry))
	protected.Handle("PUT /api/ledger-entries/{id}", httpx.Handler(h.ledger.UpdateEntry))
	protected.Handle("DELETE /api/ledger-entries/{id}", httpx.Handler(h.ledger.DeleteEntry))
}

func newRouter(h handlers) (public, protected *http.ServeMux) {
	public, protected = http.NewServeMux(), http.NewServeMux()
	addRoutes(public, protected, h)
	public.Handle("/api/", h.auth.RequireUser(protected))
	return public, protected
}
