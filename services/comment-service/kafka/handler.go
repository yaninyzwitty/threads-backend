package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/segmentio/kafka-go"
	commentsv1 "github.com/yaninyzwitty/threads-go-backend/gen/comment/v1"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/comment-service/controller"
	"google.golang.org/protobuf/encoding/protojson"
)

func StartKafkaConsumer(ctx context.Context, kafkaReader *kafka.Reader, commentsController *controller.CommentController) {
	slog.Info("starting kafka consumer...")

	eventHandler := map[string]func([]byte) error{
		"comment.created": func(b []byte) error {
			if len(b) == 0 {
				return fmt.Errorf("empty payload")
			}

			var outboxMessage postsv1.OutboxEvent
			if err := protojson.Unmarshal(b, &outboxMessage); err != nil {
				return fmt.Errorf("failed to unmarshal OutboxEvent: %w", err)
			}

			var createdComment commentsv1.Comment
			if err := protojson.Unmarshal([]byte(outboxMessage.Payload), &createdComment); err != nil {
				return fmt.Errorf("failed to unmarshal comment payload: %w", err)
			}

			res, err := commentsController.HandleCommentsUpsert(ctx, connect.NewRequest(&commentsv1.HandleCommentsUpsertRequest{
				Comment: &createdComment,
			}))
			if err != nil {
				return fmt.Errorf("upsert error: %w", err)
			}
			if !res.Msg.Upserted {
				return fmt.Errorf("comment not marked as upserted")
			}

			slog.Info("successfully upserted comment")
			return nil
		},
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic in Kafka consumer", "panic", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Kafka consumer shutting down...")
				return
			default:
			}

			msg, err := kafkaReader.ReadMessage(ctx)
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
			handler, ok := eventHandler[eventKey]
			if !ok {
				slog.Warn("no handler found for event key", "key", eventKey)
				continue
			}

			if err := handler(msg.Value); err != nil {
				slog.Error("event handler failed", "key", eventKey, "error", err)
				continue
			}

			if err := kafkaReader.CommitMessages(ctx, msg); err != nil {
				slog.Error("failed to commit Kafka message", "error", err)
			}
		}
	}()
}
