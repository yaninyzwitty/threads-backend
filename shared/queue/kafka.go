package queue

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type Config struct {
	Brokers  []string
	Topic    string
	GroupID  string
	Username string
	Password string
}

// newDialer returns a Kafka dialer configured with SASL/PLAIN authentication and TLS using the provided configuration.
func newDialer(cfg Config) *kafka.Dialer {
	mechanism := plain.Mechanism{
		Username: cfg.Username,
		Password: cfg.Password,
	}

	return &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           &tls.Config{},
		SASLMechanism: mechanism,
	}
}

// NewKafkaConfig returns a Kafka writer configured for SASL/PLAIN authentication over TLS using the provided configuration.
// The writer uses least-bytes balancing, requires all acknowledgments, operates asynchronously, and batches messages with a 10-second timeout.

func NewKafkaConfig(cfg Config) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:      cfg.Brokers,
		Topic:        cfg.Topic,
		Dialer:       newDialer(cfg),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: int(kafka.RequireAll),
		Async:        true,
		BatchTimeout: 10 * time.Second,
	})
}

// NewKafkaReader creates and returns a Kafka consumer configured with SASL/PLAIN authentication over TLS, using the provided connection settings. The reader is set up for group consumption with specified fetch sizes, offset behavior, and session timeouts.

func NewKafkaReader(cfg Config) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:          cfg.Brokers,
		GroupID:          cfg.GroupID,
		Topic:            cfg.Topic,
		MinBytes:         10e3, // 10KB
		MaxBytes:         10e6, // 10MB
		StartOffset:      kafka.FirstOffset,
		CommitInterval:   time.Second,
		Dialer:           newDialer(cfg),
		SessionTimeout:   45 * time.Second, // Matches session.timeout.ms
		RebalanceTimeout: 60 * time.Second,
	})
}
