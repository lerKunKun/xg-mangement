package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xg-management/platform/backend/internal/config"
	"github.com/xg-management/platform/backend/internal/jobs"
	"github.com/xg-management/platform/backend/internal/platform/queue"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	broker, err := queue.Connect(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		logger.Error("RabbitMQ is unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = broker.Close() }()

	deliveries, err := broker.Consume()
	if err != nil {
		logger.Error("RabbitMQ consumer could not start", "error", err)
		os.Exit(1)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("worker listening", "queue", cfg.RabbitMQQueue)

	for {
		select {
		case <-stop:
			logger.Info("worker shutdown requested")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				logger.Error("RabbitMQ delivery channel closed")
				return
			}
			var envelope jobs.Envelope
			if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
				logger.Warn("rejecting malformed job", "error", err)
				_ = delivery.Reject(false)
				continue
			}
			if err := envelope.Validate(); err != nil {
				logger.Warn("rejecting invalid job", "job_id", envelope.ID, "error", err)
				_ = delivery.Reject(false)
				continue
			}

			// Provider-specific, idempotent handlers are added behind this stable
			// envelope contract in the next implementation slice.
			logger.Info("job accepted", "job_id", envelope.ID, "job_type", envelope.Type, "organization_id", envelope.OrganizationID)
			_ = delivery.Ack(false)
		}
	}
}
