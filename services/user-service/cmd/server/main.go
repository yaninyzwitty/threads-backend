package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
)

func main() {
	cfg := pkg.Config{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)

	}

	if err := snowflake.InitSonyFlake(); err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}

	if err := godotenv.Load(); err != nil {
		slog.Error("failed to load .env file", "error", err)
		os.Exit(1)
	}

	db := database.NewAstraDB()

	astraCfg := database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("ASTRA_DB_TOKEN", ""),
	}

	session, err := db.Connect(ctx, &astraCfg, 10*time.Second)

	if err != nil {
		slog.Error("failed to connect to astra db", "error", err)
		os.Exit(1)
	}

	defer session.Close()

}
