package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xg-management/platform/backend/internal/jobs"
)

type OutboxJobPublisher interface {
	Publish(context.Context, jobs.Envelope) error
}

type claimedOutboxMessage struct {
	ID       string
	Envelope jobs.Envelope
	Attempts int
}

func (c *Client) PublishOutboxBatch(ctx context.Context, publisher OutboxJobPublisher, limit int) (int, error) {
	if publisher == nil {
		return 0, fmt.Errorf("outbox publisher is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	messages, err := c.claimOutbox(ctx, limit)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, message := range messages {
		if err := publisher.Publish(ctx, message.Envelope); err != nil {
			if retryErr := c.retryOutbox(ctx, message.ID, message.Attempts+1, err); retryErr != nil {
				return published, retryErr
			}
			continue
		}
		if err := c.completeOutbox(ctx, message.ID); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}

func (c *Client) claimOutbox(ctx context.Context, limit int) ([]claimedOutboxMessage, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id FROM outbox_messages
			WHERE ((status IN ('pending','failed') AND available_at <= now())
			   OR (status='publishing' AND updated_at < now() - interval '5 minutes'))
			ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
		)
		UPDATE outbox_messages o SET status='publishing',updated_at=now()
		FROM candidates c WHERE o.id=c.id
		RETURNING o.id::text,o.envelope,o.attempts`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim outbox messages: %w", err)
	}
	defer rows.Close()
	result := make([]claimedOutboxMessage, 0)
	for rows.Next() {
		var item claimedOutboxMessage
		var payload []byte
		if err := rows.Scan(&item.ID, &payload, &item.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}
		item.Envelope, err = decodeOutboxEnvelope(payload)
		if err != nil {
			return nil, fmt.Errorf("decode outbox message %s: %w", item.ID, err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return result, nil
}

func (c *Client) completeOutbox(ctx context.Context, id string) error {
	_, err := c.pool.Exec(ctx, `UPDATE outbox_messages SET status='published',published_at=now(),last_error=NULL,updated_at=now() WHERE id=$1 AND status='publishing'`, id)
	if err != nil {
		return fmt.Errorf("complete outbox message: %w", err)
	}
	return nil
}

func (c *Client) retryOutbox(ctx context.Context, id string, attempts int, cause error) error {
	message := "queue publish failed"
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 512 {
		message = message[:512]
	}
	_, err := c.pool.Exec(ctx, `
		UPDATE outbox_messages SET status='failed',attempts=$2,available_at=now()+($3::bigint*interval '1 millisecond'),
		last_error=$4,updated_at=now() WHERE id=$1 AND status='publishing'`, id, attempts, outboxBackoff(attempts).Milliseconds(), message)
	if err != nil {
		return fmt.Errorf("retry outbox message: %w", err)
	}
	return nil
}

func outboxBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << min(attempt-1, 9)
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func decodeOutboxEnvelope(payload []byte) (jobs.Envelope, error) {
	var envelope jobs.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return jobs.Envelope{}, err
	}
	if err := envelope.Validate(); err != nil {
		return jobs.Envelope{}, err
	}
	return envelope, nil
}
