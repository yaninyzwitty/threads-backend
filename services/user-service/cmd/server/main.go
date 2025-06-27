package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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

	userRepo := repository.NewUserRepository(session)
	userController := controller.NewUserController(userRepo)

	userPath, userHandler := userv1connect.NewUserServiceHandler(userController)
	mux := http.NewServeMux()
	mux.Handle(userPath, userHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sig := <-quit
		slog.Info("received shutdown signal", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
		} else {
			slog.Info("server shutdown gracefully")
		}
	}()
}
