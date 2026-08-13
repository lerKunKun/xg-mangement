package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/xg-management/platform/backend/internal/jobs"
)

const attemptHeader = "x-xg-attempt"

type Client struct {
	connection  *amqp091.Connection
	channel     *amqp091.Channel
	queues      QueueNames
	maxAttempts int
}

func Connect(url, queueName string) (*Client, error) {
	return ConnectWithOptions(url, queueName, 30*time.Second, 5)
}

func ConnectWithOptions(url, queueName string, retryDelay time.Duration, maxAttempts int) (*Client, error) {
	if retryDelay <= 0 || maxAttempts <= 0 {
		return nil, fmt.Errorf("RabbitMQ retry delay and max attempts must be positive")
	}
	connection, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	names := Names(queueName)
	declarations := []struct {
		name string
		args amqp091.Table
	}{
		{names.Main, nil},
		{names.Retry, RetryQueueArguments(names.Main, retryDelay)},
		{names.Dead, nil},
	}
	for _, declaration := range declarations {
		if _, err := channel.QueueDeclare(declaration.name, true, false, false, false, declaration.args); err != nil {
			_ = channel.Close()
			_ = connection.Close()
			return nil, fmt.Errorf("declare RabbitMQ queue %s: %w", declaration.name, err)
		}
	}
	if err := channel.Qos(1, 0, false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("set RabbitMQ consumer qos: %w", err)
	}
	return &Client{connection: connection, channel: channel, queues: names, maxAttempts: maxAttempts}, nil
}

func (c *Client) Publish(ctx context.Context, envelope jobs.Envelope) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode job envelope: %w", err)
	}
	return c.publish(ctx, c.queues.Main, amqp091.Publishing{
		ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: envelope.ID,
		Type: envelope.Type, Headers: amqp091.Table{attemptHeader: int32(1)}, Body: body,
	})
}

func (c *Client) Consume() (<-chan Delivery, error) {
	raw, err := c.channel.Consume(c.queues.Main, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	result := make(chan Delivery)
	go func() {
		defer close(result)
		for item := range raw {
			result <- Delivery{raw: item, client: c}
		}
	}()
	return result, nil
}

func (c *Client) Close() error {
	channelErr := c.channel.Close()
	connectionErr := c.connection.Close()
	if channelErr != nil {
		return channelErr
	}
	return connectionErr
}

func (c *Client) publish(ctx context.Context, routingKey string, message amqp091.Publishing) error {
	return c.channel.PublishWithContext(ctx, "", routingKey, false, false, message)
}

type Delivery struct {
	raw    amqp091.Delivery
	client *Client
}

func (d Delivery) Envelope() (jobs.Envelope, error) {
	var envelope jobs.Envelope
	if err := json.Unmarshal(d.raw.Body, &envelope); err != nil {
		return jobs.Envelope{}, fmt.Errorf("decode job envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return jobs.Envelope{}, err
	}
	return envelope, nil
}

func (d Delivery) Ack() error { return d.raw.Ack(false) }

func (d Delivery) Retry(ctx context.Context, cause error) error {
	attempt := headerAttempt(d.raw.Headers)
	if attempt >= d.client.maxAttempts {
		return d.DeadLetter(ctx, cause)
	}
	message := d.copyMessage()
	message.Headers[attemptHeader] = int32(attempt + 1)
	message.Headers["x-xg-last-error"] = safeCause(cause)
	if err := d.client.publish(ctx, d.client.queues.Retry, message); err != nil {
		_ = d.raw.Nack(false, true)
		return fmt.Errorf("publish retry job: %w", err)
	}
	return d.raw.Ack(false)
}

func (d Delivery) DeadLetter(ctx context.Context, cause error) error {
	message := d.copyMessage()
	message.Headers[attemptHeader] = int32(headerAttempt(d.raw.Headers))
	message.Headers["x-xg-last-error"] = safeCause(cause)
	if err := d.client.publish(ctx, d.client.queues.Dead, message); err != nil {
		_ = d.raw.Nack(false, true)
		return fmt.Errorf("publish dead-letter job: %w", err)
	}
	return d.raw.Ack(false)
}

func (d Delivery) copyMessage() amqp091.Publishing {
	headers := amqp091.Table{}
	for key, value := range d.raw.Headers {
		headers[key] = value
	}
	return amqp091.Publishing{
		Headers: headers, ContentType: d.raw.ContentType, DeliveryMode: amqp091.Persistent,
		MessageId: d.raw.MessageId, Type: d.raw.Type, Timestamp: d.raw.Timestamp, Body: append([]byte(nil), d.raw.Body...),
	}
}

func headerAttempt(headers amqp091.Table) int {
	if headers == nil {
		return 1
	}
	switch value := headers[attemptHeader].(type) {
	case int32:
		return max(1, int(value))
	case int64:
		return max(1, int(value))
	case int:
		return max(1, value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return max(1, parsed)
	default:
		return 1
	}
}

func safeCause(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}
