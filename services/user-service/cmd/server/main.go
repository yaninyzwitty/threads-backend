package main

import (
	"log/slog"
	"os"

	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
)

func main() {
	cfg := pkg.Config{}

	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)

	}

	if err := snowflake.InitSonyFlake(); err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}

}
