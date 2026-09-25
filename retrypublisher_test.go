package goflux_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
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

// ExampleWithRetryable demonstrates restricting retries to a subset of errors.
// Errors the predicate rejects are returned immediately, without backoff.
func ExampleWithRetryable() {
	// A publisher that always fails with a permanent error.
	inner := &countingPublisher{failUntil: -1, err: goflux.NonRetryable(errPermanent)}

	pub := goflux.RetryPublisher[string](inner, 3, func(_ int) time.Duration {
		return time.Millisecond
	}, goflux.WithRetryable(func(err error) bool {
		return !goflux.IsNonRetryable(err)
	}))

	err := pub.Publish(context.Background(), "events", "hello")

	fmt.Println("error:", err)
	fmt.Println("attempts:", inner.attempts)
	// Output:
	// error: non-retryable: permanent error
	// attempts: 1
}

var (
	errTransient = errors.New("transient error")
	errPermanent = errors.New("permanent error")
)

// noBackoff keeps retry tests fast — the retry loop still waits on the timer.
func noBackoff(_ int) time.Duration { return 0 }

// countingPublisher fails the first failUntil attempts, then succeeds.
// A negative failUntil fails every attempt.
type countingPublisher struct {
	failUntil int
	err       error
	attempts  int
}

func (p *countingPublisher) Publish(_ context.Context, _ string, _ string) error {
	p.attempts++

	if p.failUntil < 0 || p.attempts <= p.failUntil {
		if p.err != nil {
			return p.err
		}

		return errTransient
	}

	return nil
}

func (p *countingPublisher) Close() error { return nil }

func TestRetryPublisher_RetriesEveryErrorByDefault(t *testing.T) {
	t.Parallel()

	inner := &countingPublisher{failUntil: -1, err: errPermanent}

	pub := goflux.RetryPublisher[string](inner, 3, noBackoff)

	if err := pub.Publish(context.Background(), "events", "hello"); !errors.Is(err, errPermanent) {
		t.Fatalf("expected %v, got %v", errPermanent, err)
	}

	if inner.attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", inner.attempts)
	}
}

func TestRetryPublisher_WithRetryable_RetriesMatchingError(t *testing.T) {
	t.Parallel()

	inner := &countingPublisher{failUntil: 2, err: errTransient}

	pub := goflux.RetryPublisher[string](inner, 3, noBackoff,
		goflux.WithRetryable(func(err error) bool {
			return errors.Is(err, errTransient)
		}),
	)

	if err := pub.Publish(context.Background(), "events", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", inner.attempts)
	}
}

func TestRetryPublisher_WithRetryable_StopsOnOtherError(t *testing.T) {
	t.Parallel()

	inner := &countingPublisher{failUntil: -1, err: errPermanent}

	var backoffCalls int

	pub := goflux.RetryPublisher[string](inner, 3, func(attempt int) time.Duration {
		backoffCalls++

		return noBackoff(attempt)
	}, goflux.WithRetryable(func(err error) bool {
		return errors.Is(err, errTransient)
	}))

	if err := pub.Publish(context.Background(), "events", "hello"); !errors.Is(err, errPermanent) {
		t.Fatalf("expected %v, got %v", errPermanent, err)
	}

	if inner.attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", inner.attempts)
	}

	if backoffCalls != 0 {
		t.Fatalf("expected no backoff, got %d calls", backoffCalls)
	}
}
