package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/latoulicious/RoutePilot/internal/adapters/db"
	"github.com/latoulicious/RoutePilot/internal/adapters/kafka"
	"github.com/latoulicious/RoutePilot/internal/job"
)

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting FaaS Outbox Worker")

	// Load configuration
	config, err := job.LoadConfigFromEnv()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create database connection
	dbPool, err := db.Connect(config.Database.URL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Create repositories
	repo := db.NewRepository(dbPool)
	outboxRepo := db.NewOutboxRepositoryAdapter(repo)

	// Create Kafka publisher
	publisher, err := kafka.NewSaramaPublisher(config.Kafka, logger)
	if err != nil {
		logger.Error("Failed to create Kafka publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	// Create outbox worker
	worker := job.NewOutboxWorker(outboxRepo, publisher, config.Worker, logger)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start worker
	if err := worker.Start(ctx); err != nil {
		logger.Error("Failed to start outbox worker", "error", err)
		os.Exit(1)
	}

	logger.Info("Outbox worker started successfully")

	// Wait for shutdown signal
	<-sigCh
	logger.Info("Shutdown signal received")

	// Cancel context to stop worker
	cancel()

	// Stop worker gracefully
	if err := worker.Stop(); err != nil {
		logger.Error("Error during worker shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("FaaS Outbox Worker stopped successfully")
}
