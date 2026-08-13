package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

func TestRetryQueueArguments(t *testing.T) {
	got := RetryQueueArguments("xg.jobs", 30*time.Second)
	if got["x-message-ttl"] != int32(30000) {
		t.Fatalf("ttl = %#v", got["x-message-ttl"])
	}
	if got["x-dead-letter-exchange"] != "" || got["x-dead-letter-routing-key"] != "xg.jobs" {
		t.Fatalf("dead letter arguments = %#v", got)
	}
}

func TestDrainPublisherReturns(t *testing.T) {
	returns := make(chan amqp091.Return, 2)
	returns <- amqp091.Return{MessageId: "stale-1"}
	returns <- amqp091.Return{MessageId: "stale-2"}
	drainPublisherReturns(returns)
	if len(returns) != 0 {
		t.Fatalf("publisher returns remaining = %d", len(returns))
	}
	drainPublisherReturns(nil)
}

func TestWaitForPublisherConfirmation(t *testing.T) {
	if err := waitForPublisherConfirmation(context.Background(), confirmationStub{acked: true}); err != nil {
		t.Fatalf("acked confirmation error = %v", err)
	}
	err := waitForPublisherConfirmation(context.Background(), confirmationStub{acked: false})
	if err == nil || err.Error() != "RabbitMQ negatively acknowledged the published message" {
		t.Fatalf("nacked confirmation error = %v", err)
	}
	cause := errors.New("confirmation timeout")
	err = waitForPublisherConfirmation(context.Background(), confirmationStub{err: cause})
	if !errors.Is(err, cause) {
		t.Fatalf("timeout confirmation error = %v", err)
	}
}

func TestShouldPreserveAttempt(t *testing.T) {
	if !shouldPreserveAttempt(attemptPreserverStub{}) {
		t.Fatal("attempt-preserving error was not recognized")
	}
	if shouldPreserveAttempt(errors.New("ordinary failure")) {
		t.Fatal("ordinary error preserved queue attempt")
	}
}

type attemptPreserverStub struct{}

func (attemptPreserverStub) Error() string              { return "lease busy" }
func (attemptPreserverStub) PreserveQueueAttempt() bool { return true }

type confirmationStub struct {
	acked bool
	err   error
}

func (s confirmationStub) WaitContext(context.Context) (bool, error) { return s.acked, s.err }

func TestQueueNames(t *testing.T) {
	names := Names("xg.jobs")
	if names.Main != "xg.jobs" || names.Retry != "xg.jobs.retry" || names.Dead != "xg.jobs.dead" {
		t.Fatalf("names = %#v", names)
	}
}
