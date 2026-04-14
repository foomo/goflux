package goflux

import "context"

// Consumer provides pull-based message consumption. The caller controls the
// fetch rate. Each fetched message MUST be explicitly acknowledged via
// [Message.Ack], [Message.Nak], or [Message.Term].
type Consumer[T any] interface {
	// Fetch retrieves up to n messages. It blocks until at least one message
	// is available or ctx is cancelled.
	Fetch(ctx context.Context, n int) ([]Message[T], error)
	// Close releases any underlying resources.
	Close() error
}
