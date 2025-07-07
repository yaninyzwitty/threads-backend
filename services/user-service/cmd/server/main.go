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
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
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

	defer slog.Info("service shutdown complete")

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

	userRepo := repository.NewUserRepository(dbSession, rdb)
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
		eventHandlers := map[string]func([]byte) error{
			"user.followed": func(b []byte) error {
				var event userv1.OutboxEvent
				if err := protojson.Unmarshal(b, &event); err != nil {
					return fmt.Errorf("failed to unmarshal OutboxEvent JSON: %w", err)
				}

				var followedEvent userv1.FollowedEvent
				if err := protojson.Unmarshal([]byte(event.Payload), &followedEvent); err != nil {
					return fmt.Errorf("failed to unmarshal FollowedEvent payload: %w", err)
				}

				eg, egCtx := errgroup.WithContext(ctx)

				eg.Go(func() error {
					_, err := userController.IncrementFollowingAndFollowerCount(egCtx,
						connect.NewRequest(&userv1.IncrementFollowingAndFollowerCountRequest{
							FollowedEvent: &followedEvent,
						}))
					return err
				})

				eg.Go(func() error {
					_, err := userController.FollowUserCached(egCtx,
						connect.NewRequest(&userv1.FollowUserCachedRequest{
							UserId:      followedEvent.UserId,
							FollowingId: followedEvent.FollowingId,
						}))
					return err
				})

				if err := eg.Wait(); err != nil {
					slog.Error("follow event handling failed", "err", err)
					return err
				}
				return nil
			},

			"user.unfollowed": func(b []byte) error {
				var event userv1.OutboxEvent
				if err := protojson.Unmarshal(b, &event); err != nil {
					return fmt.Errorf("failed to unmarshal OutboxEvent JSON: %w", err)
				}

				var unfollowedEvent userv1.UnfollowedEvent
				if err := protojson.Unmarshal([]byte(event.Payload), &unfollowedEvent); err != nil {
					return fmt.Errorf("failed to unmarshal UnfollowedEvent payload: %w", err)
				}

				eg, egCtx := errgroup.WithContext(ctx)
				eg.Go(func() error {
					_, err := userController.DecrementFollowingAndFollowerCount(egCtx,
						connect.NewRequest(&userv1.DecrementFollowingAndFollowerCountRequest{
							UnfollowedEvent: &unfollowedEvent,
						}))
					return err
				})

				eg.Go(func() error {
					_, err := userController.UnfollowUserCached(egCtx,
						connect.NewRequest(&userv1.UnfollowUserCachedRequest{
							UserId:      unfollowedEvent.UserId,
							FollowingId: unfollowedEvent.FollowingId,
						}))
					return err
				})

				if err := eg.Wait(); err != nil {
					slog.Error("unfollow event handling failed", "err", err)
					return err
				}
				return nil
			},
			"user.created": func(b []byte) error {
				// Step 1: Unmarshal OutboxEvent
				var event userv1.OutboxEvent
				if err := protojson.Unmarshal(b, &event); err != nil {
					return fmt.Errorf("failed to unmarshal OutboxEvent JSON: %w", err)
				}

				// Step 2: Unmarshal Payload (User) from the event
				var createdEvent userv1.User
				if err := protojson.Unmarshal([]byte(event.Payload), &createdEvent); err != nil {
					return fmt.Errorf("failed to unmarshal User payload: %w", err)
				}

				// Step 3: Call InsertFollowerCounts
				_, err := userController.InsertFollowerCounts(
					ctx,
					connect.NewRequest(&userv1.InsertFollowerCountsRequest{
						UserId: createdEvent.Id,
					}),
				)
				if err != nil {
					return fmt.Errorf("failed to insert follower counts: %w", err)
				}

				return nil
			},
		}

		for {
			msg, err := kafkaReader.ReadMessage(ctx)
			if ctx.Err() != nil {
				slog.Info("Receive interrupted, shutting down message loop...")
				return
			}
			if err != nil {
				slog.Error("failed to read Kafka message", "error", err)
				continue
			}

			parts := strings.Split(string(msg.Key), ":")
			if len(parts) != 2 {
				slog.Warn("invalid Kafka message key format", "key", string(msg.Key))
				continue
			}

			eventKey := parts[0]
			handler, ok := eventHandlers[eventKey]
			if !ok {
				slog.Warn("no handler found for event key", "key", eventKey)
				continue
			}

			if err := handler(msg.Value); err != nil {
				slog.Error("event handler failed", "key", eventKey, "error", err)
			}
		}
	}()

	slog.Info("starting ConnectRPC server", "address", server.Addr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
