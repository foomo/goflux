package goflux

import "context"

// BoundSubscriber subscribes to a fixed nats. No nats param needed.
type BoundSubscriber[T any] interface {
	// Subscribe registers handler for the bound nats. The call blocks until
	// ctx is canceled or the implementation encounters a fatal error.
	Subscribe(ctx context.Context, handler Handler[T]) error
	// Close unsubscribes and releases resources.
	Close() error
}

// BindSubscriber wraps a Subscriber with a fixed nats.
func BindSubscriber[T any](sub Subscriber[T], subject string) BoundSubscriber[T] {
	return &boundSubscriber[T]{sub: sub, subject: subject}
}

type boundSubscriber[T any] struct {
	sub     Subscriber[T]
	subject string
}

func (b *boundSubscriber[T]) Subscribe(ctx context.Context, handler Handler[T]) error {
	return b.sub.Subscribe(ctx, b.subject, handler)
}

func (b *boundSubscriber[T]) Close() error {
	return b.sub.Close()
}
