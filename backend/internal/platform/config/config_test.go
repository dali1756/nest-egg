package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

const validSecret = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SESSION_SECRET", validSecret)
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

func TestLoadDecodesSessionSecret(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := make([]byte, 32)
	for i := range want {
		want[i] = byte(i)
	}
	if !bytes.Equal(cfg.SessionSecret, want) {
		t.Errorf("SessionSecret = %x, want %x", cfg.SessionSecret, want)
	}
}

func TestLoadRejectsInvalidSessionSecret(t *testing.T) {
	tests := map[string]string{
		"missing":  "",
		"not hex":  "zz" + validSecret[2:],
		"31 bytes": validSecret[:62],
	}
	for name, secret := range tests {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			t.Setenv("SESSION_SECRET", secret)

			_, err := Load()
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if secret != "" && strings.Contains(err.Error(), secret) {
				t.Errorf("error leaks SESSION_SECRET: %v", err)
			}
		})
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
