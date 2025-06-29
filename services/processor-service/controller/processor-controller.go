package controller

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	processorv1 "github.com/yaninyzwitty/threads-go-backend/gen/processor/v1"
	"github.com/yaninyzwitty/threads-go-backend/gen/processor/v1/processorv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/processor-service/repository"
)

type ProcessorController struct {
	processorv1connect.UnimplementedProcessorServiceHandler
	repo *repository.ProcessorRepository
}

func NewProcessorController(repo *repository.ProcessorRepository) *ProcessorController {
	return &ProcessorController{
		repo: repo,
	}

}

func (c *ProcessorController) ProcessOutboxMessage(ctx context.Context, req *connect.Request[processorv1.ProcessOutboxMessageRequest]) (*connect.Response[processorv1.ProcessOutboxMessageResponse], error) {
	if req.Msg.Published {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("published messages cannot be processed"))
	}

	events, err := c.repo.GetOutboxMessages(ctx, req.Msg.Published, 10)

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, event := range events {
		if err := c.repo.PublishEvent(ctx, event); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := c.repo.MarkEventAsPublished(ctx, event.EventId); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&processorv1.ProcessOutboxMessageResponse{
		ProcessedCount: int32(len(events)),
	}), nil
}
