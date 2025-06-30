package main

import (
	"context"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/gocql/gocql"
	processorv1 "github.com/yaninyzwitty/threads-go-backend/gen/processor/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/processor-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
)

func ProcessEvents(ctx context.Context, session *gocql.Session, writer *queue.Producer, processorServiceController *controller.ProcessorController) error {

	req := connect.NewRequest(&processorv1.ProcessOutboxMessageRequest{
		Published: false,
	})

	resp, err := processorServiceController.ProcessOutboxMessage(ctx, req)
	if err != nil {
		return err
	}

	if resp.Msg.ProcessedCount > 0 {
		slog.Info("Processed events", "count", resp.Msg.ProcessedCount)
	}

	return nil
}
