package queue

import (
	"context"
	"fmt"

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
			fmt.Println("Handler error:", err)
			// Decide: retry, skip, log, send to DLQ, etc.
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
