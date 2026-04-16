package goflux

import (
	"errors"
	"fmt"
)

// Sentinel errors for classifying failures. Transports join these with the
// causal error via [errors.Join], so callers can inspect with [errors.Is]:
//
//	if errors.Is(err, goflux.ErrEncode) { /* codec problem */ }
//	if errors.Is(err, goflux.ErrPublish) { /* publish-path failure */ }
var (
	// ErrPublish indicates a failure in the publish path.
	ErrPublish = errors.New("publish")
	// ErrSubscribe indicates a failure in the subscribe path.
	ErrSubscribe = errors.New("subscribe")
	// ErrEncode indicates a serialization failure.
	ErrEncode = errors.New("encode")
	// ErrDecode indicates a deserialization failure.
	ErrDecode = errors.New("decode")
	// ErrTransport indicates a transport-level failure (network, protocol).
	ErrTransport = errors.New("transport")
)

// ErrNonRetryable is a sentinel error that marks a handler failure as
// permanent. When a [RetryPolicy] encounters this error (via [errors.Is]),
// it returns [RetryTerm] so that the message is terminated rather than
// redelivered.
var ErrNonRetryable = errors.New("non-retryable")

// NonRetryable wraps err so that errors.Is(err, [ErrNonRetryable]) returns
// true. Use this to signal that a handler error is permanent and the message
// should not be retried.
func NonRetryable(err error) error {
	return fmt.Errorf("%w: %w", ErrNonRetryable, err)
}

// IsNonRetryable reports whether err (or any error in its chain) is marked
// as non-retryable.
func IsNonRetryable(err error) bool {
	return errors.Is(err, ErrNonRetryable)
}
