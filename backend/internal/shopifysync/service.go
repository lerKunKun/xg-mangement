package shopifysync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xg-management/platform/backend/internal/integrations/shopify"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultSyncTimeout  = 15 * time.Minute
)

const productsBulkQuery = `{
  products {
    edges {
      node {
        id handle title status vendor productType updatedAt
        variants { edges { node { id sku title price compareAtPrice inventoryQuantity updatedAt } } }
      }
    }
  }
}`

const collectionsBulkQuery = `{
  collections {
    edges { node { id handle title updatedAt productsCount { count } } }
  }
}`

const themesQuery = `query Themes {
  themes(first: 100) {
    nodes { id name role processing processingFailed themeStoreId updatedAt }
  }
}`

type SyncRepository interface {
	StartSyncRun(context.Context, SyncRequest) error
	ReplaceMirror(context.Context, SyncRequest, MirrorBatch) error
	CompleteSyncRun(context.Context, SyncRequest, ResourceCounts) error
	FailSyncRun(context.Context, SyncRequest, string, string) error
}

type AccessTokenProvider interface {
	AccessToken(context.Context, string, string) (string, error)
}

type SyncTarget struct {
	Domain     string
	APIVersion string
}

type SyncTargetResolver interface {
	ResolveSyncTarget(context.Context, string, string) (SyncTarget, error)
}

type BulkClient interface {
	StartBulkQuery(context.Context, string, string, string, string) (shopify.BulkOperation, error)
	GetBulkOperation(context.Context, string, string, string, string) (shopify.BulkOperation, error)
	DownloadJSONL(context.Context, string) (io.ReadCloser, error)
	GraphQL(context.Context, string, string, string, string, any, any) error
}

type Service struct {
	Repository   SyncRepository
	Tokens       AccessTokenProvider
	Stores       SyncTargetResolver
	Shopify      BulkClient
	PollInterval time.Duration
	Timeout      time.Duration
	Clock        func() time.Time
	Sleeper      func(context.Context, time.Duration) error
}

func (s Service) Sync(ctx context.Context, request SyncRequest) (returnedErr error) {
	if err := request.Validate(); err != nil {
		return err
	}
	if s.Repository == nil || s.Tokens == nil || s.Stores == nil || s.Shopify == nil {
		return fmt.Errorf("Shopify sync service is not configured")
	}
	if request.Mode == "" {
		request.Mode = SyncModeFull
	}
	if err := s.Repository.StartSyncRun(ctx, request); err != nil {
		if errors.Is(err, ErrSyncAlreadyCompleted) {
			return nil
		}
		if errors.Is(err, ErrSyncAlreadyRunning) {
			return &Error{Code: "sync_already_running", Message: "Shopify synchronization is already running", Retryable: true}
		}
		return err
	}
	defer func() {
		if returnedErr == nil {
			return
		}
		code, message := syncFailure(returnedErr)
		_ = s.Repository.FailSyncRun(context.WithoutCancel(ctx), request, code, message)
	}()

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultSyncTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target, err := s.Stores.ResolveSyncTarget(ctx, request.OrganizationID, request.StoreID)
	if err != nil {
		return fmt.Errorf("resolve Shopify sync target: %w", err)
	}
	accessToken, err := s.Tokens.AccessToken(ctx, request.OrganizationID, request.StoreID)
	if err != nil {
		return err
	}

	products, variants, err := s.syncProducts(ctx, target, accessToken)
	if err != nil {
		return err
	}
	collections, err := s.syncCollections(ctx, target, accessToken)
	if err != nil {
		return err
	}
	themes, err := s.listThemes(ctx, target, accessToken)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Clock != nil {
		now = s.Clock().UTC()
	}
	batch := MirrorBatch{Products: products, Variants: variants, Collections: collections, Themes: themes, SyncedAt: now}
	if err := s.Repository.ReplaceMirror(ctx, request, batch); err != nil {
		return err
	}
	if err := s.Repository.CompleteSyncRun(ctx, request, batch.Counts()); err != nil {
		return err
	}
	return nil
}

func (s Service) syncProducts(ctx context.Context, target SyncTarget, token string) ([]Product, []Variant, error) {
	operation, err := s.Shopify.StartBulkQuery(ctx, target.Domain, token, target.APIVersion, productsBulkQuery)
	if err != nil {
		return nil, nil, err
	}
	operation, err = s.waitForBulk(ctx, target, token, operation)
	if err != nil {
		return nil, nil, err
	}
	body, err := s.Shopify.DownloadJSONL(ctx, operation.URL)
	if err != nil {
		return nil, nil, err
	}
	defer body.Close()
	return DecodeProductsJSONL(body)
}

func (s Service) syncCollections(ctx context.Context, target SyncTarget, token string) ([]Collection, error) {
	operation, err := s.Shopify.StartBulkQuery(ctx, target.Domain, token, target.APIVersion, collectionsBulkQuery)
	if err != nil {
		return nil, err
	}
	operation, err = s.waitForBulk(ctx, target, token, operation)
	if err != nil {
		return nil, err
	}
	body, err := s.Shopify.DownloadJSONL(ctx, operation.URL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return DecodeCollectionsJSONL(body)
}

func (s Service) waitForBulk(ctx context.Context, target SyncTarget, token string, operation shopify.BulkOperation) (shopify.BulkOperation, error) {
	interval := s.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	sleep := s.Sleeper
	if sleep == nil {
		sleep = sleepContext
	}
	for {
		if operation.Status == shopify.BulkStatusCompleted {
			if operation.URL == "" {
				return shopify.BulkOperation{}, &Error{Code: "bulk_download_missing", Message: "completed Shopify bulk operation has no download URL"}
			}
			return operation, nil
		}
		switch operation.Status {
		case shopify.BulkStatusFailed, shopify.BulkStatusCanceled, shopify.BulkStatusExpired:
			return shopify.BulkOperation{}, &Error{Code: "bulk_" + strings.ToLower(string(operation.Status)), Message: operation.ErrorCode, Retryable: operation.Status == shopify.BulkStatusFailed}
		}
		if err := sleep(ctx, interval); err != nil {
			return shopify.BulkOperation{}, err
		}
		var err error
		operation, err = s.Shopify.GetBulkOperation(ctx, target.Domain, token, target.APIVersion, operation.ID)
		if err != nil {
			return shopify.BulkOperation{}, err
		}
	}
}

type themeNode struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Role             string     `json:"role"`
	Processing       bool       `json:"processing"`
	ProcessingFailed bool       `json:"processingFailed"`
	ThemeStoreID     *int64     `json:"themeStoreId"`
	UpdatedAt        *time.Time `json:"updatedAt"`
}

type themesQueryResponse struct {
	Themes struct {
		Nodes []themeNode `json:"nodes"`
	} `json:"themes"`
}

func (s Service) listThemes(ctx context.Context, target SyncTarget, token string) ([]Theme, error) {
	var response themesQueryResponse
	if err := s.Shopify.GraphQL(ctx, target.Domain, token, target.APIVersion, themesQuery, nil, &response); err != nil {
		return nil, err
	}
	result := make([]Theme, 0, len(response.Themes.Nodes))
	for _, item := range response.Themes.Nodes {
		result = append(result, Theme{
			ShopifyGID: item.ID, Name: item.Name, Role: item.Role, Processing: item.Processing,
			ProcessingFailed: item.ProcessingFailed, ThemeStoreID: item.ThemeStoreID, UpdatedAt: item.UpdatedAt,
		})
	}
	return result, nil
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func syncFailure(err error) (string, string) {
	var syncErr *Error
	if errors.As(err, &syncErr) {
		return syncErr.Code, syncErr.Message
	}
	var providerErr *shopify.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Code, providerErr.Message
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "sync_timeout", "Shopify sync exceeded its deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "sync_canceled", "Shopify sync was canceled"
	}
	return "sync_failed", "Shopify synchronization failed"
}
