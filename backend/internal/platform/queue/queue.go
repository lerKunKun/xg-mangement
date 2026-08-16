package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/xg-management/platform/backend/internal/jobs"
)

const attemptHeader = "x-xg-attempt"
const preserveAttemptHeader = "x-xg-preserve-attempt"

type Client struct {
	connection      *amqp091.Connection
	consumeChannel  *amqp091.Channel
	publishChannel  *amqp091.Channel
	returns         <-chan amqp091.Return
	publisherClosed <-chan *amqp091.Error
	queues          QueueNames
	maxAttempts     int
	publishMu       sync.Mutex
}

type LazyPublisher struct {
	url         string
	queueName   string
	retryDelay  time.Duration
	maxAttempts int
	mu          sync.Mutex
	client      *Client
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
	publishChannel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ publisher channel: %w", err)
	}
	names := Names(queueName)
	if err := declareQueues(publishChannel, names, retryDelay); err != nil {
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, err
	}
	if err := publishChannel.Confirm(false); err != nil {
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	returns := publishChannel.NotifyReturn(make(chan amqp091.Return, 1))
	publisherClosed := publishChannel.NotifyClose(make(chan *amqp091.Error, 1))
	consumeChannel, err := connection.Channel()
	if err != nil {
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ consumer channel: %w", err)
	}
	if err := consumeChannel.Qos(1, 0, false); err != nil {
		_ = consumeChannel.Close()
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("set RabbitMQ consumer qos: %w", err)
	}
	return &Client{connection: connection, consumeChannel: consumeChannel, publishChannel: publishChannel, returns: returns, publisherClosed: publisherClosed, queues: names, maxAttempts: maxAttempts}, nil
}

func connectPublisher(url, queueName string, retryDelay time.Duration, maxAttempts int) (*Client, error) {
	connection, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	publishChannel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ publisher channel: %w", err)
	}
	names := Names(queueName)
	if err := declareQueues(publishChannel, names, retryDelay); err != nil {
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, err
	}
	if err := publishChannel.Confirm(false); err != nil {
		_ = publishChannel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	returns := publishChannel.NotifyReturn(make(chan amqp091.Return, 1))
	publisherClosed := publishChannel.NotifyClose(make(chan *amqp091.Error, 1))
	return &Client{connection: connection, publishChannel: publishChannel, returns: returns, publisherClosed: publisherClosed, queues: names, maxAttempts: maxAttempts}, nil
}

func declareQueues(channel *amqp091.Channel, names QueueNames, retryDelay time.Duration) error {
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
			return fmt.Errorf("declare RabbitMQ queue %s: %w", declaration.name, err)
		}
	}
	return nil
}

func NewLazyPublisher(url, queueName string, retryDelay time.Duration, maxAttempts int) *LazyPublisher {
	return &LazyPublisher{url: url, queueName: queueName, retryDelay: retryDelay, maxAttempts: maxAttempts}
}

func (p *LazyPublisher) Publish(ctx context.Context, envelope jobs.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		client, err := connectPublisher(p.url, p.queueName, p.retryDelay, p.maxAttempts)
		if err != nil {
			return err
		}
		p.client = client
	}
	if err := p.client.Publish(ctx, envelope); err != nil {
		_ = p.client.Close()
		p.client = nil
		return err
	}
	return nil

}

func (p *LazyPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil
	}
	err := p.client.Close()
	p.client = nil
	return err
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
	if c.consumeChannel == nil {
		return nil, fmt.Errorf("RabbitMQ consumer channel is unavailable")
	}
	raw, err := c.consumeChannel.Consume(c.queues.Main, "", false, false, false, false, nil)
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

func (c *Client) PublisherClosed() <-chan *amqp091.Error { return c.publisherClosed }

func (c *Client) Close() error {
	var channelErr error
	if c.consumeChannel != nil {
		channelErr = c.consumeChannel.Close()
	}
	if c.publishChannel != nil {
		if err := c.publishChannel.Close(); channelErr == nil {
			channelErr = err
		}
	}
	connectionErr := c.connection.Close()
	if channelErr != nil {
		return channelErr
	}
	return connectionErr
}

func (c *Client) publish(ctx context.Context, routingKey string, message amqp091.Publishing) error {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	if c.publishChannel == nil {
		return fmt.Errorf("RabbitMQ publisher channel is unavailable")
	}
	drainPublisherReturns(c.returns)
	confirmation, err := c.publishChannel.PublishWithDeferredConfirmWithContext(ctx, "", routingKey, true, false, message)
	if err != nil {
		return err
	}
	if confirmation == nil {
		return fmt.Errorf("RabbitMQ publisher confirmation is unavailable")
	}
	if err := waitForPublisherConfirmation(ctx, confirmation); err != nil {
		return err
	}
	select {
	case returned, ok := <-c.returns:
		if ok {
			return fmt.Errorf("RabbitMQ returned unroutable message %s: %s", returned.MessageId, returned.ReplyText)
		}
	default:
	}
	return nil
}

func drainPublisherReturns(returns <-chan amqp091.Return) {
	if returns == nil {
		return
	}
	for {
		select {
		case <-returns:
		default:
			return
		}
	}
}

type publisherConfirmation interface {
	WaitContext(context.Context) (bool, error)
}

func waitForPublisherConfirmation(ctx context.Context, confirmation publisherConfirmation) error {
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for RabbitMQ publisher confirmation: %w", err)
	}
	if !acked {
		return fmt.Errorf("RabbitMQ negatively acknowledged the published message")
	}
	return nil
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
	preserveAttempt := shouldPreserveAttempt(cause)
	if attempt >= d.client.maxAttempts && !preserveAttempt {
		return d.DeadLetter(ctx, cause)
	}
	message := d.copyMessage()
	if preserveAttempt {
		message.Headers[attemptHeader] = int32(attempt)
		message.Headers[preserveAttemptHeader] = true
	} else {
		message.Headers[attemptHeader] = int32(attempt + 1)
		delete(message.Headers, preserveAttemptHeader)
	}
	message.Headers["x-xg-last-error"] = safeCause(cause)
	if err := d.client.publish(ctx, d.client.queues.Retry, message); err != nil {
		_ = d.raw.Nack(false, true)
		return fmt.Errorf("publish retry job: %w", err)
	}
	return d.raw.Ack(false)
}

type attemptPreserver interface {
	PreserveQueueAttempt() bool
}

func shouldPreserveAttempt(err error) bool {
	var preserver attemptPreserver
	return errors.As(err, &preserver) && preserver.PreserveQueueAttempt()
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
