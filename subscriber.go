package goflux

import (
	"context"
)

// Subscriber listens on one or more subjects and dispatches decoded messages
// to a Handler.
type Subscriber[T any] interface {
	// Subscribe registers handler for the nats. The call blocks until ctx is
	// canceled or the implementation encounters a fatal error.
	Subscribe(ctx context.Context, subject string, handler Handler[T]) error
	// Close unsubscribes and releases resources.
	Close() error
}
