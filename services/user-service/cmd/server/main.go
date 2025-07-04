package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"
)

// main initializes and runs the user service server, setting up configuration, logging, database and Redis connections, and handling graceful shutdown.
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

	rdbOpts, err := redis.ParseURL(helpers.GetEnvOrDefault("REDIS_URL", ""))
	if err != nil {
		slog.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rdbOpts)
	refreshTokenStore := auth.RefreshTokenStore{Redis: rdb}

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

	userRepo := repository.NewUserRepository(dbSession)
	userController := controller.NewUserController(userRepo, refreshTokenStore)

	userPath, userHandler := userv1connect.NewUserServiceHandler(
		userController,
		connect.WithInterceptors(auth.AuthInterceptor()),
	)

	mux := http.NewServeMux()
	mux.Handle(userPath, userHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.UserServer.Port),
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

	// Kafka event handler
	go func() {
		eventHandler := map[string]func([]byte) error{
			"user.followed": func(b []byte) error {
				// TODO - use outbox event
				var event userv1.FollowedEvent
				if err := protojson.Unmarshal(b, &event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}
				_, err := userController.IncrementFollowingAndFollowerCount(ctx,
					connect.NewRequest(&userv1.IncrementFollowingAndFollowerCountRequest{
						FollowedEvent: &event,
					}))
				return err
			},
			"user.unfollowed": func(b []byte) error {
				var event userv1.UnfollowedEvent
				if err := protojson.Unmarshal(b, &event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}
				_, err := userController.DecrementFollowingAndFollowerCount(ctx,
					connect.NewRequest(&userv1.DecrementFollowingAndFollowerCountRequest{
						UnfollowedEvent: &event,
					}))
				return err
			},
		}

		for {
			msg, err := kafkaReader.ReadMessage(ctx)
			if ctx.Err() != nil {
				slog.Info("Receive interrupted due to shutdown, exiting message loop...")
				return
			}
			if err != nil {
				slog.Error("failed to read message", "error", err)
				continue
			}

			parts := strings.Split(string(msg.Key), ":")
			if len(parts) != 2 {
				slog.Warn("invalid message key", "key", string(msg.Key))
				continue
			}

			handler, ok := eventHandler[parts[0]]
			if !ok {
				slog.Warn("no handler for message key", "key", parts[0])
				continue
			}
			if err := handler(msg.Value); err != nil {
				slog.Error("handler error", "key", parts[0], "err", err)
			}
		}
	}()

	slog.Info("starting ConnectRPC server", "address", server.Addr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("service shutdown complete")
}
