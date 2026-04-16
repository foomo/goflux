package goflux

import (
	"context"
	"time"
)

// BackoffFunc returns the delay before the next retry attempt.
// attempt starts at 0 for the first retry (i.e. the second overall call).
type BackoffFunc func(attempt int) time.Duration

// RetryPublisher wraps a Publisher with retry logic. On publish failure, it
// retries up to maxAttempts times with delays determined by backoff. Context
// cancellation aborts the retry loop immediately.
//
// If all attempts fail, the last error is returned.
func RetryPublisher[T any](pub Publisher[T], maxAttempts int, backoff BackoffFunc) Publisher[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	return &retryPublisher[T]{
		pub:         pub,
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

type retryPublisher[T any] struct {
	pub         Publisher[T]
	maxAttempts int
	backoff     BackoffFunc
}

func (r *retryPublisher[T]) Publish(ctx context.Context, subject string, v T) error {
	var lastErr error

	for attempt := range r.maxAttempts {
		lastErr = r.pub.Publish(ctx, subject, v)
		if lastErr == nil {
			return nil
		}

		if attempt == r.maxAttempts-1 {
			break
		}

		delay := r.backoff(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

func (r *retryPublisher[T]) Close() error {
	return r.pub.Close()
}
