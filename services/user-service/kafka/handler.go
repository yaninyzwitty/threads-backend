package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/segmentio/kafka-go"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/controller"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
)

func StartKafkaConsumer(ctx context.Context, kafkaReader *kafka.Reader, userController *controller.UserController) {
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

			return eg.Wait()
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

			return eg.Wait()
		},

		"user.created": func(b []byte) error {
			var event userv1.OutboxEvent
			if err := protojson.Unmarshal(b, &event); err != nil {
				return fmt.Errorf("failed to unmarshal OutboxEvent JSON: %w", err)
			}

			var createdEvent userv1.User
			if err := protojson.Unmarshal([]byte(event.Payload), &createdEvent); err != nil {
				return fmt.Errorf("failed to unmarshal User payload: %w", err)
			}

			_, err := userController.InsertFollowerCounts(ctx,
				connect.NewRequest(&userv1.InsertFollowerCountsRequest{
					UserId: createdEvent.Id,
				}),
			)
			return err
		},
	}

	go func() {
		for {
			msg, err := kafkaReader.ReadMessage(ctx)
			if ctx.Err() != nil {
				slog.Info("Kafka consumer shutting down...")
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
				continue
			}

			if err := kafkaReader.CommitMessages(ctx, msg); err != nil {
				slog.Error("failed to commit kafka message", "error", err)
			}
		}
	}()
}
