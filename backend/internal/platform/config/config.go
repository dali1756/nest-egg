package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

const minSessionSecretLen = 32

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
	LogFormat   string

	SessionSecret []byte
	CookieSecure  bool
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
	secret, err := hex.DecodeString(os.Getenv("SESSION_SECRET"))
	if err != nil {
		return Config{}, errors.New("SESSION_SECRET must be hex (generate with: openssl rand -hex 32)")
	}
	if len(secret) < minSessionSecretLen {
		return Config{}, fmt.Errorf("SESSION_SECRET must decode to at least %d bytes (generate with: openssl rand -hex 32)", minSessionSecretLen)
	}
	cfg.SessionSecret = secret

	cfg.CookieSecure, err = strconv.ParseBool(getenv("COOKIE_SECURE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
