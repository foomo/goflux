package goflux_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/foomo/goflux"
)

// ExampleRetryPublisher demonstrates wrapping a publisher with retry logic.
// Transient errors are retried with the given backoff until success.
func ExampleRetryPublisher() {
	// A publisher that fails twice then succeeds.
	inner := &countingPublisher{failUntil: 2}

	pub := goflux.RetryPublisher[string](inner, 3, func(_ int) time.Duration {
		return time.Millisecond // fast backoff for example
	})

	err := pub.Publish(context.Background(), "events", "hello")

	fmt.Println("error:", err)
	fmt.Println("attempts:", inner.attempts)
	// Output:
	// error: <nil>
	// attempts: 3
}

// countingPublisher fails the first failUntil attempts, then succeeds.
type countingPublisher struct {
	failUntil int
	attempts  int
}

func (p *countingPublisher) Publish(_ context.Context, _ string, _ string) error {
	p.attempts++

	if p.attempts <= p.failUntil {
		return errors.New("transient error")
	}

	return nil
}

func (p *countingPublisher) Close() error { return nil }
