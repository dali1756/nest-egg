package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"nest-egg/backend/db/migrations"
	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/platform/config"
	"nest-egg/backend/internal/platform/db"
	"nest-egg/backend/internal/platform/health"
	"nest-egg/backend/internal/platform/middleware"
	"nest-egg/backend/internal/platform/ratelimit"
	"nest-egg/backend/internal/platform/requestid"
	"nest-egg/backend/internal/rate"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	slog.SetDefault(newLogger(os.Stdout, cfg))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()

	cc := pool.Config().ConnConfig
	slog.Info("database connected", "host", cc.Host, "port", cc.Port, "database", cc.Database, "user", cc.User)

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	authSvc, err := auth.NewService(auth.NewRepository(pool), auth.Config{}, slog.Default())
	if err != nil {
		return fmt.Errorf("set up auth: %w", err)
	}
	currencies := currency.NewService(currency.NewRepository(pool))
	rates := rate.NewService(rate.NewRepository(pool), rate.NewCBCProvider(rate.CBCURL), currencies)

	h := handlers{
		health:    health.NewHandler(pool),
		auth:      auth.NewHandler(authSvc, auth.HandlerConfig{CookieSecure: cfg.CookieSecure}, slog.Default()),
		currency:  currency.NewHandler(currencies),
		rate:      rate.NewHandler(rates),
		rateLimit: newRateLimit(cfg),
	}
	srv := newServer(cfg.HTTPAddr, newHandler(h, cfg))
	return runServer(ctx, srv, rate.NewUpdater(rates, slog.Default()).Run)
}

const maxBodyBytes = 1 << 20

func newRateLimit(cfg config.Config) func(http.Handler) http.Handler {
	return ratelimit.New(10, 6*time.Second, cfg.TrustedProxies).Middleware
}

func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func newHandler(h handlers, cfg config.Config) http.Handler {
	public, _ := newRouter(h)
	return withMiddleware(public, cfg)
}

func withMiddleware(next http.Handler, cfg config.Config) http.Handler {
	var h http.Handler = http.MaxBytesHandler(next, maxBodyBytes)
	h = http.NewCrossOriginProtection().Handler(h)
	h = middleware.SecurityHeaders(h, cfg.CookieSecure)
	h = middleware.Recover(h)
	h = middleware.AccessLog(h, func(r *http.Request) string { return ratelimit.ClientIP(r, cfg.TrustedProxies) })
	return requestid.Middleware(h)
}

func newLogger(w io.Writer, cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var handler slog.Handler
	switch cfg.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(requestid.NewLogHandler(handler))
}

func runServer(ctx context.Context, srv *http.Server, jobs ...func(context.Context)) error {
	jobCtx, cancelJobs := context.WithCancel(context.WithoutCancel(ctx))
	var wg sync.WaitGroup
	defer func() {
		cancelJobs()
		wg.Wait()
	}()
	for _, job := range jobs {
		wg.Go(func() { job(jobCtx) })
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
