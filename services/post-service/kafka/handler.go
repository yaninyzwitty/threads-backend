package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/segmentio/kafka-go"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/post-service/controller"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
)

func StartKafkaConsumer(ctx context.Context, kafkaReader *kafka.Reader, postController *controller.PostController) {
	slog.Info("starting kafka consumer...")
	eventHandlers := map[string]func([]byte) error{
		"post.created": func(b []byte) error {
			var event postsv1.OutboxEvent
			if err := protojson.Unmarshal(b, &event); err != nil {
				return fmt.Errorf("failed to unmarshal OutboxEvent JSON: %w", err)
			}

			var postCreatedEvent postsv1.Post
			if err := protojson.Unmarshal([]byte(event.Payload), &postCreatedEvent); err != nil {
				return fmt.Errorf("failed to unmarshal post created event payload: %w", err)
			}

			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				_, err := postController.CreatePostIndexedByUser(egCtx,
					connect.NewRequest(&postsv1.CreatePostIndexedByUserRequest{
						Post: &postCreatedEvent,
					}))
				return err
			})

			eg.Go(func() error {
				_, err := postController.InitializePostEngagements(egCtx, connect.NewRequest(&postsv1.InitializePostEngagementsRequest{
					PostId: postCreatedEvent.Id,
				}))
				return err
			})
			return eg.Wait()

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
