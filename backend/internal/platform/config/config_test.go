package config

import (
	"log/slog"
	"testing"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, "127.0.0.1:8080")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true by default")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load without DATABASE_URL: want error, got nil")
	}
}

func TestLoadRejectsInvalidLogFormat(t *testing.T) {
	setRequired(t)
	t.Setenv("LOG_FORMAT", "xml")

	if _, err := Load(); err == nil {
		t.Fatal("Load with LOG_FORMAT = xml: want error, got nil")
	}
}

func TestLoadCookieSecure(t *testing.T) {
	tests := map[string]bool{"false": false, "true": true}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			setRequired(t)
			t.Setenv("COOKIE_SECURE", value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CookieSecure != want {
				t.Errorf("CookieSecure = %v, want %v", cfg.CookieSecure, want)
			}
		})
	}
}

func TestLoadRejectsInvalidCookieSecure(t *testing.T) {
	setRequired(t)
	t.Setenv("COOKIE_SECURE", "maybe")

	if _, err := Load(); err == nil {
		t.Fatal("Load with COOKIE_SECURE = maybe: want error, got nil")
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	setRequired(t)
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want none by default", cfg.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Errorf("TrustedProxies = %v, want 2 entries", cfg.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "localhost")
	if _, err := Load(); err == nil {
		t.Error("Load with TRUSTED_PROXIES = localhost: want error, got nil")
	}
}
