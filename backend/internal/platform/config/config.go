package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"

	"nest-egg/backend/internal/platform/ratelimit"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
	LogFormat   string

	CookieSecure   bool
	TrustedProxies []netip.Prefix
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    getenv("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogFormat:   getenv("LOG_FORMAT", "text"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
	}
	switch cfg.LogFormat {
	case "text", "json":
	default:
		return Config{}, fmt.Errorf("invalid LOG_FORMAT %q: must be text or json", cfg.LogFormat)
	}
	secure, err := strconv.ParseBool(getenv("COOKIE_SECURE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = secure

	cfg.TrustedProxies, err = ratelimit.ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, fmt.Errorf("parse TRUSTED_PROXIES: %w", err)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
