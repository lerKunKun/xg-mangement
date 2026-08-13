package redis

import (
	"context"
	"fmt"

	redisclient "github.com/redis/go-redis/v9"
)

type Client struct {
	client *redisclient.Client
}

func Connect(ctx context.Context, redisURL string) (*Client, error) {
	options, err := redisclient.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}
	client := redisclient.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.client.Close()
}
