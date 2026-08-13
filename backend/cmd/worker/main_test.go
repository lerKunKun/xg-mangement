package main

import (
	"context"
	"testing"
	"time"
)

func TestWaitForReconnectStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if waitForReconnect(ctx, time.Minute) {
		t.Fatal("waitForReconnect() = true after cancellation")
	}
	if time.Since(started) > time.Second {
		t.Fatal("waitForReconnect did not stop promptly")
	}
}
