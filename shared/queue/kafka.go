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

// Create SASL Dialer with TLS - compatible with astra-streaming
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

// KafkaWriter initializes a Kafka writer (producer)

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

// KafkaReader initializes a Kafka reader (consumer)

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
