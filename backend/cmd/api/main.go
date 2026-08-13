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
	"github.com/xg-management/platform/backend/internal/platform/objectstore"
	"github.com/xg-management/platform/backend/internal/platform/postgres"
	rediscache "github.com/xg-management/platform/backend/internal/platform/redis"
	"github.com/xg-management/platform/backend/internal/rbac"
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

	cache, err := rediscache.Connect(startupContext, cfg.RedisURL)
	if err != nil {
		logger.Error("Redis is unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = cache.Close() }()

	storage, err := objectstore.New(startupContext, cfg.ObjectStorage)
	if err != nil {
		logger.Error("object storage configuration is invalid", "error", err)
		os.Exit(1)
	}
	if err := storage.Ping(startupContext); err != nil {
		logger.Error("object storage is unavailable", "error", err)
		os.Exit(1)
	}

	router := httpapi.NewRouter(httpapi.Dependencies{
		Authenticator: auth.DevAuthenticator{Enabled: cfg.Auth.DevLoginEnabled},
		Authorizer:    rbac.Authorizer{},
		Stores:        database,
		Assets:        postgres.NewAssetRepository(database),
		Approvals:     postgres.NewApprovalRepository(database),
		Integrations:  integrations.Catalog(cfg),
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
