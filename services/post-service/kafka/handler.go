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
			slog.Info("handling post.created event...")
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

				slog.Info("creating post indexed by user...", "post_id", postCreatedEvent.Id)
				_, err := postController.CreatePostIndexedByUser(egCtx,
					connect.NewRequest(&postsv1.CreatePostIndexedByUserRequest{
						Post: &postCreatedEvent,
					}))
				return err
			})

			eg.Go(func() error {
				slog.Info("initializing post engagements...", "post_id", postCreatedEvent.Id)

				_, err := postController.InitializePostEngagements(egCtx, connect.NewRequest(&postsv1.InitializePostEngagementsRequest{
					PostId: postCreatedEvent.Id,
				}))
				return err
			})
			return eg.Wait()

		},
		"like.created": func(b []byte) error {
			slog.Info("handling like.created event...")

			var event postsv1.OutboxEvent
			if err := protojson.Unmarshal(b, &event); err != nil {
				return fmt.Errorf("failed to unmarshal OutboxEvent: %w", err)
			}

			var like postsv1.Like
			if err := protojson.Unmarshal([]byte(event.Payload), &like); err != nil {
				return fmt.Errorf("failed to unmarshal Like payload: %w", err)
			}

			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				slog.Info("inserting user like...")
				_, err := postController.CreateLikeByUser(egCtx, connect.NewRequest(&postsv1.CreateLikeByUserRequest{
					Like: &like,
				}))
				return err
			})

			eg.Go(func() error {
				slog.Info("incrementing post likes...")
				_, err := postController.IncrementPostLikes(egCtx, connect.NewRequest(&postsv1.IncrementPostLikesRequest{
					PostId: like.PostId,
				}))
				return err
			})

			return eg.Wait()
		},
		"comment.created": func(b []byte) error {
			// TODO implement update for post_engagements table
			return nil
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
