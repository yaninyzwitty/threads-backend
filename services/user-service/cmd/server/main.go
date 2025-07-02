package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// main initializes and runs the user service server, setting up configuration, logging, database and Redis connections, and handling graceful shutdown.
func main() {

	// set up logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	// Init helpers (Redis + JWT)
	// helpers.InitHelpers()

	// Load config
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

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	refreshTokenStore := auth.RefreshTokenStore{
		Redis: rdb,
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

	// create the user service address
	userServiceAddr := fmt.Sprintf(":%d", cfg.UserServer.Port)

	slog.Info("user service address", "address", userServiceAddr)

	// inject the repository and controller DDD
	userRepo := repository.NewUserRepository(session)
	userController := controller.NewUserController(userRepo, refreshTokenStore)

	//build http server from service implementation
	userPath, userHandler := userv1connect.NewUserServiceHandler(
		userController,
		connect.WithInterceptors(auth.AuthInterceptor()))
	mux := http.NewServeMux()
	mux.Handle(userPath, userHandler)

	server := &http.Server{
		Addr:    userServiceAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Graceful shutdown
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

	// Start Connectrpc server
	slog.Info("starting ConnectRPC server", "address", userServiceAddr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	wg.Wait()
	slog.Info("service shutdown complete")
}
