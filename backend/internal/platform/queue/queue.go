package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
	"github.com/xg-management/platform/backend/internal/jobs"
)

type Client struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	queue      string
}

func Connect(url, queueName string) (*Client, error) {
	connection, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare RabbitMQ queue: %w", err)
	}
	return &Client{connection: connection, channel: channel, queue: queueName}, nil
}

func (c *Client) Publish(ctx context.Context, envelope jobs.Envelope) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode job envelope: %w", err)
	}
	return c.channel.PublishWithContext(ctx, "", c.queue, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    envelope.ID,
		Type:         envelope.Type,
		Body:         body,
	})
}

func (c *Client) Consume() (<-chan amqp091.Delivery, error) {
	return c.channel.Consume(c.queue, "", false, false, false, false, nil)
}

func (c *Client) Close() error {
	channelErr := c.channel.Close()
	connectionErr := c.connection.Close()
	if channelErr != nil {
		return channelErr
	}
	return connectionErr
}
