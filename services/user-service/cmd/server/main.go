package main

import (
	"log/slog"
	"os"

	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
)

func main() {
	cfg := pkg.Config{}

	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
}
