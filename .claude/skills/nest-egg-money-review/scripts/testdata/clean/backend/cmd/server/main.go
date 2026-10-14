package main

import (
	"log/slog"
	"os"
	_ "time/tzdata"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
