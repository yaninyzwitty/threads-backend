package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/threads-go-backend/gen/processor/v1/processorv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/processor-service/controller"
	"github.com/yaninyzwitty/threads-go-backend/services/processor-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/shared/database"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
	"github.com/yaninyzwitty/threads-go-backend/shared/queue"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
)

func main() {

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	// Load config
	cfg := pkg.Config{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)

	}

	if err := snowflake.InitSonyFlake(); err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}

	db := database.NewAstraDB()

	astraCfg := database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("ASTRA_DB_TOKEN", ""),
	}

	session, err := db.Connect(ctx, &astraCfg, 10*time.Second)

	if err != nil {
		slog.Error("failed to connect to astra db", "error", err)
		os.Exit(1)
	}

	defer session.Close()

	// create kafka instance

	kafkaConfig := queue.Config{
		Brokers:  cfg.Queue.Brokers,
		Topic:    cfg.Queue.Topic,
		GroupID:  cfg.Queue.GroupID,
		Username: cfg.Queue.Username,
		Password: helpers.GetEnvOrDefault("KAFKA_PASSWORD", ""),
	}

	// create kafka producer
	kafkaWriter := queue.NewKafkaConfig(kafkaConfig)

	producer := queue.NewProducer(kafkaWriter)

	if producer == nil {
		slog.Error("failed to create kafka producer")
		os.Exit(1)
	}

	defer producer.Close()

	// start background worker pool
	workerContext, workerCancel := context.WithCancel(context.Background())

	defer workerCancel()

	// create the user service address
	processorServiceAddr := fmt.Sprintf(":%d", cfg.ProcessorServer.Port)

	// create repository
	processorServiceRepo := repository.NewProcessorRepository(session, producer)

	// create controller
	processorServiceController := controller.NewProcessorController(processorServiceRepo)

	//build http server from service implementation
	processorPath, processorHandler := processorv1connect.NewProcessorServiceHandler(
		processorServiceController,
		connect.WithInterceptors(auth.AuthInterceptor()))
	mux := http.NewServeMux()
	mux.Handle(processorPath, processorHandler)

	server := &http.Server{
		Addr:    processorServiceAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		sig := <-quit
		slog.Info("received shutdown signal", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
		} else {
			slog.Info("server shutdown gracefully")
		}
	}()

	go func() {

		defer wg.Done()
		StartWorkerPool(workerContext, session, producer, 3, processorServiceController)

	}()

	// Start ConnectRPC server
	slog.Info("starting ConnectRPC server", "address", processorServiceAddr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
	}
}
