package queue

import (
	"testing"
	"time"
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

func TestQueueNames(t *testing.T) {
	names := Names("xg.jobs")
	if names.Main != "xg.jobs" || names.Retry != "xg.jobs.retry" || names.Dead != "xg.jobs.dead" {
		t.Fatalf("names = %#v", names)
	}
}
