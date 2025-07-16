package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/threads-go-backend/gen/comment/v1/commentsv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/comment-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/services/comment-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	cfg := pkg.Config{}
	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := snowflake.InitSonyFlake(); err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}

	db := database.NewAstraDB()
	sessionCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	dbSession, err := db.Connect(sessionCtx, &database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("ASTRA_DB_TOKEN", ""),
	}, 10*time.Second)
	if err != nil {
		slog.Error("failed to connect to astra db", "error", err)
		os.Exit(1)
	}
	defer dbSession.Close()

	kafkaReader := queue.NewKafkaReader(queue.Config{
		Brokers:  cfg.Queue.Brokers,
		Topic:    cfg.Queue.Topic,
		GroupID:  cfg.Queue.GroupID,
		Username: cfg.Queue.Username,
		Password: helpers.GetEnvOrDefault("KAFKA_PASSWORD", ""),
	})
	defer kafkaReader.Close()

	commentsRepo := repository.NewCommentRepository(dbSession)
	commentsController := controller.NewCommentController(commentsRepo)
	commentsPath, commentsHandler := commentsv1connect.NewCommentServiceHandler(
		commentsController,
		connect.WithInterceptors(auth.AuthInterceptor()),
	)

	mux := http.NewServeMux()
	mux.Handle(commentsPath, commentsHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.CommentServer.Port),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		slog.Info("received shutdown signal", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
		} else {
			slog.Info("server shutdown gracefully")

		}
		cancel()
	}()

	// TODO-start kafka consumer here
	slog.Info("starting ConnectRPC server", "address", server.Addr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("service shutdown complete")

}
