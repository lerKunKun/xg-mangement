package shopifysync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xg-management/platform/backend/internal/integrations/shopify"
	"github.com/xg-management/platform/backend/internal/jobs"
)

type SyncProcessor interface {
	Sync(context.Context, SyncRequest) error
}

type Handler struct{ Syncer SyncProcessor }

type HandlerError struct {
	Code      string
	Retryable bool
	Cause     error
}

func (e *HandlerError) Error() string {
	if e.Cause == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *HandlerError) Unwrap() error { return e.Cause }

func (h Handler) Handle(ctx context.Context, envelope jobs.Envelope) error {
	if envelope.Type != jobs.TypeShopifyStoreSyncRequested {
		return &HandlerError{Code: "unsupported_job_type", Cause: fmt.Errorf("unsupported job type %q", envelope.Type)}
	}
	var payload struct {
		StoreID string   `json:"store_id"`
		RunID   string   `json:"run_id"`
		Mode    SyncMode `json:"mode"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return &HandlerError{Code: "invalid_job_payload", Cause: err}
	}
	request := SyncRequest{OrganizationID: envelope.OrganizationID, StoreID: payload.StoreID, RunID: payload.RunID, Mode: payload.Mode}
	if err := request.Validate(); err != nil {
		return &HandlerError{Code: "invalid_job_payload", Cause: err}
	}
	if h.Syncer == nil {
		return &HandlerError{Code: "handler_not_configured", Cause: fmt.Errorf("Shopify sync handler is not configured")}
	}
	if err := h.Syncer.Sync(ctx, request); err != nil {
		return classifyHandlerError(err)
	}
	return nil
}

func classifyHandlerError(err error) error {
	var providerErr *shopify.ProviderError
	if errors.As(err, &providerErr) {
		return &HandlerError{Code: providerErr.Code, Retryable: providerErr.Retryable, Cause: err}
	}
	var syncErr *Error
	if errors.As(err, &syncErr) {
		return &HandlerError{Code: syncErr.Code, Retryable: syncErr.Retryable, Cause: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &HandlerError{Code: "sync_timeout", Retryable: true, Cause: err}
	}
	return &HandlerError{Code: "sync_failed", Cause: err}
}

func IsRetryable(err error) bool {
	var handlerErr *HandlerError
	return errors.As(err, &handlerErr) && handlerErr.Retryable
}
