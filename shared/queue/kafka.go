package queue

import (
	"time"

	"github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers []string
	Topic   string
	GroupID string
}

// NewKafkaConfig returns a Kafka writer configured as a producer for the specified brokers and topic.
// The writer uses least-bytes balancing, requires acknowledgments from all replicas, operates asynchronously, and batches messages with a 10 millisecond timeout.

func NewKafkaConfig(cfg Config) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		Async:        true,
		BatchTimeout: 10 * time.Millisecond,
	}
}

// NewKafkaReader returns a Kafka reader configured as a consumer for the specified brokers, topic, and consumer group.
// The reader fetches messages in batches between 10KB and 10MB, starts from the earliest offset, and commits offsets every second.

func NewKafkaReader(cfg Config) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.Topic,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})
}
