package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type MessageHandler func(msg kafka.Message) error

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(reader *kafka.Reader) *Consumer {
	return &Consumer{reader: reader}
}

func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// context canceled -> graceful shutdown
				return nil
			}
			return fmt.Errorf("error fetching message: %w", err)
		}

		// Handle the message
		if err := handler(m); err != nil {
			slog.Error("Message handler failed", "error", err, "topic", m.Topic, "partition", m.Partition, "offset", m.Offset)
			// TODO: Implement retry logic or dead letter queue
			continue // Skip committing this message
		}

		// Commit offset after successful handling
		if err := c.reader.CommitMessages(ctx, m); err != nil {
			return fmt.Errorf("failed to commit message: %w", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
