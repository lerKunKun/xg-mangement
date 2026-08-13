package postgres

import (
	"context"
	"fmt"
)

func (c *Client) IsJobProcessed(ctx context.Context, jobID string) (bool, error) {
	var processed bool
	if err := c.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM processed_jobs WHERE job_id=$1)`, jobID).Scan(&processed); err != nil {
		return false, fmt.Errorf("check processed job: %w", err)
	}
	return processed, nil
}

func (c *Client) MarkJobProcessed(ctx context.Context, jobID, organizationID, jobType string) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO processed_jobs(job_id,organization_id,job_type) VALUES($1,$2,$3)
		ON CONFLICT(job_id) DO NOTHING`, jobID, organizationID, jobType)
	if err != nil {
		return fmt.Errorf("mark job processed: %w", err)
	}
	return nil
}
