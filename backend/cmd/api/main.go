package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/config"
	"github.com/xg-management/platform/backend/internal/httpapi"
	"github.com/xg-management/platform/backend/internal/integrations"
	"github.com/xg-management/platform/backend/internal/integrations/dingtalk"
	"github.com/xg-management/platform/backend/internal/integrations/shopify"
	"github.com/xg-management/platform/backend/internal/platform/objectstore"
	"github.com/xg-management/platform/backend/internal/platform/postgres"
	"github.com/xg-management/platform/backend/internal/platform/queue"
	rediscache "github.com/xg-management/platform/backend/internal/platform/redis"
	"github.com/xg-management/platform/backend/internal/rbac"
	"github.com/xg-management/platform/backend/internal/security"
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
	authorizer, err := rbac.NewAuthorizer(startupContext, database)
	if err != nil {
		logger.Error("Casbin policy is unavailable", "error", err)
		os.Exit(1)
	}

	cache, err := rediscache.Connect(startupContext, cfg.RedisURL)
	if err != nil {
		logger.Error("Redis is unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = cache.Close() }()
	sessions := auth.NewSessionManager(cache, 12*time.Hour)
	states := auth.NewOAuthStateManager(cache, 10*time.Minute)
	cipher, err := security.NewCredentialCipher(cfg.CredentialEncryptionKey)
	if err != nil {
		logger.Error("credential encryption configuration is invalid", "error", err)
		os.Exit(1)
	}

	storage, err := objectstore.New(startupContext, cfg.ObjectStorage)
	if err != nil {
		logger.Error("object storage configuration is invalid", "error", err)
		os.Exit(1)
	}
	if err := storage.Ping(startupContext); err != nil {
		logger.Error("object storage is unavailable", "error", err)
		os.Exit(1)
	}
	jobQueue, err := queue.Connect(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		logger.Error("RabbitMQ is unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = jobQueue.Close() }()

	secureCookies := cfg.Environment != "development"
	integrationFlow := &httpapi.IntegrationDependencies{
		Repository:     database,
		Cipher:         cipher,
		States:         states,
		Sessions:       sessions,
		DingTalk:       dingtalk.NewClient(),
		Shopify:        shopify.NewClient(),
		Jobs:           jobQueue,
		WebBaseURL:     cfg.WebBaseURL,
		SecureCookies:  secureCookies,
		SessionTTL:     12 * time.Hour,
		PolicyReloader: authorizer,
	}
	router := httpapi.NewRouter(httpapi.Dependencies{
		Authenticator: auth.CompositeAuthenticator{
			auth.SessionAuthenticator{Sessions: sessions, Principals: database},
			auth.DevAuthenticator{Enabled: cfg.Auth.DevLoginEnabled},
		},
		Authorizer:      authorizer,
		PolicyReloader:  authorizer,
		Stores:          database,
		Assets:          postgres.NewAssetRepository(database),
		Approvals:       postgres.NewApprovalRepository(database),
		Integrations:    integrations.Catalog(cfg),
		Admin:           database,
		Sessions:        sessions,
		IntegrationFlow: integrationFlow,
		DevLoginEnabled: cfg.Auth.DevLoginEnabled,
		SecureCookies:   secureCookies,
		SessionTTL:      12 * time.Hour,
	})
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API listening", "address", cfg.HTTPAddress, "environment", cfg.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case signalValue := <-stop:
		logger.Info("shutdown requested", "signal", signalValue.String())
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("API stopped unexpectedly", "error", err)
		}
	}

	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
