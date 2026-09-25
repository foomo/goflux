package goflux

import (
	"context"
	"time"
)

// BackoffFunc returns the delay before the next retry attempt.
// attempt starts at 0 for the first retry (i.e. the second overall call).
type BackoffFunc func(attempt int) time.Duration

// RetryableFunc decides whether a failed publish should be retried.
// Returning false stops the retry loop and returns the error to the caller.
type RetryableFunc func(err error) bool

// RetryPublisherOption configures a [RetryPublisher].
type RetryPublisherOption func(*retryPublisherConfig)

// WithRetryable limits retries to the errors for which fn returns true. All
// other errors are returned immediately, without backoff. Without this option
// every publish error is retried.
func WithRetryable(fn RetryableFunc) RetryPublisherOption {
	return func(c *retryPublisherConfig) { c.retryable = fn }
}

// RetryPublisher wraps a Publisher with retry logic. On publish failure, it
// retries until maxAttempts publish calls have been made, with delays
// determined by backoff. Context cancellation aborts the retry loop
// immediately.
//
// By default every publish error is retried — use [WithRetryable] to restrict
// retries to a subset of errors.
//
// If all attempts fail, the last error is returned.
func RetryPublisher[T any](pub Publisher[T], maxAttempts int, backoff BackoffFunc, opts ...RetryPublisherOption) Publisher[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	cfg := &retryPublisherConfig{
		retryable: func(err error) bool { return err != nil },
	}
	for _, o := range opts {
		o(cfg)
	}

	return &retryPublisher[T]{
		pub:         pub,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		retryable:   cfg.retryable,
	}
}

type retryPublisherConfig struct {
	retryable RetryableFunc
}

type retryPublisher[T any] struct {
	pub         Publisher[T]
	maxAttempts int
	backoff     BackoffFunc
	retryable   RetryableFunc
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

		if !r.retryable(lastErr) {
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
