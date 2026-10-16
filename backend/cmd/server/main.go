package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"nest-egg/backend/db/migrations"
	"nest-egg/backend/internal/auth"
	"nest-egg/backend/internal/platform/config"
	"nest-egg/backend/internal/platform/db"
	"nest-egg/backend/internal/platform/health"
	"nest-egg/backend/internal/platform/middleware"
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
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var handler slog.Handler
	switch cfg.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))

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

	a, err := auth.New(auth.Config{SessionSecret: cfg.SessionSecret, CookieSecure: cfg.CookieSecure}, auth.NewRepository(pool), slog.Default())
	if err != nil {
		return fmt.Errorf("set up auth: %w", err)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newHandler(pool, a),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
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

func newHandler(pinger health.Pinger, a *auth.Auth) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health.NewHandler(pinger).Get)
	a.Routes(mux)
	return middleware.AccessLog(http.NewCrossOriginProtection().Handler(a.Middleware(mux)))
}
