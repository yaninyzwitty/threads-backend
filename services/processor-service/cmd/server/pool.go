package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/threads-go-backend/services/processor-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
)

func StartWorkerPool(ctx context.Context, session *gocql.Session, writer *queue.Producer, numWorkers int, processorServiceController *controller.ProcessorController) {
	wg := sync.WaitGroup{}

	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			slog.Info("Worker started", "id", workerID)

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					slog.Info("Worker stopped", "id", workerID)
					return
				case <-ticker.C:
					if err := ProcessEvents(ctx, session, writer, processorServiceController); err != nil {
						slog.Error("Worker failed", "id", workerID, "error", err)
					}
				}
			}

		}(i)

		wg.Wait()
	}

}
