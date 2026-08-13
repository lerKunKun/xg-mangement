package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xg-management/platform/backend/internal/config"
	"github.com/xg-management/platform/backend/internal/integrations/shopify"
	"github.com/xg-management/platform/backend/internal/jobs"
	"github.com/xg-management/platform/backend/internal/platform/postgres"
	"github.com/xg-management/platform/backend/internal/platform/queue"
	"github.com/xg-management/platform/backend/internal/security"
	"github.com/xg-management/platform/backend/internal/shopifysync"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	startupContext, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer startupCancel()
	database, err := postgres.Connect(startupContext, cfg.DatabaseURL)
	if err != nil {
		logger.Error("PostgreSQL is unavailable", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	cipher, err := security.NewCredentialCipher(cfg.CredentialEncryptionKey)
	if err != nil {
		logger.Error("credential encryption configuration is invalid", "error", err)
		os.Exit(1)
	}
	shopifyClient := shopify.NewClient()
	tokenManager := shopifysync.TokenManager{Repository: database, Cipher: cipher, Refresher: shopifyClient}
	syncService := shopifysync.Service{
		Repository: database, Tokens: tokenManager, Stores: database, Shopify: shopifyClient,
		PollInterval: cfg.ShopifySync.PollInterval, Timeout: cfg.ShopifySync.Timeout,
	}
	handler := shopifysync.Handler{Syncer: syncService}

	broker, err := queue.ConnectWithOptions(cfg.RabbitMQURL, cfg.RabbitMQQueue, cfg.RabbitMQRetryDelay, cfg.ShopifySync.MaxAttempts)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info("worker listening", "queue", cfg.RabbitMQQueue, "max_attempts", cfg.ShopifySync.MaxAttempts)
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutdown requested")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				logger.Error("RabbitMQ delivery channel closed")
				return
			}
			processDelivery(ctx, logger, database, handler, delivery)
		}
	}
}

type jobStore interface {
	IsJobProcessed(context.Context, string) (bool, error)
	MarkJobProcessed(context.Context, string, string, string) error
}

func processDelivery(ctx context.Context, logger *slog.Logger, store jobStore, handler shopifysync.Handler, delivery queue.Delivery) {
	envelope, err := delivery.Envelope()
	if err != nil {
		logger.Warn("dead-lettering invalid job", "error", err)
		_ = delivery.DeadLetter(ctx, err)
		return
	}
	processed, err := store.IsJobProcessed(ctx, envelope.ID)
	if err != nil {
		logger.Error("processed job lookup failed", "job_id", envelope.ID, "error", err)
		_ = delivery.Retry(ctx, err)
		return
	}
	if processed {
		_ = delivery.Ack()
		return
	}
	err = safeHandle(ctx, handler, envelope)
	if err != nil {
		logger.Warn("job handling failed", "job_id", envelope.ID, "job_type", envelope.Type, "retryable", shopifysync.IsRetryable(err), "error", err)
		if shopifysync.IsRetryable(err) {
			_ = delivery.Retry(ctx, err)
		} else {
			_ = delivery.DeadLetter(ctx, err)
		}
		return
	}
	if err := store.MarkJobProcessed(ctx, envelope.ID, envelope.OrganizationID, envelope.Type); err != nil {
		logger.Error("recording processed job failed", "job_id", envelope.ID, "error", err)
		_ = delivery.Retry(ctx, err)
		return
	}
	_ = delivery.Ack()
	logger.Info("job completed", "job_id", envelope.ID, "job_type", envelope.Type, "organization_id", envelope.OrganizationID)
}

func safeHandle(ctx context.Context, handler shopifysync.Handler, envelope jobs.Envelope) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &shopifysync.HandlerError{Code: "handler_panic", Retryable: true, Cause: fmt.Errorf("recovered worker panic: %v", recovered)}
		}
	}()
	return handler.Handle(ctx, envelope)
}
