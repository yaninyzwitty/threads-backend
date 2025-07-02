package repository

import (
	"context"
	"fmt"

	"github.com/gocql/gocql"
	processorv1 "github.com/yaninyzwitty/threads-go-backend/gen/processor/v1"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
	"google.golang.org/protobuf/encoding/protojson"
)

type ProcessorRepository struct {
	session  *gocql.Session
	producer *queue.Producer
}

func NewProcessorRepository(session *gocql.Session, producer *queue.Producer) *ProcessorRepository {
	return &ProcessorRepository{
		session:  session,
		producer: producer,
	}
}

func (r *ProcessorRepository) GetOutboxMessages(ctx context.Context, published bool, limit int) ([]*processorv1.OutboxMessage, error) {
	query := `
		SELECT event_id, event_type, payload, published
		FROM threads_keyspace.outbox
		WHERE published = ?
		LIMIT ?;
	`

	iter := r.session.Query(query, published, limit).WithContext(ctx).Iter()

	var outboxMessages []*processorv1.OutboxMessage

	var eventId, eventType string
	var payload []byte
	var isPublished bool

	for iter.Scan(&eventId, &eventType, &payload, &isPublished) {
		outboxMessages = append(outboxMessages, &processorv1.OutboxMessage{
			EventId:   eventId,
			EventType: eventType,
			Payload:   payload,
			Published: isPublished,
		})
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return outboxMessages, nil
}

func (r *ProcessorRepository) PublishEvent(ctx context.Context, event *processorv1.OutboxMessage) error {
	payload, err := protojson.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return r.producer.Publish(ctx, event.EventId, payload)
}

func (r *ProcessorRepository) MarkEventAsPublished(ctx context.Context, eventId string) error {
	query := `
		UPDATE threads_keyspace.outbox
		SET published = true
		WHERE event_id = ?
	`

	return r.session.Query(query, eventId).WithContext(ctx).Exec()
}
