package config

import "log/slog"

func logConfig(cfg Config) {
	slog.Info("config loaded", "url", cfg.DatabaseURL)
	slog.Debug("env", "DATABASE_URL", databaseURL)
	slog.Info("login", "password", password)
	slog.Warn("provider", "apiKey", key)
	slog.Error("auth failed", "token", tok)
	slog.InfoContext(ctx, "secret rotated", "secret", s)
	slog.Info("database connected", "host", host, "user", user)
}
