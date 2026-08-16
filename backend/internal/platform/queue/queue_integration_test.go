package queue

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/jobs"
)

func TestRabbitMQPublisherConfirmAndConsume(t *testing.T) {
	url := os.Getenv("RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("RABBITMQ_TEST_URL is not set")
	}
	client, err := ConnectWithOptions(url, "xg.jobs.integration-test", time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = client.publishChannel.QueueDelete(client.queues.Retry, false, false, false)
		_, _ = client.publishChannel.QueueDelete(client.queues.Dead, false, false, false)
		_, _ = client.publishChannel.QueueDelete(client.queues.Main, false, false, false)
		_ = client.Close()
	}()
	deliveries, err := client.Consume()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"store_id": "store-1", "run_id": "run-1", "mode": "full"})
	envelope := jobs.Envelope{
		Version: 1, ID: "queue-integration-confirm", Type: jobs.TypeShopifyStoreSyncRequested,
		OrganizationID: "org-1", OccurredAt: time.Now().UTC(), Payload: payload,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Publish(ctx, envelope); err != nil {
		t.Fatalf("confirmed publish failed: %v", err)
	}
	select {
	case delivery := <-deliveries:
		got, err := delivery.Envelope()
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != envelope.ID {
			t.Fatalf("message ID = %q, want %q", got.ID, envelope.ID)
		}
		if err := delivery.Ack(); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for confirmed RabbitMQ message")
	}
}

func TestRabbitMQPublisherChannelClosureIsObservable(t *testing.T) {
	url := os.Getenv("RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("RABBITMQ_TEST_URL is not set")
	}
	client, err := ConnectWithOptions(url, "xg.jobs.publisher-close-test", time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.connection.Close() }()
	closed := client.PublisherClosed()
	if err := client.publishChannel.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher channel closure was not observable")
	}
}
